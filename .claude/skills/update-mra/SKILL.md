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
2. MRA ページから最新の MRA zip の URL とバージョンを特定する（例: `https://echonet.jp/wp/wp-content/uploads/pdf/General/Standard/MRA/MRA_v1.4.0.zip`）

### 2. バージョン比較

`echonet/spec/mra/VERSION` の 1 行目（例: `MRA_v1.4.0`）と比較する。同じなら「既に最新」と報告して終了。

### 3. ダウンロードと展開

```bash
curl -sL -o /tmp/MRA.zip "<zip の URL>"
unzip -q -o /tmp/MRA.zip -d /tmp/mra
ls /tmp/mra   # MRA_vX.Y.Z/ ディレクトリ（devices/ superClass/ nodeProfile/ definitions/ metaData.json）
```

### 4. 対象ファイルの上書きコピー

収録対象は以下のみ（全クラスは取り込まない）:

```bash
M=/tmp/mra/MRA_vX.Y.Z
for c in 0x0011 0x0012 0x001B 0x0130 0x026B 0x0279 0x027C 0x027D 0x027E 0x0287 0x0288 0x0290 0x02A1; do
  cp "$M/devices/$c.json" echonet/spec/mra/
done
cp "$M/nodeProfile/0x0EF0.json" "$M/superClass/0x0000.json" \
   "$M/definitions/definitions.json" "$M/metaData.json" echonet/spec/mra/
```

`echonet/spec/mra/VERSION` を新バージョンと zip URL で更新する。

### 5. ビルドと動作確認

```bash
go build -o el-mcp-server .
```

HTTP モードで起動し、MCP 経由で確認する:
- `search_device_class` query="エアコン" → 0130 が返る
- `list_epc` eoj="0130" → スーパークラス EPC + クラス EPC が返る
- `get_epc_detail` eoj="0288" epc="E7" → 瞬時電力計測値が返る

パースに失敗する（起動時 panic する）場合は MRA の JSON スキーマが変わっている。`echonet/spec/load.go` の `mraDevice` / `mraProp` / `mraData` 構造体を新スキーマに合わせて修正する。

### 6. 差分確認とコミット

`git diff --stat` と主要な変更点（EPC の追加・削除・名称変更）をサマリしてユーザーに提示し、確認を得てからコミットする。

## あわせて確認すること

https://echonet.jp/spec_g/ で「第2部 ECHONET Lite 通信ミドルウェア仕様」の最新版も確認する（現行実装は Ver.1.14 準拠）。新版が出ていた場合、フレーム構造・UDP 仕様は `echonet/frame.go` / `echonet/udp.go` のコード実装のため自動更新はできない。README のリンク更新と、コード修正の要否をユーザーに報告するのみとする。

## 注意

- zip の URL・配布ページ URL は版ごとに変わる。ハードコードせず必ず `spec_g/` から辿る
- リリースノート PDF やガイドブック PDF はリポジトリに含めない
