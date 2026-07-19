package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/thekuwayama/el-mcp-server/echonet"
	"github.com/thekuwayama/el-mcp-server/echonet/spec/manufacturers"
)

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
}

type discoverDevicesParams struct {
	TimeoutSec int `json:"timeout_sec" jsonschema:"探索タイムアウト秒数(デフォルト3秒)。LAN環境に合わせて1〜10を指定。"`
}

type discoveredDevice struct {
	IP   string   `json:"ip"`
	EOJs []string `json:"eojs"`
}

func discoverDevices(_ context.Context, _ *mcp.CallToolRequest, params *discoverDevicesParams) (*mcp.CallToolResult, any, error) {
	timeout := params.TimeoutSec
	if timeout <= 0 {
		timeout = 3
	}

	results, err := echonet.Discover(timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("discover: %w", err)
	}

	if len(results) == 0 {
		return textResult("ECHONET Lite機器が見つかりませんでした。同一LAN上に機器が存在するか確認してください。"), nil, nil
	}

	devices := make([]discoveredDevice, len(results))
	for i, r := range results {
		eojs := make([]string, len(r.EOJs))
		for j, eoj := range r.EOJs {
			eojs[j] = fmt.Sprintf("%06X", eoj)
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

func getProperty(_ context.Context, _ *mcp.CallToolRequest, params *getPropertyParams) (*mcp.CallToolResult, any, error) {
	eoj, err := echonet.ParseEOJHex(params.EOJ)
	if err != nil {
		return textResult(fmt.Sprintf("EOJの形式が正しくありません: %s", params.EOJ)), nil, nil
	}

	epcCode, err := parseHexByte(params.EPC)
	if err != nil {
		return textResult(fmt.Sprintf("EPCの形式が正しくありません: %s", params.EPC)), nil, nil
	}

	edt, err := echonet.GetProperty(params.IP, eoj, epcCode, 5*time.Second)
	if err != nil {
		return textResult(fmt.Sprintf("プロパティ取得エラー: %v", err)), nil, nil
	}

	hexParts := make([]string, len(edt))
	for i, b := range edt {
		hexParts[i] = fmt.Sprintf("%02X", b)
	}

	result := propertyValue{
		IP:       params.IP,
		EOJ:      strings.ToUpper(params.EOJ),
		EPC:      strings.ToUpper(params.EPC),
		EDTHex:   strings.Join(hexParts, " "),
		EDTBytes: len(edt),
	}
	if epcCode == 0x8A {
		if name, ok := manufacturers.Lookup(edt); ok {
			result.ManufacturerName = name
		}
	}
	return jsonResult(result)
}
