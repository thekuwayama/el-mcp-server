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
		Description: "指定した低圧スマート電力量メータ(EOJ 0288xx)の現在状態(稼働状態・瞬時電力・瞬時電流(R相/T相)・積算電力量(正方向))と、指定日(dateで指定、省略時は今日)の30分毎の買電/売電電力量の履歴を取得し、MCP Apps対応クライアントではダッシュボードUIとして表示します。履歴は棒グラフで表示され、ダッシュボード上の前日/翌日ボタンで表示日を切り替えられます(内部でrender_smartmeter_uiを再度呼び出します)。履歴の読み出しには積算履歴収集日(EPC E5)への書き込みが必要ですが、取得後に元の値へ戻します。このダッシュボードは操作系のコントロールを持たない読み取り専用ダッシュボードです。",
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

// smartmeterHistoryDateLayout is the format of render_smartmeter_ui's date
// parameter and of the date it reports back.
const smartmeterHistoryDateLayout = "2006-01-02"

type renderSmartmeterUIParams struct {
	IP   string `json:"ip"  jsonschema:"低圧スマート電力量メータのIPアドレス。例: 192.168.1.50"`
	EOJ  string `json:"eoj" jsonschema:"対象オブジェクトのEOJコード(4〜6桁16進、低圧スマート電力量メータクラス0288のみ対応)。例: 028801"`
	Date string `json:"date,omitempty" jsonschema:"30分毎の電力量履歴を表示する日付(YYYY-MM-DD)。省略時は今日。今日から99日前まで指定可能。例: 2026-11-14"`
	// HistoryOnly is set by the dashboard's previous/next-day buttons, which
	// only need the history; skipping the current-state properties saves four
	// Get round trips per day change.
	HistoryOnly bool `json:"history_only,omitempty" jsonschema:"trueの場合は履歴(EPC E2/E4)とその換算に必要なEPC D3/E1のみ取得し、稼働状態・瞬時電力・瞬時電流・積算電力量は取得しない。ダッシュボードの前日/翌日ボタンが使用する。省略時はfalse"`
}

type smartmeterUIState struct {
	IP                   string   `json:"ip"`
	EOJ                  string   `json:"eoj"`
	OperatingStatus      string   `json:"operating_status,omitempty"`
	InstantaneousPowerW  *int32   `json:"instantaneous_power_w,omitempty"`
	InstantaneousRPhaseA *float64 `json:"instantaneous_r_phase_a,omitempty"`
	InstantaneousTPhaseA *float64 `json:"instantaneous_t_phase_a,omitempty"`
	CumulativeEnergyKWh  *float64 `json:"cumulative_energy_kwh,omitempty"`
	// History always names the requested day; its slots are omitted when the
	// log could not be read, with HistoryError explaining why (e.g. the meter
	// returned the log of a different day than requested).
	History      smartmeterHistoryDay `json:"history"`
	HistoryError string               `json:"history_error,omitempty"`
}

// smartmeterHistoryDay is one day of half-hourly energy. Slots is nil when
// the day's log could not be read.
type smartmeterHistoryDay struct {
	Date           string                  `json:"date"`
	DaysAgo        int                     `json:"days_ago"`
	Slots          []smartmeterHistorySlot `json:"slots,omitempty"`
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

// parseHistoryDate resolves render_smartmeter_ui's date parameter against now
// (local time) into the number of days ago EPC 0xE5 expects. An empty date
// means today. Dates in the future or beyond what EPC 0xE5 can select are
// rejected.
func parseHistoryDate(date string, now time.Time) (int, error) {
	if date == "" {
		return 0, nil
	}
	target, err := time.ParseInLocation(smartmeterHistoryDateLayout, date, now.Location())
	if err != nil {
		return 0, fmt.Errorf("date の形式が正しくありません(YYYY-MM-DDで指定してください): %s", date)
	}
	// Count calendar days via UTC midnights so a DST shift can't skew the
	// division.
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	ty, tm, td := target.Date()
	daysAgo := int(today.Sub(time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC)).Hours() / 24)
	switch {
	case daysAgo < 0:
		return 0, fmt.Errorf("未来の日付は指定できません: %s", date)
	case daysAgo > ui.MaxEnergyLogDay:
		return 0, fmt.Errorf("date は今日から%d日前までの日付を指定してください: %s", ui.MaxEnergyLogDay, date)
	}
	return daysAgo, nil
}

func renderSmartmeterUI(_ context.Context, _ *mcp.CallToolRequest, params *renderSmartmeterUIParams) (*mcp.CallToolResult, any, error) {
	eoj, errRes := parseAndGuardEOJ(params.EOJ, "render_smartmeter_ui", "低圧スマート電力量メータ(EOJ 0288xx)", 0x02, 0x88)
	if errRes != nil {
		return errRes, nil, nil
	}

	now := time.Now()
	daysAgo, err := parseHistoryDate(params.Date, now)
	if err != nil {
		return errorResult(err.Error()), nil, nil
	}

	const timeout = 5 * time.Second

	state := smartmeterUIState{
		IP:  params.IP,
		EOJ: fmt.Sprintf("%06X", eoj),
		History: smartmeterHistoryDay{
			Date:    now.AddDate(0, 0, -daysAgo).Format(smartmeterHistoryDateLayout),
			DaysAgo: daysAgo,
		},
	}

	if !params.HistoryOnly {
		state.OperatingStatus = getString(params.IP, eoj, ui.OperatingStatusEPC, timeout, ui.DecodeOperatingStatus)
		state.InstantaneousPowerW = getPtr(params.IP, eoj, ui.InstantaneousElectricPowerEPC, timeout, ui.DecodeInstantaneousPowerW)
		if currents := getPtr(params.IP, eoj, ui.InstantaneousCurrentsEPC, timeout, ui.DecodeInstantaneousCurrents); currents != nil {
			state.InstantaneousRPhaseA = &currents.RPhaseA
			state.InstantaneousTPhaseA = &currents.TPhaseA
		}
	}

	// The coefficient (0xD3) and unit (0xE1) scale both the current cumulative
	// reading and the history, so they are fetched once up front.
	unitFactor := getPtr(params.IP, eoj, ui.CumulativeElectricEnergyUnitEPC, timeout, ui.DecodeEnergyUnit)
	if unitFactor != nil {
		coefficient := getPtr(params.IP, eoj, ui.CoefficientEPC, timeout, ui.DecodeCoefficient)
		if !params.HistoryOnly {
			if rawEnergy := getPtr(params.IP, eoj, ui.CumulativeElectricEnergyEPC, timeout, ui.DecodeRawCumulativeEnergy); rawEnergy != nil {
				kwh := ui.CumulativeEnergyKWh(*rawEnergy, coefficient, *unitFactor)
				state.CumulativeEnergyKWh = &kwh
			}
		}
		if err := fetchSmartmeterHistory(params.IP, eoj, &state.History, coefficient, *unitFactor, timeout); err != nil {
			state.HistoryError = err.Error()
		}
	} else {
		state.HistoryError = "積算電力量単位(EPC E1)を取得できないため、履歴を電力量に換算できませんでした。"
	}

	// In history_only mode the caller (the dashboard) already shows the current
	// state, so a missing history is reported via history_error rather than as
	// a failed call, letting the dashboard move to the new day and show why.
	if !params.HistoryOnly && state.OperatingStatus == "" && state.InstantaneousPowerW == nil && state.InstantaneousRPhaseA == nil &&
		state.CumulativeEnergyKWh == nil && state.History.Slots == nil {
		return errorResult(fmt.Sprintf("低圧スマート電力量メータ(IP: %s, EOJ: %s)から表示可能なプロパティを取得できませんでした。", params.IP, state.EOJ)), nil, nil
	}

	return jsonResult(state)
}

// fetchSmartmeterHistory fills day's slots and totals from the half-hourly
// energy log of the day day.DaysAgo days ago. For each log it selects the collection day with a SetC on EPC
// 0xE5 and then reads EPC 0xE2 (import) and 0xE4 (export). The day after is
// read too, only for its 00:00 reading, which closes the 23:30-24:00 slot.
// The collection day is meter-wide state another controller (e.g. a HEMS) may
// rely on, so the original value is written back when done.
func fetchSmartmeterHistory(ip string, eoj uint32, day *smartmeterHistoryDay, coefficient *uint32, unitFactor float64, timeout time.Duration) error {
	daysAgo := day.DaysAgo
	originalDay, err := echonet.GetProperty(ip, eoj, ui.EnergyLogDayEPC, timeout)
	if err != nil {
		return fmt.Errorf("積算履歴収集日(EPC E5)を取得できませんでした: %w", err)
	}
	defer func() {
		_ = echonet.SetProperty(ip, eoj, ui.EnergyLogDayEPC, originalDay, timeout)
	}()

	// readLogs selects day d and reads its import and (if wantExport) export
	// logs. A log whose embedded day differs from d means 0xE5 didn't take
	// effect (e.g. another controller changed it in between, or the meter
	// ignores it), so it is reported as an error rather than shown under the
	// wrong date.
	readLogs := func(d int, wantExport bool) (imp, exp *ui.EnergyLog, err error) {
		if err := echonet.SetProperty(ip, eoj, ui.EnergyLogDayEPC, ui.EncodeEnergyLogDay(d), timeout); err != nil {
			return nil, nil, fmt.Errorf("積算履歴収集日(EPC E5)を%d日前に設定できませんでした: %w", d, err)
		}
		read := func(epc byte) (*ui.EnergyLog, error) {
			edt, err := echonet.GetProperty(ip, eoj, epc, timeout)
			if err != nil {
				return nil, nil // not supported (e.g. 0xE4 on an import-only meter)
			}
			log, ok := ui.DecodeEnergyLog(edt)
			if !ok {
				return nil, fmt.Errorf("EPC %02X の履歴データの形式が正しくありません", epc)
			}
			if log.Day != d {
				return nil, fmt.Errorf("メーターが返した履歴の日付(%d日前)が要求した日付(%d日前)と一致しませんでした。メーターが積算履歴収集日(EPC E5)の指定に対応していない可能性があります", log.Day, d)
			}
			return &log, nil
		}
		if imp, err = read(ui.NormalDirectionEnergyLogEPC); err != nil {
			return nil, nil, err
		}
		if wantExport {
			if exp, err = read(ui.ReverseDirectionEnergyLogEPC); err != nil {
				return nil, nil, err
			}
		}
		return imp, exp, nil
	}

	importLog, exportLog, err := readLogs(daysAgo, true)
	if err != nil {
		return err
	}
	if importLog == nil && exportLog == nil {
		return fmt.Errorf("積算電力量計測値履歴(EPC E2/E4)を取得できませんでした")
	}

	// The following day's log only contributes its 00:00 reading; if it can't
	// be read, the 23:30 slot is simply left without data.
	var nextImport, nextExport *ui.EnergyLog
	if daysAgo > 0 {
		nextImport, nextExport, _ = readLogs(daysAgo-1, exportLog != nil)
	}

	var imports, exports *[ui.SlotsPerDay]*float64
	if importLog != nil {
		e := ui.SlotEnergiesKWh(*importLog, nextImport, coefficient, unitFactor)
		imports = &e
	}
	if exportLog != nil {
		e := ui.SlotEnergiesKWh(*exportLog, nextExport, coefficient, unitFactor)
		exports = &e
	}

	day.Slots = make([]smartmeterHistorySlot, ui.SlotsPerDay)
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
	return nil
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
