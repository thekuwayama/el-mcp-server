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

//go:embed ui/templates/solar.html
var solarDashboardHTML string

//go:embed ui/templates/v2h.html
var v2hDashboardHTML string

const batteryDashboardURI = "ui://el-mcp-server/battery"
const solarDashboardURI = "ui://el-mcp-server/solar"
const v2hDashboardURI = "ui://el-mcp-server/v2h"

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
		Description: "指定した蓄電池(EOJ 027Dxx)の現在状態(稼働状態・運転モード・蓄電残量・充放電電力)を取得し、MCP Apps対応クライアントではダッシュボードUIとして表示します。ダッシュボード上では稼働状態(ON/OFF)の切り替えと運転モードの変更が可能で、操作は内部でset_propertyツールを呼び出します。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Meta: mcp.Meta{
			"ui": map[string]any{
				"resourceUri": batteryDashboardURI,
			},
		},
	}, renderBatteryUI)

	s.AddResource(&mcp.Resource{
		URI:      solarDashboardURI,
		Name:     "solar_dashboard",
		Title:    "太陽光発電ダッシュボード",
		MIMEType: MCPAppsUIMimeType,
	}, solarDashboardResource)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_solar_ui",
		Description: "指定した太陽光発電システム(EOJ 0279xx)の現在状態(稼働状態・瞬時発電電力・積算発電電力量・積算売電電力量・系統連系状態・出力抑制状態)を取得し、MCP Apps対応クライアントではダッシュボードUIとして表示します。このダッシュボードは読み取り専用です。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Meta: mcp.Meta{
			"ui": map[string]any{
				"resourceUri": solarDashboardURI,
			},
		},
	}, renderSolarUI)

	s.AddResource(&mcp.Resource{
		URI:      v2hDashboardURI,
		Name:     "v2h_dashboard",
		Title:    "V2Hダッシュボード",
		MIMEType: MCPAppsUIMimeType,
	}, v2hDashboardResource)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_v2h_ui",
		Description: "指定したV2H(電気自動車充放電器、EOJ 027Exx)の現在状態(稼働状態・運転モード・車載電池残容量・充放電電力・積算充電/放電電力量・車両接続状態)を取得し、MCP Apps対応クライアントではダッシュボードUIとして表示します。ダッシュボード上では稼働状態(ON/OFF)の切り替えと運転モードの変更が可能で、操作は内部でset_propertyツールを呼び出します。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Meta: mcp.Meta{
			"ui": map[string]any{
				"resourceUri": v2hDashboardURI,
			},
		},
	}, renderV2HUI)
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

func solarDashboardResource(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: MCPAppsUIMimeType,
				Text:     solarDashboardHTML,
			},
		},
	}, nil
}

type renderSolarUIParams struct {
	IP  string `json:"ip"  jsonschema:"太陽光発電システムのIPアドレス。例: 192.168.1.50"`
	EOJ string `json:"eoj" jsonschema:"対象オブジェクトのEOJコード(4〜6桁16進、住宅用太陽光発電クラス0279のみ対応)。例: 027901"`
}

type solarUIState struct {
	IP                          string   `json:"ip"`
	EOJ                         string   `json:"eoj"`
	OperatingStatus             string   `json:"operating_status,omitempty"`
	InstantaneousPowerW         *uint16  `json:"instantaneous_power_w,omitempty"`
	CumulativeGeneratedKWh      *float64 `json:"cumulative_generated_kwh,omitempty"`
	CumulativeSoldKWh           *float64 `json:"cumulative_sold_kwh,omitempty"`
	SelfConsumptionPercent      *int     `json:"self_consumption_percent,omitempty"`
	SystemInterconnectionStatus string   `json:"system_interconnection_status,omitempty"`
	OutputPowerRestraintStatus  string   `json:"output_power_restraint_status,omitempty"`
}

func renderSolarUI(_ context.Context, _ *mcp.CallToolRequest, params *renderSolarUIParams) (*mcp.CallToolResult, any, error) {
	eoj, err := echonet.ParseEOJHex(params.EOJ)
	if err != nil {
		return errorResult(fmt.Sprintf("EOJの形式が正しくありません: %s", params.EOJ)), nil, nil
	}

	groupCode := byte(eoj >> 16)
	classCode := byte(eoj >> 8)
	if groupCode != 0x02 || classCode != 0x79 {
		return errorResult(fmt.Sprintf("render_solar_ui は住宅用太陽光発電(EOJ 0279xx)専用です。指定されたEOJ %s は対象外です。他の機器は get_property をご利用ください。", params.EOJ)), nil, nil
	}

	state := solarUIState{
		IP:  params.IP,
		EOJ: fmt.Sprintf("%06X", eoj),
	}

	const timeout = 5 * time.Second

	if edt, err := echonet.GetProperty(params.IP, eoj, ui.OperatingStatusEPC, timeout); err == nil {
		if v, ok := ui.DecodeOperatingStatus(edt); ok {
			state.OperatingStatus = v
		}
	}
	if edt, err := echonet.GetProperty(params.IP, eoj, ui.InstantaneousElectricPowerGenerationEPC, timeout); err == nil {
		if v, ok := ui.DecodeUnsignedPowerW(edt); ok {
			state.InstantaneousPowerW = &v
		}
	}
	if edt, err := echonet.GetProperty(params.IP, eoj, ui.CumulativeElectricEnergyOfGenerationEPC, timeout); err == nil {
		if v, ok := ui.DecodeCumulativeEnergyKWh(edt); ok {
			state.CumulativeGeneratedKWh = &v
		}
	}
	if edt, err := echonet.GetProperty(params.IP, eoj, ui.CumulativeElectricEnergySoldEPC, timeout); err == nil {
		if v, ok := ui.DecodeCumulativeEnergyKWh(edt); ok {
			state.CumulativeSoldKWh = &v
		}
	}
	if state.CumulativeGeneratedKWh != nil && state.CumulativeSoldKWh != nil {
		if v, ok := ui.SelfConsumptionPercent(*state.CumulativeGeneratedKWh, *state.CumulativeSoldKWh); ok {
			state.SelfConsumptionPercent = &v
		}
	}
	if edt, err := echonet.GetProperty(params.IP, eoj, ui.SystemInterconnectionStatusEPC, timeout); err == nil {
		if v, ok := ui.DecodeSystemInterconnectionStatus(edt); ok {
			state.SystemInterconnectionStatus = v
		}
	}
	if edt, err := echonet.GetProperty(params.IP, eoj, ui.OutputPowerRestraintStatusEPC, timeout); err == nil {
		if v, ok := ui.DecodeOutputPowerRestraintStatus(edt); ok {
			state.OutputPowerRestraintStatus = v
		}
	}

	if state.OperatingStatus == "" && state.InstantaneousPowerW == nil && state.CumulativeGeneratedKWh == nil &&
		state.CumulativeSoldKWh == nil && state.SystemInterconnectionStatus == "" && state.OutputPowerRestraintStatus == "" {
		return errorResult(fmt.Sprintf("太陽光発電システム(IP: %s, EOJ: %s)から表示可能なプロパティを取得できませんでした。", params.IP, state.EOJ)), nil, nil
	}

	return jsonResult(state)
}

func v2hDashboardResource(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: MCPAppsUIMimeType,
				Text:     v2hDashboardHTML,
			},
		},
	}, nil
}

type renderV2HUIParams struct {
	IP  string `json:"ip"  jsonschema:"V2H(電気自動車充放電器)のIPアドレス。例: 192.168.1.50"`
	EOJ string `json:"eoj" jsonschema:"対象オブジェクトのEOJコード(4〜6桁16進、電気自動車充放電器クラス027Eのみ対応)。例: 027E01"`
}

type v2hUIState struct {
	IP                       string   `json:"ip"`
	EOJ                      string   `json:"eoj"`
	OperatingStatus          string   `json:"operating_status,omitempty"`
	OperationMode            string   `json:"operation_mode,omitempty"`
	RemainingCapacityPercent *int     `json:"remaining_capacity_percent,omitempty"`
	ChargeDischargePowerW    *int32   `json:"charge_discharge_power_w,omitempty"`
	CumulativeChargingKWh    *float64 `json:"cumulative_charging_kwh,omitempty"`
	CumulativeDischargingKWh *float64 `json:"cumulative_discharging_kwh,omitempty"`
	VehicleConnectionStatus  string   `json:"vehicle_connection_status,omitempty"`
}

func renderV2HUI(_ context.Context, _ *mcp.CallToolRequest, params *renderV2HUIParams) (*mcp.CallToolResult, any, error) {
	eoj, err := echonet.ParseEOJHex(params.EOJ)
	if err != nil {
		return errorResult(fmt.Sprintf("EOJの形式が正しくありません: %s", params.EOJ)), nil, nil
	}

	groupCode := byte(eoj >> 16)
	classCode := byte(eoj >> 8)
	if groupCode != 0x02 || classCode != 0x7E {
		return errorResult(fmt.Sprintf("render_v2h_ui はV2H(電気自動車充放電器、EOJ 027Exx)専用です。指定されたEOJ %s は対象外です。他の機器は get_property をご利用ください。", params.EOJ)), nil, nil
	}

	state := v2hUIState{
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
		if v, ok := ui.DecodeV2HOperationMode(edt); ok {
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
	if edt, err := echonet.GetProperty(params.IP, eoj, ui.CumulativeChargingEnergyEPC, timeout); err == nil {
		if v, ok := ui.DecodeCumulativeEnergyKWh(edt); ok {
			state.CumulativeChargingKWh = &v
		}
	}
	if edt, err := echonet.GetProperty(params.IP, eoj, ui.CumulativeDischargingEnergyEPC, timeout); err == nil {
		if v, ok := ui.DecodeCumulativeEnergyKWh(edt); ok {
			state.CumulativeDischargingKWh = &v
		}
	}
	if edt, err := echonet.GetProperty(params.IP, eoj, ui.VehicleConnectionStatusEPC, timeout); err == nil {
		if v, ok := ui.DecodeVehicleConnectionStatus(edt); ok {
			state.VehicleConnectionStatus = v
		}
	}

	if state.OperatingStatus == "" && state.OperationMode == "" && state.RemainingCapacityPercent == nil &&
		state.ChargeDischargePowerW == nil && state.CumulativeChargingKWh == nil &&
		state.CumulativeDischargingKWh == nil && state.VehicleConnectionStatus == "" {
		return errorResult(fmt.Sprintf("V2H(IP: %s, EOJ: %s)から表示可能なプロパティを取得できませんでした。", params.IP, state.EOJ)), nil, nil
	}

	return jsonResult(state)
}
