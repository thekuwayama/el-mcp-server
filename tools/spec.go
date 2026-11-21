package tools

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/thekuwayama/el-mcp-server/echonet"
	"github.com/thekuwayama/el-mcp-server/echonet/spec"
)

func registerSpecTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_device_class",
		Description: "名前・キーワード・EOJコードでECHONET Lite機器クラスを検索します。EOJは4桁16進(例: 0130)で指定可能。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, searchDeviceClass)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_epc",
		Description: "指定機器クラス(EOJ)のECHONET LiteプロパティコードEPC一覧を返します。EOJは4桁16進(例: 0130)で指定。スーパークラス共通EPCも含みます。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, listEPC)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_epc_detail",
		Description: "指定機器クラス(EOJ)の特定EPC詳細(データ型・単位・アクセス規則・説明)を返します。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, getEPCDetail)
}

type searchDeviceClassParams struct {
	Query string `json:"query" jsonschema:"機器クラスの検索クエリ。日本語名・英語名・EOJコード(4桁16進)で検索可能。例: エアコン, meter, 0288"`
}

type deviceClassResult struct {
	EOJ      string `json:"eoj"`
	NameJP   string `json:"name_jp"`
	NameEN   string `json:"name_en"`
	EPCCount int    `json:"epc_count"`
}

func searchDeviceClass(_ context.Context, _ *mcp.CallToolRequest, params *searchDeviceClassParams) (*mcp.CallToolResult, any, error) {
	q := strings.ToUpper(strings.TrimSpace(params.Query))
	var results []deviceClassResult

	for _, cls := range spec.Classes {
		eojHex := cls.EOJHex()
		if strings.Contains(strings.ToUpper(cls.NameJP), q) ||
			strings.Contains(strings.ToUpper(cls.NameEN), q) ||
			strings.Contains(eojHex, q) {
			results = append(results, deviceClassResult{
				EOJ:      eojHex,
				NameJP:   cls.NameJP,
				NameEN:   cls.NameEN,
				EPCCount: len(cls.EPCs) + len(superClassEPCsFor(cls)),
			})
		}
	}

	if len(results) == 0 {
		return textResult(fmt.Sprintf("'%s' に一致する機器クラスが見つかりませんでした。", params.Query)), nil, nil
	}
	return jsonResult(results)
}

type listEPCParams struct {
	EOJ string `json:"eoj" jsonschema:"機器クラスのEOJコード(4桁16進)。例: 0130(家庭用エアコン), 0288(低圧スマートメーター)"`
}

type epcSummary struct {
	EPC         string `json:"epc"`
	NameJP      string `json:"name_jp"`
	NameEN      string `json:"name_en"`
	DataType    string `json:"data_type"`
	Unit        string `json:"unit,omitempty"`
	AccessRules string `json:"access_rules"`
}

func listEPC(_ context.Context, _ *mcp.CallToolRequest, params *listEPCParams) (*mcp.CallToolResult, any, error) {
	cls, ok := findClass(params.EOJ)
	if !ok {
		return errorResult(fmt.Sprintf("EOJ '%s' の機器クラスが見つかりません。search_device_class で検索してください。", params.EOJ)), nil, nil
	}

	type response struct {
		EOJ        string       `json:"eoj"`
		NameJP     string       `json:"name_jp"`
		SuperClass []epcSummary `json:"super_class_epcs"`
		ClassEPCs  []epcSummary `json:"class_epcs"`
	}

	resp := response{
		EOJ:        cls.EOJHex(),
		NameJP:     cls.NameJP,
		SuperClass: toEPCSummaries(superClassEPCsFor(cls)),
		ClassEPCs:  toEPCSummaries(cls.EPCs),
	}
	return jsonResult(resp)
}

type getEPCDetailParams struct {
	EOJ string `json:"eoj" jsonschema:"機器クラスのEOJコード(4桁16進)。例: 0288"`
	EPC string `json:"epc" jsonschema:"プロパティコード(2桁16進)。例: E7"`
}

type epcDetail struct {
	EPC         string `json:"epc"`
	NameJP      string `json:"name_jp"`
	NameEN      string `json:"name_en"`
	DataType    string `json:"data_type"`
	Unit        string `json:"unit,omitempty"`
	AccessRules string `json:"access_rules"`
	Description string `json:"description"`
}

func getEPCDetail(_ context.Context, _ *mcp.CallToolRequest, params *getEPCDetailParams) (*mcp.CallToolResult, any, error) {
	epcCode, err := parseHexByte(params.EPC)
	if err != nil {
		return errorResult(fmt.Sprintf("EPCの形式が正しくありません: %s", params.EPC)), nil, nil
	}

	cls, ok := findClass(params.EOJ)
	if !ok {
		return errorResult(fmt.Sprintf("EOJ '%s' の機器クラスが見つかりません。", params.EOJ)), nil, nil
	}
	e, ok := findEPC(cls, epcCode)
	if ok {
		return jsonResult(toEPCDetail(e))
	}
	return errorResult(fmt.Sprintf("EOJ %s に EPC %s が定義されていません。", params.EOJ, params.EPC)), nil, nil
}

// --- helpers ---

func findClass(eojHex string) (spec.DeviceClass, bool) {
	upper := strings.ToUpper(strings.TrimSpace(eojHex))
	for _, cls := range spec.Classes {
		if cls.EOJHex() == upper {
			return cls, true
		}
	}
	return spec.DeviceClass{}, false
}

// findEPC looks up epc for cls. The device class definition takes precedence
// over the super class one, because the MRA redefines some super class EPCs
// per device class (e.g. 0x8F of 0x0130).
func findEPC(cls spec.DeviceClass, epc byte) (spec.EPCDef, bool) {
	for _, e := range cls.EPCs {
		if e.Code == epc {
			return e, true
		}
	}
	for _, e := range spec.SuperClassEPCs {
		if e.Code == epc {
			return e, true
		}
	}
	return spec.EPCDef{}, false
}

// superClassEPCsFor returns the super class EPCs that cls does not redefine.
func superClassEPCsFor(cls spec.DeviceClass) []spec.EPCDef {
	defined := make(map[byte]bool, len(cls.EPCs))
	for _, e := range cls.EPCs {
		defined[e.Code] = true
	}
	out := make([]spec.EPCDef, 0, len(spec.SuperClassEPCs))
	for _, e := range spec.SuperClassEPCs {
		if !defined[e.Code] {
			out = append(out, e)
		}
	}
	return out
}

// parseHexByte decodes exactly two hex digits (e.g. "B0" or "b0") into a
// byte. Surrounding whitespace is ignored; anything else, such as a "0x"
// prefix or extra digits, is rejected.
func parseHexByte(s string) (byte, error) {
	t := strings.TrimSpace(s)
	if len(t) != 2 {
		return 0, fmt.Errorf("hex byte must be 2 digits: %q", s)
	}
	b, err := hex.DecodeString(t)
	if err != nil {
		return 0, fmt.Errorf("invalid hex byte %q: %w", s, err)
	}
	return b[0], nil
}

// errHexTooLong is returned by parseHexBytes when the decoded value does not
// fit in a single property's EDT.
var errHexTooLong = errors.New("hex string too long")

// parseHexBytes decodes a whitespace-separated or contiguous hex string (e.g.
// "30" or "FF 01") into a byte slice. Every character other than the spaces
// must be a hex digit, and the result must fit in a single property's EDT
// (at most echonet.MaxPDC bytes); a longer value yields errHexTooLong.
func parseHexBytes(s string) ([]byte, error) {
	hexStr := strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	if hexStr == "" {
		return nil, fmt.Errorf("empty hex string")
	}
	if len(hexStr)%2 != 0 {
		return nil, fmt.Errorf("odd-length hex string: %s", s)
	}
	if n := len(hexStr) / 2; n > echonet.MaxPDC {
		return nil, fmt.Errorf("%w: %d bytes (max %d)", errHexTooLong, n, echonet.MaxPDC)
	}
	edt, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string %q: %w", s, err)
	}
	return edt, nil
}

func toEPCSummaries(epcs []spec.EPCDef) []epcSummary {
	out := make([]epcSummary, len(epcs))
	for i, e := range epcs {
		out[i] = epcSummary{
			EPC:         fmt.Sprintf("%02X", e.Code),
			NameJP:      e.NameJP,
			NameEN:      e.NameEN,
			DataType:    e.DataType,
			Unit:        e.Unit,
			AccessRules: e.AccessRules,
		}
	}
	return out
}

func toEPCDetail(e spec.EPCDef) epcDetail {
	return epcDetail{
		EPC:         fmt.Sprintf("%02X", e.Code),
		NameJP:      e.NameJP,
		NameEN:      e.NameEN,
		DataType:    e.DataType,
		Unit:        e.Unit,
		AccessRules: e.AccessRules,
		Description: e.Description,
	}
}

// parseEOJParam parses eojHex, returning a ready-to-return error
// CallToolResult when it is malformed.
func parseEOJParam(eojHex string) (uint32, *mcp.CallToolResult) {
	eoj, err := echonet.ParseEOJHex(eojHex)
	if err != nil {
		return 0, errorResult(fmt.Sprintf("EOJの形式が正しくありません: %s", eojHex))
	}
	return eoj, nil
}

// hexJoin renders edt as a space-separated uppercase hex string, e.g. "30 01".
func hexJoin(edt []byte) string {
	parts := make([]string, len(edt))
	for i, b := range edt {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, " ")
}

func textResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: msg}}}
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}
}

func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return errorResult(fmt.Sprintf("JSON marshal エラー: %v", err)), nil, nil
	}

	// structuredContent must marshal to a JSON object per the MCP spec; arrays
	// and other non-object values are only safe to surface as text content.
	structured := v
	switch reflect.ValueOf(v).Kind() {
	case reflect.Slice, reflect.Array:
		structured = nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, structured, nil
}
