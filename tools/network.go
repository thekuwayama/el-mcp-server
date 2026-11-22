package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/thekuwayama/el-mcp-server/echonet"
	"github.com/thekuwayama/el-mcp-server/echonet/spec/manufacturers"
)

// parseEOJAndEPC parses eojHex and epcHex, returning a ready-to-return error
// CallToolResult when either is malformed.
func parseEOJAndEPC(eojHex, epcHex string) (uint32, byte, *mcp.CallToolResult) {
	eoj, errRes := parseEOJParam(eojHex)
	if errRes != nil {
		return 0, 0, errRes
	}
	epcCode, err := parseHexByte(epcHex)
	if err != nil {
		return 0, 0, errorResult(fmt.Sprintf("EPCの形式が正しくありません: %s", epcHex))
	}
	return eoj, epcCode, nil
}

func registerNetworkTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "discover_devices",
		Description: "LAN内のECHONET Lite機器を探索します。UDPマルチキャストでノードプロファイルに問い合わせ、応答した機器のIPアドレスとEOJ一覧を返します。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, discoverDevices)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_property",
		Description: "指定したECHONET Lite機器のEPCプロパティ値をUDP Getで取得します。値はhex文字列で返ります。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, getProperty)

	destructive := true
	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_property",
		Description: "指定したECHONET Lite機器のEPCプロパティ値をUDP SetC(応答あり)で書き込みます。機器の実際の状態を変更する操作です。事前にget_propertyやget_epc_detailで書き込み可能なEPCとEDTの形式を確認してから呼び出してください。",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			DestructiveHint: &destructive,
			IdempotentHint:  true,
		},
	}, setProperty)
}

type discoverDevicesParams struct {
	TimeoutSec int `json:"timeout_sec" jsonschema:"探索タイムアウト秒数(デフォルト3秒)。LAN環境に合わせて1〜10を指定。"`
}

type discoveredDevice struct {
	IP   string   `json:"ip"`
	EOJs []string `json:"eojs"`
}

func discoverDevices(ctx context.Context, _ *mcp.CallToolRequest, params *discoverDevicesParams) (*mcp.CallToolResult, any, error) {
	timeoutSec := params.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 3
	}

	results, err := echonet.Discover(ctx, time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return errorResult(fmt.Sprintf("機器探索エラー: %v", err)), nil, nil
	}

	if len(results) == 0 {
		return textResult("ECHONET Lite機器が見つかりませんでした。同一LAN上に機器が存在するか確認してください。"), nil, nil
	}

	devices := make([]discoveredDevice, len(results))
	for i, r := range results {
		eojs := make([]string, len(r.EOJs))
		for j, eoj := range r.EOJs {
			eojs[j] = formatEOJ(eoj)
		}
		devices[i] = discoveredDevice{IP: r.IP, EOJs: eojs}
	}
	return jsonResult(devices)
}

type getPropertyParams struct {
	IP  string `json:"ip"  jsonschema:"機器のIPアドレス。例: 192.168.1.100"`
	EOJ string `json:"eoj" jsonschema:"対象オブジェクトのEOJコード(4〜6桁16進)。例: 0130 または 013001"`
	EPC string `json:"epc" jsonschema:"取得するプロパティコード(2桁16進)。例: BB(室内温度計測値)"`
}

type propertyValue struct {
	IP               string `json:"ip"`
	EOJ              string `json:"eoj"`
	EPC              string `json:"epc"`
	EDTHex           string `json:"edt_hex"`
	EDTBytes         int    `json:"edt_bytes"`
	ManufacturerName string `json:"manufacturer_name,omitempty"`
}

func getProperty(ctx context.Context, _ *mcp.CallToolRequest, params *getPropertyParams) (*mcp.CallToolResult, any, error) {
	eoj, epcCode, errRes := parseEOJAndEPC(params.EOJ, params.EPC)
	if errRes != nil {
		return errRes, nil, nil
	}

	edt, err := echonet.GetProperty(ctx, params.IP, eoj, epcCode, 5*time.Second)
	if err != nil {
		return errorResult(fmt.Sprintf("プロパティ取得エラー: %v", err)), nil, nil
	}

	result := propertyValue{
		IP:       params.IP,
		EOJ:      formatEOJ(eoj),
		EPC:      formatEPC(epcCode),
		EDTHex:   hexJoin(edt),
		EDTBytes: len(edt),
	}
	if epcCode == 0x8A {
		if name, ok := manufacturers.Lookup(edt); ok {
			result.ManufacturerName = name
		}
	}
	return jsonResult(result)
}

type setPropertyParams struct {
	IP  string `json:"ip"  jsonschema:"機器のIPアドレス。例: 192.168.1.100"`
	EOJ string `json:"eoj" jsonschema:"対象オブジェクトのEOJコード(4〜6桁16進)。例: 0130 または 013001"`
	EPC string `json:"epc" jsonschema:"書き込むプロパティコード(2桁16進)。例: B0(運転モード設定)"`
	EDT string `json:"edt" jsonschema:"書き込む値(hex文字列、1バイトごとに区切っても可、最大255バイト)。例: 41 または 4100(マルチバイト値)"`
}

type setPropertyResult struct {
	IP     string `json:"ip"`
	EOJ    string `json:"eoj"`
	EPC    string `json:"epc"`
	EDTHex string `json:"edt_hex"`
	Status string `json:"status"`
}

func setProperty(ctx context.Context, req *mcp.CallToolRequest, params *setPropertyParams) (*mcp.CallToolResult, any, error) {
	eoj, epcCode, errRes := parseEOJAndEPC(params.EOJ, params.EPC)
	if errRes != nil {
		return errRes, nil, nil
	}

	edt, err := parseHexBytes(params.EDT)
	if errors.Is(err, errHexTooLong) {
		return errorResult(fmt.Sprintf("EDTが長すぎます: %dバイト以下で指定してください。", echonet.MaxPDC)), nil, nil
	}
	if err != nil {
		return errorResult(fmt.Sprintf("EDTの形式が正しくありません: %s", params.EDT)), nil, nil
	}

	confirmed, pending := confirmPropertyWrite(req.ClientCapabilities(), req.Params, params.IP, formatEOJ(eoj), formatEPC(epcCode), hexJoin(edt))
	if pending != nil {
		return pending, nil, nil
	}
	if !confirmed {
		return textResult("書き込みをキャンセルしました。"), nil, nil
	}

	if err := echonet.SetProperty(ctx, params.IP, eoj, epcCode, edt, 5*time.Second); err != nil {
		return errorResult(fmt.Sprintf("プロパティ書き込みエラー: %v", err)), nil, nil
	}

	return jsonResult(setPropertyResult{
		IP:     params.IP,
		EOJ:    formatEOJ(eoj),
		EPC:    formatEPC(epcCode),
		EDTHex: hexJoin(edt),
		Status: "success",
	})
}

// confirmInputKey is the input request ID used for the write confirmation.
const confirmInputKey = "confirm"

// confirmPropertyWrite asks the user to confirm a device write via
// elicitation (SEP-1034 default values) before it is performed, when the
// client advertises support for form elicitation. Clients without
// elicitation support skip confirmation so existing behavior is unchanged.
//
// The confirmation is a multi round-trip request (SEP-2322): on the first
// call it returns a non-nil pending result carrying the elicitation as an
// input request, and the tool call is retried with the user's answer in
// params.InputResponses. Protocol 2026-07-28 forbids server-initiated
// elicitation requests; for older clients the SDK fulfills the input request
// with elicitation/create and re-invokes the handler itself.
//
// The request state binds the answer to the write it was asked for, so an
// answer echoed back with different arguments asks again instead of writing.
func confirmPropertyWrite(caps *mcp.ClientCapabilities, params *mcp.CallToolParamsRaw, ip, eoj, epc, edtHex string) (confirmed bool, pending *mcp.CallToolResult) {
	if caps == nil || !formElicitationSupported(caps.Elicitation) {
		return true, nil
	}

	state := strings.Join([]string{"set_property", ip, eoj, epc, edtHex}, "|")
	if params != nil && params.RequestState == state {
		if res, ok := params.InputResponses[confirmInputKey].(*mcp.ElicitResult); ok {
			if res.Action != "accept" {
				return false, nil
			}
			confirm, _ := res.Content["confirm"].(bool)
			return confirm, nil
		}
	}

	return false, &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{
			confirmInputKey: &mcp.ElicitParams{
				Message: fmt.Sprintf("機器 %s (EOJ %s) の EPC %s に %s を書き込みます。実行しますか？", ip, strings.ToUpper(eoj), strings.ToUpper(epc), edtHex),
				RequestedSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"confirm": map[string]any{
							"type":    "boolean",
							"title":   "実行する",
							"default": false,
						},
					},
				},
			},
		},
		RequestState: state,
	}
}

// formElicitationSupported implements the same form-support check as
// (*mcp.ServerSession).Elicit: absent capabilities means no support, and an
// empty ElicitationCapabilities (both Form and URL nil) is assumed to
// support form elicitation for backward compatibility.
func formElicitationSupported(caps *mcp.ElicitationCapabilities) bool {
	if caps == nil {
		return false
	}
	if caps.Form == nil && caps.URL != nil {
		return false
	}
	return true
}
