package tools

import (
	"context"
	_ "embed"
	"fmt"
	"math"
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

//go:embed ui/templates/smartmeter.html
var smartmeterDashboardHTML string

const batteryDashboardURI = "ui://el-mcp-server/battery"
const solarDashboardURI = "ui://el-mcp-server/solar"
const v2hDashboardURI = "ui://el-mcp-server/v2h"
const airconDashboardURI = "ui://el-mcp-server/aircon"
const smartmeterDashboardURI = "ui://el-mcp-server/smartmeter"

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

	s.AddResource(&mcp.Resource{
		URI:      smartmeterDashboardURI,
		Name:     "smartmeter_dashboard",
		Title:    "スマートメーターダッシュボード",
		MIMEType: MCPAppsUIMimeType,
	}, dashboardResourceHandler(smartmeterDashboardHTML))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "render_smartmeter_ui",
		Description: "指定した低圧スマート電力量メータ(EOJ 0288xx)の現在状態(稼働状態・瞬時電力・瞬時電流(R相/T相)・積算電力量(正方向))と、30分毎の買電/売電電力量の履歴(daysで指定した日数分、デフォルトは今日のみ)を取得し、MCP Apps対応クライアントではダッシュボードUIとして表示します。履歴は棒グラフで表示されます。履歴の読み出しには積算履歴収集日(EPC E5)への書き込みが必要ですが、取得後に元の値へ戻します。このダッシュボードは操作系のコントロールを持たない読み取り専用ダッシュボードです。",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Meta: mcp.Meta{
			"ui": map[string]any{
				"resourceUri": smartmeterDashboardURI,
			},
		},
	}, renderSmartmeterUI)
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

// maxSmartmeterHistoryDays caps render_smartmeter_ui's days parameter. Each
// day costs three UDP round trips (Set 0xE5, Get 0xE2, Get 0xE4) and ~2.5KB
// of JSON, so the cap keeps both latency and the result size reasonable.
const maxSmartmeterHistoryDays = 7

type renderSmartmeterUIParams struct {
	IP   string `json:"ip"  jsonschema:"低圧スマート電力量メータのIPアドレス。例: 192.168.1.50"`
	EOJ  string `json:"eoj" jsonschema:"対象オブジェクトのEOJコード(4〜6桁16進、低圧スマート電力量メータクラス0288のみ対応)。例: 028801"`
	Days int    `json:"days,omitempty" jsonschema:"30分毎の電力量履歴を何日分取得するか(今日を含む、1〜7、デフォルト1=今日のみ)。例: 7 なら6日前〜今日"`
}

type smartmeterUIState struct {
	IP                   string                 `json:"ip"`
	EOJ                  string                 `json:"eoj"`
	OperatingStatus      string                 `json:"operating_status,omitempty"`
	InstantaneousPowerW  *int32                 `json:"instantaneous_power_w,omitempty"`
	InstantaneousRPhaseA *float64               `json:"instantaneous_r_phase_a,omitempty"`
	InstantaneousTPhaseA *float64               `json:"instantaneous_t_phase_a,omitempty"`
	CumulativeEnergyKWh  *float64               `json:"cumulative_energy_kwh,omitempty"`
	History              []smartmeterHistoryDay `json:"history,omitempty"`
}

// smartmeterHistoryDay is one day of half-hourly energy, oldest day first in
// smartmeterUIState.History.
type smartmeterHistoryDay struct {
	DaysAgo        int                     `json:"days_ago"`
	Date           string                  `json:"date"`
	Slots          []smartmeterHistorySlot `json:"slots"`
	TotalImportKWh *float64                `json:"total_import_kwh,omitempty"`
	TotalExportKWh *float64                `json:"total_export_kwh,omitempty"`
}

// smartmeterHistorySlot is the energy bought (import, EPC 0xE2) and sold
// (export, EPC 0xE4) in the half hour starting at Time. A nil value means the
// meter has no data for that slot.
type smartmeterHistorySlot struct {
	Time      string   `json:"time"`
	ImportKWh *float64 `json:"import_kwh,omitempty"`
	ExportKWh *float64 `json:"export_kwh,omitempty"`
}

func renderSmartmeterUI(_ context.Context, _ *mcp.CallToolRequest, params *renderSmartmeterUIParams) (*mcp.CallToolResult, any, error) {
	eoj, errRes := parseAndGuardEOJ(params.EOJ, "render_smartmeter_ui", "低圧スマート電力量メータ(EOJ 0288xx)", 0x02, 0x88)
	if errRes != nil {
		return errRes, nil, nil
	}

	days := params.Days
	if days == 0 {
		days = 1
	}
	if days < 1 || days > maxSmartmeterHistoryDays {
		return errorResult(fmt.Sprintf("days は1〜%dで指定してください。指定値: %d", maxSmartmeterHistoryDays, params.Days)), nil, nil
	}

	const timeout = 5 * time.Second

	state := smartmeterUIState{
		IP:                  params.IP,
		EOJ:                 fmt.Sprintf("%06X", eoj),
		OperatingStatus:     getString(params.IP, eoj, ui.OperatingStatusEPC, timeout, ui.DecodeOperatingStatus),
		InstantaneousPowerW: getPtr(params.IP, eoj, ui.InstantaneousElectricPowerEPC, timeout, ui.DecodeInstantaneousPowerW),
	}

	if currents := getPtr(params.IP, eoj, ui.InstantaneousCurrentsEPC, timeout, ui.DecodeInstantaneousCurrents); currents != nil {
		state.InstantaneousRPhaseA = &currents.RPhaseA
		state.InstantaneousTPhaseA = &currents.TPhaseA
	}

	// The coefficient (0xD3) and unit (0xE1) scale both the current cumulative
	// reading and the history, so they are fetched once up front.
	unitFactor := getPtr(params.IP, eoj, ui.CumulativeElectricEnergyUnitEPC, timeout, ui.DecodeEnergyUnit)
	if unitFactor != nil {
		coefficient := getPtr(params.IP, eoj, ui.CoefficientEPC, timeout, ui.DecodeCoefficient)
		if rawEnergy := getPtr(params.IP, eoj, ui.CumulativeElectricEnergyEPC, timeout, ui.DecodeRawCumulativeEnergy); rawEnergy != nil {
			kwh := ui.CumulativeEnergyKWh(*rawEnergy, coefficient, *unitFactor)
			state.CumulativeEnergyKWh = &kwh
		}
		state.History = fetchSmartmeterHistory(params.IP, eoj, days, coefficient, *unitFactor, time.Now(), timeout)
	}

	if state.OperatingStatus == "" && state.InstantaneousPowerW == nil && state.InstantaneousRPhaseA == nil &&
		state.CumulativeEnergyKWh == nil && len(state.History) == 0 {
		return errorResult(fmt.Sprintf("低圧スマート電力量メータ(IP: %s, EOJ: %s)から表示可能なプロパティを取得できませんでした。", params.IP, state.EOJ)), nil, nil
	}

	return jsonResult(state)
}

// fetchSmartmeterHistory reads the half-hourly energy logs for today and the
// preceding days-1 days. For each day it selects the collection day with a
// SetC on EPC 0xE5 and then reads EPC 0xE2 (import) and 0xE4 (export). The
// collection day is meter-wide state another controller (e.g. a HEMS) may rely
// on, so the original value is written back when done. Days whose log cannot
// be read, or whose log reports a different day than requested (another
// controller changed 0xE5 in between), are left out.
func fetchSmartmeterHistory(ip string, eoj uint32, days int, coefficient *uint32, unitFactor float64, now time.Time, timeout time.Duration) []smartmeterHistoryDay {
	originalDay, err := echonet.GetProperty(ip, eoj, ui.EnergyLogDayEPC, timeout)
	if err != nil {
		return nil
	}
	defer func() {
		_ = echonet.SetProperty(ip, eoj, ui.EnergyLogDayEPC, originalDay, timeout)
	}()

	readLog := func(epc byte, daysAgo int) *ui.EnergyLog {
		edt, err := echonet.GetProperty(ip, eoj, epc, timeout)
		if err != nil {
			return nil
		}
		log, ok := ui.DecodeEnergyLog(edt)
		if !ok || log.Day != daysAgo {
			return nil
		}
		return &log
	}

	importLogs := make([]*ui.EnergyLog, days)
	exportLogs := make([]*ui.EnergyLog, days)
	// EPC 0xE4 is only required on meters measuring the reverse direction.
	// Once it fails, stop asking so a meter without it doesn't cost a timeout
	// per day.
	exportSupported := true
	for d := range days {
		if err := echonet.SetProperty(ip, eoj, ui.EnergyLogDayEPC, ui.EncodeEnergyLogDay(d), timeout); err != nil {
			continue
		}
		importLogs[d] = readLog(ui.NormalDirectionEnergyLogEPC, d)
		if exportSupported {
			exportLogs[d] = readLog(ui.ReverseDirectionEnergyLogEPC, d)
			exportSupported = exportLogs[d] != nil
		}
	}

	// Each day's last slot (23:30-24:00) ends at the following day's 00:00
	// reading, i.e. the log one day fewer ago.
	slotEnergies := func(logs []*ui.EnergyLog, d int) *[ui.SlotsPerDay]*float64 {
		if logs[d] == nil {
			return nil
		}
		var next *ui.EnergyLog
		if d > 0 {
			next = logs[d-1]
		}
		energies := ui.SlotEnergiesKWh(*logs[d], next, coefficient, unitFactor)
		return &energies
	}

	var history []smartmeterHistoryDay
	for d := days - 1; d >= 0; d-- {
		imports := slotEnergies(importLogs, d)
		exports := slotEnergies(exportLogs, d)
		if imports == nil && exports == nil {
			continue
		}
		day := smartmeterHistoryDay{
			DaysAgo: d,
			Date:    now.AddDate(0, 0, -d).Format("2006-01-02"),
			Slots:   make([]smartmeterHistorySlot, ui.SlotsPerDay),
		}
		for i := range ui.SlotsPerDay {
			slot := smartmeterHistorySlot{Time: fmt.Sprintf("%02d:%02d", i/2, i%2*30)}
			if imports != nil {
				slot.ImportKWh = imports[i]
			}
			if exports != nil {
				slot.ExportKWh = exports[i]
			}
			day.Slots[i] = slot
		}
		day.TotalImportKWh = sumKWh(imports)
		day.TotalExportKWh = sumKWh(exports)
		history = append(history, day)
	}
	return history
}

// sumKWh totals the slots that have data, returning nil when energies is nil
// or no slot has data.
func sumKWh(energies *[ui.SlotsPerDay]*float64) *float64 {
	if energies == nil {
		return nil
	}
	var total float64
	found := false
	for _, e := range energies {
		if e != nil {
			total += *e
			found = true
		}
	}
	if !found {
		return nil
	}
	total = math.Round(total*10000) / 10000
	return &total
}
