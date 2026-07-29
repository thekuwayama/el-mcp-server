package tools

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/thekuwayama/el-mcp-server/echonet"
	"github.com/thekuwayama/el-mcp-server/tools/ui"
)

//go:embed ui/templates/battery.html
var batteryDashboardHTML string

const batteryDashboardURI = "ui://el-mcp-server/battery"

// MCPAppsUIMimeType is the MIME type required by the MCP Apps extension
// (SEP-1865) for ui:// resources.
const MCPAppsUIMimeType = "text/html;profile=mcp-app"

func registerUITools(s *mcp.Server) {
	s.AddResource(&mcp.Resource{
		URI:      batteryDashboardURI,
		Name:     "battery_dashboard",
		Title:    "蓄電池ダッシュボード",
		MIMEType: MCPAppsUIMimeType,
	}, batteryDashboardResource)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_battery_ui",
		Description: "指定した蓄電池(EOJ 027Dxx)の現在状態(稼働状態・運転モード・蓄電残量・充放電電力)を取得し、MCP Apps対応クライアントではダッシュボードUIとして表示します。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Meta: mcp.Meta{
			"ui": map[string]any{
				"resourceUri": batteryDashboardURI,
			},
		},
	}, renderBatteryUI)
}

func batteryDashboardResource(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: MCPAppsUIMimeType,
				Text:     batteryDashboardHTML,
			},
		},
	}, nil
}

type renderBatteryUIParams struct {
	IP  string `json:"ip"  jsonschema:"蓄電池のIPアドレス。例: 192.168.1.50"`
	EOJ string `json:"eoj" jsonschema:"対象オブジェクトのEOJコード(4〜6桁16進、蓄電池クラス027Dのみ対応)。例: 027D01"`
}

type batteryUIState struct {
	IP                       string `json:"ip"`
	EOJ                      string `json:"eoj"`
	OperatingStatus          string `json:"operating_status,omitempty"`
	OperationMode            string `json:"operation_mode,omitempty"`
	RemainingCapacityPercent *int   `json:"remaining_capacity_percent,omitempty"`
	ChargeDischargePowerW    *int32 `json:"charge_discharge_power_w,omitempty"`
}

func renderBatteryUI(_ context.Context, _ *mcp.CallToolRequest, params *renderBatteryUIParams) (*mcp.CallToolResult, any, error) {
	eoj, err := echonet.ParseEOJHex(params.EOJ)
	if err != nil {
		return errorResult(fmt.Sprintf("EOJの形式が正しくありません: %s", params.EOJ)), nil, nil
	}

	groupCode := byte(eoj >> 16)
	classCode := byte(eoj >> 8)
	if groupCode != 0x02 || classCode != 0x7D {
		return errorResult(fmt.Sprintf("render_battery_ui は蓄電池(EOJ 027Dxx)専用です。指定されたEOJ %s は対象外です。他の機器は get_property をご利用ください。", params.EOJ)), nil, nil
	}

	state := batteryUIState{
		IP:  params.IP,
		EOJ: fmt.Sprintf("%06X", eoj),
	}

	const timeout = 5 * time.Second

	if edt, err := echonet.GetProperty(params.IP, eoj, ui.OperatingStatusEPC, timeout); err == nil {
		if v, ok := ui.DecodeOperatingStatus(edt); ok {
			state.OperatingStatus = v
		}
	}
	if edt, err := echonet.GetProperty(params.IP, eoj, ui.OperationModeEPC, timeout); err == nil {
		if v, ok := ui.DecodeOperationMode(edt); ok {
			state.OperationMode = v
		}
	}
	if edt, err := echonet.GetProperty(params.IP, eoj, ui.RemainingCapacityPercentEPC, timeout); err == nil {
		if v, ok := ui.DecodePercent(edt); ok {
			state.RemainingCapacityPercent = &v
		}
	}
	if edt, err := echonet.GetProperty(params.IP, eoj, ui.ChargeDischargePowerEPC, timeout); err == nil {
		if v, ok := ui.DecodeSignedPowerW(edt); ok {
			state.ChargeDischargePowerW = &v
		}
	}

	if state.OperatingStatus == "" && state.OperationMode == "" && state.RemainingCapacityPercent == nil && state.ChargeDischargePowerW == nil {
		return errorResult(fmt.Sprintf("蓄電池(IP: %s, EOJ: %s)から表示可能なプロパティを取得できませんでした。", params.IP, state.EOJ)), nil, nil
	}

	return jsonResult(state)
}
