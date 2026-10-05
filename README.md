# Solar Monitor

Pure Go SQLiteで記録する、家庭用太陽光発電の記録・表示サーバーです。
単一バイナリとしてビルドでき、タブレットからブラウザで閲覧できます。

要望と仕様の整理は [docs/SPEC.md](docs/SPEC.md) にまとめています。
ビルドや調査に使うパッケージも同じ仕様書に記録しています。

## 起動

実機から取得して運用する場合:

```sh
go build -o solar-monitor .
./solar-monitor \
  -addr :8080 \
  -data data/solar.db \
  -interval 60s \
  -source echonet \
  -echonet-addr 192.168.100.202
```

タブレットからは同じLAN内で以下を開きます。

```text
http://<このPCやRaspberry PiのIPアドレス>:8080/
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

`-source` を指定しない場合はデモ用の疑似データです。

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

`grid_watts` は、正の値を買電、負の値を売電として扱う想定です。

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
./solar-monitor -source echonet
```

起動後の初回収集は、取得できるプロパティをできるだけ拾うためにフルスキャンします。通常運用では2回目以降は高速なバッチ取得になります。毎回フルスキャンしたい場合は以下です。

```sh
./solar-monitor -source echonet -full-scan
```

ECHONET Lite はUDP `3610` を使います。端末側のファイアウォールでUDP通信が止まっている場合は許可してください。

初期設定では以下のプロパティを読みに行きます。

```text
pv=027901:e0
today=027901:e1:0.001
```

機種や設定でEOJ/EPCが違う場合は、以下のように差し替えられます。

```sh
./solar-monitor \
  -source echonet \
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

## 次に差し替える場所

実機のパワコン、HEMS、スマートメーターに合わせて `Source` インターフェースを実装します。

```go
type Source interface {
	Read(ctx context.Context) (Sample, error)
}
```

機器が決まれば、ECHONET Lite、Modbus、メーカーAPI、ローカルHTTPなどの取得処理をここへ追加します。
