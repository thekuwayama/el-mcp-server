---
name: update-manufacturer-codes
description: echonet.jp のメーカーコード一覧 XLSX を取得し、echonet/spec/manufacturers/codes.json を更新する
---

# メーカーコード一覧更新

`echonet/spec/manufacturers/codes.json` に収録しているメーカーコード一覧を、echonet.jp で公開されている最新版に更新する。

## 手順

### 1. 現在のバージョン確認

`echonet/spec/manufacturers/VERSION` の 1 行目（例: `list_code.xlsx (2026-07-19)`）の日付を確認する。最近更新済みであれば作業不要。

### 2. 再生成

```bash
make manufacturers
```

`VERSION` の 2 行目の URL から XLSX を取得し、`cmd/gen-manufacturers` で `codes.json` を上書き生成する。`codes.json` に差分があるときだけ `VERSION` の日付を JST で更新する。

URL が変わっていた場合は echonet.jp のメーカーコード配布ページから最新の XLSX URL を取得し、`VERSION` の 2 行目を書き換えてから実行する。

### 3. ビルドと動作確認

```bash
go build -o el-mcp-server .
```

`get_property` で EPC 0x8A を取得した際に `manufacturer_name` フィールドが正しく返ることを確認する。

### 4. 差分確認とコミット

```bash
git diff --stat echonet/spec/manufacturers/
```

追加・変更されたメーカー数をサマリしてユーザーに提示し、確認を得てからコミットする。

## 注意

- `cmd/gen-manufacturers` は `archive/zip` + `encoding/xml` のみで XLSX を解析する（外部ライブラリ不要）
- XLSX の `<rPh>` 要素はふりがな（ルビ）なので除外している。除外しないと社名にカタカナ読みが混入する
- XLSX の列構造が変わった場合（コード列が A 列・6 桁 16 進数でなくなった等）は `cmd/gen-manufacturers/main.go` を調整する
- コードは 3 バイト = 6 桁 16 進数（大文字）で保存する
