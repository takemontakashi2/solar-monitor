# 開発ガイド

利用者向けの説明は [README](../README.md)、仕様は [SPEC.md](SPEC.md)、機器調査と外部データ連携は [ADVANCED.md](ADVANCED.md) を参照してください。

## 開発環境

Goの必要バージョンは [go.mod](../go.mod) を参照してください。SQLiteドライバーはPure Go実装を使用しています。

既存のリポジトリを取得して作業します。

```sh
git clone git@github.com:takemontakashi2/solar-monitor.git
cd solar-monitor
```

## データ取得処理・テスト・ビルド

実機のパワコン、HEMS、スマートメーターに合わせて `Source` インターフェースを実装します。

```go
type Source interface {
	Read(ctx context.Context) (Sample, error)
}
```

テストとビルドは次のコマンドで実行します。SQLiteはバイナリに含まれるため、実行時に別途導入する必要はありません。

```sh
go test ./...
go build -o solar-monitor .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o solar-monitor.exe .
GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 go build -o solar-monitor-linux-armv6 .
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o solar-monitor-linux-arm64 .
```


ビルド後の起動例:

```sh
./solar-monitor -source mock
./solar-monitor --version
```

`main.go` の `version` が表示用のバージョンです。Gitのコミット情報はGoのビルド情報から取得し、未コミットの変更を含む場合は `dirty` と表示します。

## リポジトリに含めないもの

実行データの `data/`、ビルド済みバイナリ、配布物の `dist/`、写真の展開先 `photos/` は `.gitignore` で除外しています。バイナリはGitへのpushだけでは配布されません。

## リリース手順

1. `main.go` のバージョンを更新し、対応する `docs/releases/vX.Y.Z.md` に変更点・利用方法・確認状況を記載します。
2. テストと静的チェックを実行し、変更をコミットします。
3. バージョンと一致する注釈付きタグを作成し、コミットとタグをpushします。

```sh
go test ./...
go vet ./...
# X.Y.Zをリリースするバージョンに置き換える
git tag -a vX.Y.Z -m "Solar Monitor X.Y.Z"
git push --atomic origin main vX.Y.Z
```

[リリースワークフロー](../.github/workflows/release.yml) がテスト・Windows x64版とLinux ARMv6 / ARM64版のビルドを実行し、exe・Linux実行ファイル・zip・tar.gz・SHA-256を添付したドラフトリリースを作成します。自動で実行されない場合はGitHub Actionsの「Prepare release」で「Run workflow」を選び、既存のタグを指定します。

Actionsの成功、添付ファイル、Windowsでのバージョン表示と起動、リリースノートを確認してからGitHub上で公開します。公開済みタグの付け替えはせず、修正は次のバージョンで配布してください。

0.9.9は公開承認済みのため、ワークフローが配布物をアップロードした後に自動公開します。他のタグはドラフト作成までです。

## 保存のバッファリング

`-flush-interval` が正の場合、Storeは未保存のサンプルを保持し、読み出し時にDBの記録と統合します。flush時はサンプルとプロパティを1つのトランザクションにまとめ、成功した分だけバッファから除去します。固定項目の状態はサンプルのコミット後に保存します。API読み出しとflushをロックで同期し、重複表示や保存切り替え時の欠落を防ぎます。

保存タイマーは収集処理と独立しています。通常停止では収集とタイマーの終了を待ち、最後のflushを実行します。バッファ上限・保存失敗・再試行・日付境界・未保存データのAPI表示は `buffer_test.go` で確認します。
