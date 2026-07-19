package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	EOJ         string `json:"eoj"`
	NameJP      string `json:"name_jp"`
	NameEN      string `json:"name_en"`
	Description string `json:"description"`
	EPCCount    int    `json:"epc_count"`
}

func searchDeviceClass(_ context.Context, _ *mcp.CallToolRequest, params *searchDeviceClassParams) (*mcp.CallToolResult, any, error) {
	q := strings.ToUpper(strings.TrimSpace(params.Query))
	var results []deviceClassResult

	for _, cls := range spec.Classes {
		eojHex := cls.EOJHex()
		if strings.Contains(strings.ToUpper(cls.NameJP), strings.ToUpper(params.Query)) ||
			strings.Contains(strings.ToUpper(cls.NameEN), q) ||
			strings.Contains(strings.ToUpper(cls.Description), strings.ToUpper(params.Query)) ||
			strings.Contains(eojHex, q) {
			results = append(results, deviceClassResult{
				EOJ:         eojHex,
				NameJP:      cls.NameJP,
				NameEN:      cls.NameEN,
				Description: cls.Description,
				EPCCount:    len(cls.EPCs) + len(spec.SuperClassEPCs),
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
		EOJ         string       `json:"eoj"`
		NameJP      string       `json:"name_jp"`
		SuperClass  []epcSummary `json:"super_class_epcs"`
		ClassEPCs   []epcSummary `json:"class_epcs"`
	}

	resp := response{
		EOJ:        cls.EOJHex(),
		NameJP:     cls.NameJP,
		SuperClass: toEPCSummaries(spec.SuperClassEPCs),
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

	// search in super class first
	for _, e := range spec.SuperClassEPCs {
		if e.Code == epcCode {
			return jsonResult(toEPCDetail(e))
		}
	}

	cls, ok := findClass(params.EOJ)
	if !ok {
		return errorResult(fmt.Sprintf("EOJ '%s' の機器クラスが見つかりません。", params.EOJ)), nil, nil
	}
	for _, e := range cls.EPCs {
		if e.Code == epcCode {
			return jsonResult(toEPCDetail(e))
		}
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

func parseHexByte(s string) (byte, error) {
	var v byte
	_, err := fmt.Sscanf(strings.ToUpper(strings.TrimSpace(s)), "%02X", &v)
	return v, err
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
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, v, nil
}
