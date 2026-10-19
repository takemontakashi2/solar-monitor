# Solar Monitor

Pure Go SQLiteで記録する、家庭用太陽光発電の記録・表示サーバーです。
単一バイナリとしてビルドでき、タブレットからブラウザで閲覧できます。

要望と仕様の整理は [docs/SPEC.md](docs/SPEC.md) にまとめています。
実機調査の詳細は [docs/echonet-findings.md](docs/echonet-findings.md) に、ビルドや調査に使うパッケージは仕様書に記録しています。

## 起動

実機から取得して運用する場合:

```sh
go build -o solar-monitor .
./solar-monitor
```

初期設定ではECHONET Lite実機から取得します。初回起動時にマルチキャスト探索でモニターを探し、見つかったIPアドレスを `data/config.json` に保存します。次回以降は保存したアドレスを使います。

タブレットからは同じLAN内で以下を開きます。

```text
http://<このPCやRaspberry PiのIPアドレス>:8080/
```

HTTPポートや保存先を変える場合だけ、起動オプションを指定します。

```sh
./solar-monitor -addr :8081 -data data/solar.db
```

Windows版は次のように起動します。

```powershell
.\solar-monitor.exe
```

探索結果が意図した機器でない場合は、一度だけ対象IPを指定すると `data/config.json` が更新されます。

```powershell
.\solar-monitor.exe -echonet-addr 192.168.100.202
```

## GitHubで管理する

初回だけ、作業ディレクトリで以下を実行します。

```sh
git init
git add .
git commit -m "Initial solar monitor"
```

GitHub CLI を使える場合:

```sh
gh repo create solar-monitor --private --source . --remote origin --push
```

GitHubの画面で空リポジトリを作る場合:

```sh
git remote add origin git@github.com:<ユーザー名>/solar-monitor.git
git branch -M main
git push -u origin main
```

`data/`、ビルドしたバイナリ、添付写真の展開先 `photos/` は `.gitignore` で除外しています。

## データ取得方式

デモ用の疑似データで動かす場合は `-source mock` を指定します。

```sh
./solar-monitor -source mock
```

外部のHTTP JSONエンドポイントから取る場合は、以下のJSONを返すURLを指定します。

```json
{
  "time": "2026-10-01T12:00:00+09:00",
  "pv_watts": 3200,
  "load_watts": 1100,
  "grid_watts": -2100,
  "today_kwh": 18.4
}
```

```sh
./solar-monitor -source http-json -source-url http://192.168.1.50/solar.json
```

`grid_watts` は、正の値を買電、負の値を売電として扱います。

## ECHONET Liteをスキャンする

蓄電池やパワコンがECHONET Lite経由で見えるか調べるには、まずマルチキャスト探索を実行します。

```sh
./solar-monitor scan
```

出力されるEOJの例です。

```text
027901  太陽光発電
027d01  蓄電池
028801  低圧スマート電力量メータ
```

特定ノードの詳細を見る場合は `./solar-monitor scan -echonet-addr 192.168.100.202` を実行します。

応答の有無を生パケットで確認する場合は以下を使います。

```sh
./solar-monitor scan -raw
./solar-monitor scan -echonet-addr 192.168.100.202 -raw
```
各EOJの `get:` に表示されるEPCが、読み取り可能なプロパティです。蓄電池は `027dxx`、太陽光発電は `0279xx` が手がかりになります。

## SHARP SUNVISTA JH-RWL8で試す

添付写真の機器は SHARP SUNVISTA のモニター `JH-RWL8` で、IPアドレスは `192.168.100.202` と読めます。
同じLANにいる端末から、自動探索を試す場合は以下です。

```sh
./solar-monitor
```

起動後の初回収集は、取得できるプロパティをできるだけ拾うためにフルスキャンします。通常運用では2回目以降は高速なバッチ取得になります。毎回フルスキャンしたい場合は以下です。

```sh
./solar-monitor -full-scan
```

ECHONET Lite はUDP `3610` を使います。端末側のファイアウォールでUDP通信が止まっている場合は許可してください。

SHARP JH-RWL8環境では、標準の対応属性一覧 `9f` に依存せず、実測済みのプロパティを直接読みます。代表値には以下を使います。

```text
発電:             027901:e0
積算発電電力量:   027901:e1 (0.001 kWh)
蓄電池充放電電力: 027d01:d3 (+が充電、-が放電)
瞬時売電電力:     05ff01:f2 (0.1 W)
瞬時買電電力:     05ff01:f3 (0.1 W)
積算売電電力量:   05ff01:f4 (0.001 kWh)
積算買電電力量:   05ff01:f5 (0.001 kWh)
```

消費電力は `発電 + 買電 - 蓄電池充電電力 - 売電` で算出します。グラフには直近720件（標準の60秒間隔で約12時間）の発電・消費・買電/売電を表示します。上部に色付き凡例があり、縦軸はkW、横軸は時刻です。買電/売電の線は絶対値で表示します。

機種や設定でEOJ/EPCが違う場合は、以下のように差し替えられます。

```sh
./solar-monitor \
  -echonet-addr 192.168.100.202 \
  -echonet-fields "pv=027901:e0,today=027901:e1:0.001"
```

写真から読めた情報:

```text
メーカー: SHARP / SUNVISTA
モニター型名: JH-RWL8
IPアドレス: 192.168.100.202
ネットマスク: 255.255.255.0
ゲートウェイ: 192.168.100.1
PCS機種: T068
CNV機種: B002
蓄電池機種: T02
```

## 開発

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
```

## バージョンと表示

Windows配布版は [GitHub Releases](https://github.com/takemontakashi2/solar-monitor/releases) から取得できます。リリース手順と0.2.0の変更点は [リリースノート](docs/releases/v0.2.0.md) を参照してください。`vX.Y.Z` タグをpushするとGitHub ActionsがテストとWindows版のビルドを行い、exe・zip・SHA-256を添付したドラフトリリースを作成します。確認後にGitHub上で公開します。

`./solar-monitor -v` または `./solar-monitor --version` でバージョンとビルド元のコミットを確認できます。開発中の変更を含むビルドは `dirty` と表示します。

グラフは標準で直近5分の移動平均を表示し、「実測値」で元の値に切り替えられます。買電/売電は絶対値を平均します。保存データや上部の現在値は平滑化しません。欠測は0にせず線を途切れさせ、5分を超える取得間隔も線で結びません。

充電率などの選択カードは、項目ごとの最後の取得値と取得時刻を表示します。最新回で未取得なら「前回値」、取得から5分を超えた値には「更新待ち」と表示します。まだ取得できていない項目もカードを残します。
