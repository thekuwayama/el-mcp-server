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

//go:embed ui/templates/aircon.html
var airconDashboardHTML string

const batteryDashboardURI = "ui://el-mcp-server/battery"
const solarDashboardURI = "ui://el-mcp-server/solar"
const v2hDashboardURI = "ui://el-mcp-server/v2h"
const airconDashboardURI = "ui://el-mcp-server/aircon"

// MCPAppsUIMimeType is the MIME type required by the MCP Apps extension
// (SEP-1865) for ui:// resources.
const MCPAppsUIMimeType = "text/html;profile=mcp-app"

// dashboardResourceHandler returns an mcp.AddResource handler that serves the
// given pre-rendered dashboard HTML for any ui:// resource read request.
func dashboardResourceHandler(html string) func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: MCPAppsUIMimeType,
					Text:     html,
				},
			},
		}, nil
	}
}

// parseAndGuardEOJ parses eojHex and verifies it belongs to (groupCode,
// classCode), returning a ready-to-return error CallToolResult when either
// check fails. deviceDesc is embedded verbatim in the class-mismatch
// message, e.g. "蓄電池(EOJ 027Dxx)".
func parseAndGuardEOJ(eojHex, toolName, deviceDesc string, groupCode, classCode byte) (uint32, *mcp.CallToolResult) {
	eoj, errRes := parseEOJParam(eojHex)
	if errRes != nil {
		return 0, errRes
	}
	if byte(eoj>>16) != groupCode || byte(eoj>>8) != classCode {
		return 0, errorResult(fmt.Sprintf("%s は%s専用です。指定されたEOJ %s は対象外です。他の機器は get_property をご利用ください。", toolName, deviceDesc, eojHex))
	}
	return eoj, nil
}

// getString fetches epc and decodes it with decode, returning "" if the
// property could not be fetched or decoded.
func getString(ip string, eoj uint32, epc byte, timeout time.Duration, decode func([]byte) (string, bool)) string {
	edt, err := echonet.GetProperty(ip, eoj, epc, timeout)
	if err != nil {
		return ""
	}
	v, ok := decode(edt)
	if !ok {
		return ""
	}
	return v
}

// getPtr fetches epc and decodes it with decode, returning nil if the
// property could not be fetched or decoded. Used for numeric fields, where a
// zero value would be indistinguishable from "not reported by the device".
func getPtr[T any](ip string, eoj uint32, epc byte, timeout time.Duration, decode func([]byte) (T, bool)) *T {
	edt, err := echonet.GetProperty(ip, eoj, epc, timeout)
	if err != nil {
		return nil
	}
	v, ok := decode(edt)
	if !ok {
		return nil
	}
	return &v
}

func registerUITools(s *mcp.Server) {
	s.AddResource(&mcp.Resource{
		URI:      batteryDashboardURI,
		Name:     "battery_dashboard",
		Title:    "蓄電池ダッシュボード",
		MIMEType: MCPAppsUIMimeType,
	}, dashboardResourceHandler(batteryDashboardHTML))

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
	}, dashboardResourceHandler(solarDashboardHTML))

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
	}, dashboardResourceHandler(v2hDashboardHTML))

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

	s.AddResource(&mcp.Resource{
		URI:      airconDashboardURI,
		Name:     "aircon_dashboard",
		Title:    "エアコンダッシュボード",
		MIMEType: MCPAppsUIMimeType,
	}, dashboardResourceHandler(airconDashboardHTML))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_aircon_ui",
		Description: "指定した家庭用エアコン(EOJ 0130xx)の現在状態(稼働状態・運転モード・設定温度・室内温度・風量設定)を取得し、MCP Apps対応クライアントではダッシュボードUIとして表示します。ダッシュボード上では稼働状態(ON/OFF)の切り替え・運転モードの変更・設定温度の増減・風量の変更が可能で、操作は内部でset_propertyツールを呼び出します。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Meta: mcp.Meta{
			"ui": map[string]any{
				"resourceUri": airconDashboardURI,
			},
		},
	}, renderAirconUI)
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
	eoj, errRes := parseAndGuardEOJ(params.EOJ, "render_battery_ui", "蓄電池(EOJ 027Dxx)", 0x02, 0x7D)
	if errRes != nil {
		return errRes, nil, nil
	}

	const timeout = 5 * time.Second

	state := batteryUIState{
		IP:                       params.IP,
		EOJ:                      fmt.Sprintf("%06X", eoj),
		OperatingStatus:          getString(params.IP, eoj, ui.OperatingStatusEPC, timeout, ui.DecodeOperatingStatus),
		OperationMode:            getString(params.IP, eoj, ui.OperationModeEPC, timeout, ui.DecodeOperationMode),
		RemainingCapacityPercent: getPtr(params.IP, eoj, ui.RemainingCapacityPercentEPC, timeout, ui.DecodePercent),
		ChargeDischargePowerW:    getPtr(params.IP, eoj, ui.ChargeDischargePowerEPC, timeout, ui.DecodeSignedPowerW),
	}

	if state.OperatingStatus == "" && state.OperationMode == "" && state.RemainingCapacityPercent == nil && state.ChargeDischargePowerW == nil {
		return errorResult(fmt.Sprintf("蓄電池(IP: %s, EOJ: %s)から表示可能なプロパティを取得できませんでした。", params.IP, state.EOJ)), nil, nil
	}

	return jsonResult(state)
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
	eoj, errRes := parseAndGuardEOJ(params.EOJ, "render_solar_ui", "住宅用太陽光発電(EOJ 0279xx)", 0x02, 0x79)
	if errRes != nil {
		return errRes, nil, nil
	}

	const timeout = 5 * time.Second

	state := solarUIState{
		IP:                          params.IP,
		EOJ:                         fmt.Sprintf("%06X", eoj),
		OperatingStatus:             getString(params.IP, eoj, ui.OperatingStatusEPC, timeout, ui.DecodeOperatingStatus),
		InstantaneousPowerW:         getPtr(params.IP, eoj, ui.InstantaneousElectricPowerGenerationEPC, timeout, ui.DecodeUnsignedPowerW),
		CumulativeGeneratedKWh:      getPtr(params.IP, eoj, ui.CumulativeElectricEnergyOfGenerationEPC, timeout, ui.DecodeCumulativeEnergyKWh),
		CumulativeSoldKWh:           getPtr(params.IP, eoj, ui.CumulativeElectricEnergySoldEPC, timeout, ui.DecodeCumulativeEnergyKWh),
		SystemInterconnectionStatus: getString(params.IP, eoj, ui.SystemInterconnectionStatusEPC, timeout, ui.DecodeSystemInterconnectionStatus),
		OutputPowerRestraintStatus:  getString(params.IP, eoj, ui.OutputPowerRestraintStatusEPC, timeout, ui.DecodeOutputPowerRestraintStatus),
	}
	if state.CumulativeGeneratedKWh != nil && state.CumulativeSoldKWh != nil {
		if v, ok := ui.SelfConsumptionPercent(*state.CumulativeGeneratedKWh, *state.CumulativeSoldKWh); ok {
			state.SelfConsumptionPercent = &v
		}
	}

	if state.OperatingStatus == "" && state.InstantaneousPowerW == nil && state.CumulativeGeneratedKWh == nil &&
		state.CumulativeSoldKWh == nil && state.SystemInterconnectionStatus == "" && state.OutputPowerRestraintStatus == "" {
		return errorResult(fmt.Sprintf("太陽光発電システム(IP: %s, EOJ: %s)から表示可能なプロパティを取得できませんでした。", params.IP, state.EOJ)), nil, nil
	}

	return jsonResult(state)
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
	eoj, errRes := parseAndGuardEOJ(params.EOJ, "render_v2h_ui", "V2H(電気自動車充放電器、EOJ 027Exx)", 0x02, 0x7E)
	if errRes != nil {
		return errRes, nil, nil
	}

	const timeout = 5 * time.Second

	state := v2hUIState{
		IP:                       params.IP,
		EOJ:                      fmt.Sprintf("%06X", eoj),
		OperatingStatus:          getString(params.IP, eoj, ui.OperatingStatusEPC, timeout, ui.DecodeOperatingStatus),
		OperationMode:            getString(params.IP, eoj, ui.OperationModeEPC, timeout, ui.DecodeV2HOperationMode),
		RemainingCapacityPercent: getPtr(params.IP, eoj, ui.RemainingCapacityPercentEPC, timeout, ui.DecodePercent),
		ChargeDischargePowerW:    getPtr(params.IP, eoj, ui.ChargeDischargePowerEPC, timeout, ui.DecodeSignedPowerW),
		CumulativeChargingKWh:    getPtr(params.IP, eoj, ui.CumulativeChargingEnergyEPC, timeout, ui.DecodeCumulativeEnergyKWh),
		CumulativeDischargingKWh: getPtr(params.IP, eoj, ui.CumulativeDischargingEnergyEPC, timeout, ui.DecodeCumulativeEnergyKWh),
		VehicleConnectionStatus:  getString(params.IP, eoj, ui.VehicleConnectionStatusEPC, timeout, ui.DecodeVehicleConnectionStatus),
	}

	if state.OperatingStatus == "" && state.OperationMode == "" && state.RemainingCapacityPercent == nil &&
		state.ChargeDischargePowerW == nil && state.CumulativeChargingKWh == nil &&
		state.CumulativeDischargingKWh == nil && state.VehicleConnectionStatus == "" {
		return errorResult(fmt.Sprintf("V2H(IP: %s, EOJ: %s)から表示可能なプロパティを取得できませんでした。", params.IP, state.EOJ)), nil, nil
	}

	return jsonResult(state)
}

type renderAirconUIParams struct {
	IP  string `json:"ip"  jsonschema:"家庭用エアコンのIPアドレス。例: 192.168.1.50"`
	EOJ string `json:"eoj" jsonschema:"対象オブジェクトのEOJコード(4〜6桁16進、家庭用エアコンクラス0130のみ対応)。例: 013001"`
}

type airconUIState struct {
	IP                string `json:"ip"`
	EOJ               string `json:"eoj"`
	OperatingStatus   string `json:"operating_status,omitempty"`
	OperationMode     string `json:"operation_mode,omitempty"`
	TargetTemperature *int   `json:"target_temperature,omitempty"`
	RoomTemperature   *int   `json:"room_temperature,omitempty"`
	AirFlowLevel      string `json:"air_flow_level,omitempty"`
}

func renderAirconUI(_ context.Context, _ *mcp.CallToolRequest, params *renderAirconUIParams) (*mcp.CallToolResult, any, error) {
	eoj, errRes := parseAndGuardEOJ(params.EOJ, "render_aircon_ui", "家庭用エアコン(EOJ 0130xx)", 0x01, 0x30)
	if errRes != nil {
		return errRes, nil, nil
	}

	const timeout = 5 * time.Second

	state := airconUIState{
		IP:                params.IP,
		EOJ:               fmt.Sprintf("%06X", eoj),
		OperatingStatus:   getString(params.IP, eoj, ui.OperatingStatusEPC, timeout, ui.DecodeOperatingStatus),
		OperationMode:     getString(params.IP, eoj, ui.AirconOperationModeEPC, timeout, ui.DecodeAirconOperationMode),
		TargetTemperature: getPtr(params.IP, eoj, ui.TargetTemperatureEPC, timeout, ui.DecodeTargetTemperature),
		RoomTemperature:   getPtr(params.IP, eoj, ui.RoomTemperatureEPC, timeout, ui.DecodeRoomTemperature),
		AirFlowLevel:      getString(params.IP, eoj, ui.AirFlowLevelEPC, timeout, ui.DecodeAirFlowLevel),
	}

	if state.OperatingStatus == "" && state.OperationMode == "" && state.TargetTemperature == nil &&
		state.RoomTemperature == nil && state.AirFlowLevel == "" {
		return errorResult(fmt.Sprintf("エアコン(IP: %s, EOJ: %s)から表示可能なプロパティを取得できませんでした。", params.IP, state.EOJ)), nil, nil
	}

	return jsonResult(state)
}
