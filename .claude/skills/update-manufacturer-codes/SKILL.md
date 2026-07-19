---
name: update-manufacturer-codes
description: echonet.jp のメーカーコード一覧 XLSX を取得し、echonet/spec/manufacturers/codes.json を更新する
---

# メーカーコード一覧更新

`echonet/spec/manufacturers/codes.json` に収録しているメーカーコード一覧を、echonet.jp で公開されている最新版に更新する。

## 手順

### 1. 現在のバージョン確認

`echonet/spec/manufacturers/VERSION` の 1 行目（例: `list_code.xlsx (2026-07-19)`）の日付を確認する。最近更新済みであれば作業不要。

### 2. XLSX のダウンロード

```bash
curl -sL -o /tmp/list_code.xlsx \
  "https://echonet.jp/wp/wp-content/uploads/pdf/General/Echonet/ManufacturerCode/list_code.xlsx"
file /tmp/list_code.xlsx   # "Microsoft Excel 2007+" であることを確認
```

URL が変わっていた場合は echonet.jp のメーカーコード配布ページから最新の XLSX URL を取得する。

### 3. JSON 生成

`cmd/gen-manufacturers` が XLSX を解析して `echonet/spec/manufacturers/codes.json` を上書き生成する:

```bash
go run ./cmd/gen-manufacturers /tmp/list_code.xlsx
```

### 4. VERSION 更新

```bash
DATE=$(date +%Y-%m-%d)
printf "list_code.xlsx (%s)\nhttps://echonet.jp/wp/wp-content/uploads/pdf/General/Echonet/ManufacturerCode/list_code.xlsx\n" \
  "$DATE" > echonet/spec/manufacturers/VERSION
```

### 5. ビルドと動作確認

```bash
go build -o el-mcp-server .
```

`get_property` で EPC 0x8A を取得した際に `manufacturer_name` フィールドが正しく返ることを確認する。

### 6. 差分確認とコミット

```bash
git diff --stat echonet/spec/manufacturers/
```

追加・変更されたメーカー数をサマリしてユーザーに提示し、確認を得てからコミットする。

## 注意

- `cmd/gen-manufacturers` は `archive/zip` + `encoding/xml` のみで XLSX を解析する（外部ライブラリ不要）
- XLSX の `<rPh>` 要素はふりがな（ルビ）なので除外している。除外しないと社名にカタカナ読みが混入する
- XLSX の列構造が変わった場合（コード列が A 列・6 桁 16 進数でなくなった等）は `cmd/gen-manufacturers/main.go` を調整する
- コードは 3 バイト = 6 桁 16 進数（大文字）で保存する
