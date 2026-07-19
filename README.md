# el-mcp-server

ECHONET Lite の情報を AI から利用可能にする、読み取り専用の MCP (Model Context Protocol) サーバーです。Go で実装しています。

## 提供する MCP ツール

すべて読み取り専用（`ReadOnlyHint: true`）です。

### 仕様検索（静的データ）

ECHONET Lite Appendix Release R をもとにした静的データを検索します。

| ツール | 概要 |
|---|---|
| `search_device_class` | 名前・キーワード・EOJ コードで機器クラスを検索 |
| `list_epc` | 機器クラスの EPC（プロパティコード）一覧を取得 |
| `get_epc_detail` | 特定 EPC の詳細（データ型・単位・アクセス規則）を取得 |

収録機器クラス: ノードプロファイル / 温度・湿度・CO2 センサ / 家庭用エアコン / 電気温水器 / 太陽光発電 / 燃料電池 / 蓄電池 / EV 充放電器 / 分電盤メタリング / 低圧スマート電力量メータ / 一般照明 / EV 充電器（全 14 クラス + スーパークラス共通 EPC）

### ネットワーク（UDP 通信）

同一 LAN 上の ECHONET Lite 機器と UDP（ポート 3610）で通信します。

| ツール | 概要 |
|---|---|
| `discover_devices` | マルチキャスト（224.0.23.0）で LAN 内の機器を探索 |
| `get_property` | 指定機器の EPC プロパティ値を Get で取得 |

### 製品検索（HTTP）

| ツール | 概要 |
|---|---|
| `search_certified_products` | [echonet.jp](https://echonet.jp/product/echonet-lite/) の認証登録製品を検索 |

## ビルド

```bash
go build -o el-mcp-server .
```

## 起動

```bash
# stdio モード（デフォルト）
./el-mcp-server

# HTTP モード（Streamable HTTP）
./el-mcp-server -transport http -addr :8080
```

## Claude Code への登録

```bash
claude mcp add el-mcp-server -- /path/to/el-mcp-server
```

登録後、Claude に「LAN 内の ECHONET Lite 機器を探して」「スマートメーターの EPC 一覧を教えて」のように話しかけると各ツールが呼び出されます。

## データソース

- [ECHONET Lite 規格書 Ver.1.14](https://echonet.jp/spec_v114_lite/) — フレーム構造・UDP 通信仕様
- [Appendix ECHONET 機器オブジェクト詳細規定 Release R](https://echonet.jp/spec_object_rr/) — 機器クラス・EPC 定義（静的データとして収録）
- [ECHONET Lite 認証製品検索](https://echonet.jp/product/echonet-lite/) — `search_certified_products` が実行時に取得

## 制限事項

- 読み取り専用です。機器への書き込み（SetC/SetI）は実装していません
- `search_certified_products` の検索パラメータは echonet.jp のフォーム仕様に依存するため、絞り込みが効かない場合があります
