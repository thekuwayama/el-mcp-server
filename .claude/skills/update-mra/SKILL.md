---
name: update-mra
description: echonet.jp から最新の MRA (Machine Readable Appendix) を取得し、echonet/spec/mra/ の仕様データを更新する
---

# MRA 仕様データ更新

`echonet/spec/mra/` に収録している ECHONET Lite Appendix の機械可読データ（MRA JSON）を、echonet.jp で公開されている最新版に更新する。

## 手順

### 1. 最新版の発見

直リンクは版ごとに URL が変わるため、必ず仕様総合ページから辿る:

1. https://echonet.jp/spec_g/ を WebFetch し、「Appendix ECHONET 機器オブジェクト詳細規定」の MRA（Machine Readable Appendix）ページへのリンクを探す（URL パターン: `https://echonet.jp/spec_mra_rrN/`。N はリリース改定ごとに増える）
2. MRA ページから最新の MRA zip の URL とバージョン文字列（例: `MRA_v1.4.0`）を特定する

### 2. 更新ツールの実行

`cmd/update-mra` が VERSION 比較・ダウンロード・展開・コピーをまとめて行う:

```bash
go run ./cmd/update-mra \
  "https://echonet.jp/wp/wp-content/uploads/pdf/General/Standard/MRA/MRA_vX.Y.Z.zip" \
  MRA_vX.Y.Z
```

「already up to date」と表示された場合は終了。ファイルが更新された場合は次のステップへ。

### 3. ビルドと動作確認

```bash
go build -o el-mcp-server .
```

HTTP モードで起動し、MCP 経由で確認する:
- `search_device_class` query="エアコン" → 0130 が返る
- `list_epc` eoj="0130" → スーパークラス EPC + クラス EPC が返る
- `get_epc_detail` eoj="0288" epc="E7" → 瞬時電力計測値が返る

起動時に panic する場合は MRA の JSON スキーマが変わっている。`echonet/spec/load.go` の `mraDevice` / `mraProp` / `mraData` 構造体を新スキーマに合わせて修正する。

### 4. 差分確認とコミット

```bash
git diff --stat echonet/spec/mra/
```

主要な変更点（EPC の追加・削除・名称変更）をサマリしてユーザーに提示し、確認を得てからコミットする。

## あわせて確認すること

https://echonet.jp/spec_g/ で「第2部 ECHONET Lite 通信ミドルウェア仕様」の最新版も確認する（現行実装は Ver.1.14 準拠）。新版が出ていた場合、フレーム構造・UDP 仕様は `echonet/frame.go` / `echonet/udp.go` のコード実装のため自動更新はできない。README のリンク更新と、コード修正の要否をユーザーに報告するのみとする。

## 注意

- zip の URL・配布ページ URL は版ごとに変わる。ハードコードせず必ず `spec_g/` から辿る
- `cmd/update-mra` は収録対象 14 クラス + スーパークラス + nodeProfile + definitions + metaData のみをコピーする。全クラスは取り込まない
- リリースノート PDF やガイドブック PDF はリポジトリに含めない
