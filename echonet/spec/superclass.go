package spec

// SuperClassEPCs are the common EPCs defined in the Device Object Super Class.
// These apply to all ECHONET Lite device objects (Appendix Release R).
var SuperClassEPCs = []EPCDef{
	{0x80, "動作状態", "Operation status", "unsigned char", "", "Anno/Set/Get", "ON=0x30, OFF=0x31"},
	{0x81, "設置場所", "Installation location", "unsigned char / byte[]", "", "Anno/Set/Get", "設置場所コードまたは自由記述"},
	{0x82, "規格Version情報", "Standard version information", "unsigned char[4]", "", "Get", "対応規格のリリース情報"},
	{0x83, "識別番号", "Identification number", "unsigned char[17]", "", "Get", "機器固有の識別番号"},
	{0x84, "瞬時消費電力計測値", "Instantaneous electric energy consumption", "unsigned short", "W", "Get", "現在の消費電力"},
	{0x85, "積算消費電力計測値", "Cumulative electric energy consumption", "unsigned long", "0.001 kWh", "Get", "積算消費電力量"},
	{0x86, "メーカ異常コード", "Manufacturer fault code", "unsigned char[MAX 225]", "", "Get", "メーカ固有の異常コード"},
	{0x87, "電流制限設定", "Current limit setting", "unsigned char", "%", "Set/Get", "0〜100%"},
	{0x88, "異常発生状態", "Fault status", "unsigned char", "", "Anno/Get", "異常あり=0x41, なし=0x42"},
	{0x89, "異常内容", "Fault description", "unsigned short", "", "Get", "異常の種別コード"},
	{0x8A, "メーカコード", "Manufacturer code", "unsigned char[3]", "", "Get", "ECHONET Lite割当メーカコード"},
	{0x8B, "事業場コード", "Business facility code", "unsigned char[3]", "", "Get", ""},
	{0x8C, "商品コード", "Product code", "unsigned char[12]", "", "Get", ""},
	{0x8D, "製造番号", "Production number", "unsigned char[12]", "", "Get", ""},
	{0x8E, "製造年月日", "Production date", "unsigned char[4]", "", "Get", "YYYYMMDD"},
	{0x8F, "節電動作設定", "Power-saving operation setting", "unsigned char", "", "Set/Get", "節電動作中=0x41, 通常動作中=0x42"},
	{0x93, "遠隔操作設定", "Remote control setting", "unsigned char", "", "Set/Get", "遠隔操作可=0x41, 不可=0x42"},
	{0x97, "現在時刻設定", "Current time setting", "unsigned char[2]", "", "Set/Get", "HH MM"},
	{0x98, "現在年月日設定", "Current date setting", "unsigned char[4]", "", "Set/Get", "YYYY MM DD"},
	{0x99, "電力制限設定", "Power limit setting", "unsigned short", "W", "Set/Get", ""},
	{0x9A, "積算運転時間", "Cumulative operating time", "unsigned char[5]", "", "Get", "単位+時間"},
	{0x9D, "状変アナウンスプロパティマップ", "Status change announcement property map", "unsigned char[MAX 17]", "", "Get", "通知対象プロパティ一覧"},
	{0x9E, "Setプロパティマップ", "Set property map", "unsigned char[MAX 17]", "", "Get", "書き込み可能プロパティ一覧"},
	{0x9F, "Getプロパティマップ", "Get property map", "unsigned char[MAX 17]", "", "Get", "読み取り可能プロパティ一覧"},
}
