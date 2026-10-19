# 詳細設定・機器調査

基本的な起動と画面操作は [README](../README.md) を参照してください。ここでは外部データ連携、ECHONET Liteのスキャン、SHARP環境の属性を説明します。コマンド例はUnix系の表記です。Windowsでは `./solar-monitor` を `.\solar-monitor.exe` に読み替えてください。

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

調査対象は SHARP SUNVISTA のモニター `JH-RWL8` です。以下のIPアドレスは調査環境の例なので、利用環境に合わせて変更してください。
同じLANにいる端末から、自動探索を試す場合は以下です。

```sh
./solar-monitor
```

起動後の初回収集は、取得できるプロパティをできるだけ拾うためにフルスキャンします。通常運用では2回目以降は取得対象を絞って読み取ります。メーカー・識別・製造番号・定格値は取得と保存に成功した項目から通常収集の対象外にし、再起動後も保存値を使います。接続先ごとに取得済み状態を保持します。機器の交換や情報の更新時など、固定情報も含めて毎回フルスキャンしたい場合は以下です。

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

消費電力は `発電 + 買電 - 蓄電池充電電力 - 売電` で算出します。グラフには直近720件（標準の60秒間隔で約12時間）の発電・消費・買電/売電を表示します。上部に色付き凡例があり、縦軸はkW、横軸は時刻です。買電はプラス、売電はマイナスで表示します。

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


調査経緯と実測結果は [ECHONET Lite実測まとめ](echonet-findings.md) を参照してください。

## 収集ログの読み方

0.9.5以降は各収集の終了時に `echonet collect` を出力します。`result=ok` は全要求の値を取得、`partial` は一部欠測、`failed` は収集全体の失敗です。`elapsed` は所要時間、`requests` は要求数、`ok` / `failed` は要求の成功数・失敗数、`timeouts` はそのうちのタイムアウト数、`properties` は取得したデータ項目数です。要求数にはインスタンス一覧や属性一覧の取得も含みます。

`missing` には `EOJ:EPC(timeout)` / `(missing)` / `(error)` の形で取得できなかった項目を記録します。`missing` は値のない応答も含み、未対応属性かどうかをこのログだけでは断定できません。ログのない期間を成功と判断せず、収集ごとの結果を確認してください。

PowerShellで画面とファイルの両方にログを残す例:

```powershell
.\solar-monitor-0.9.5-windows-amd64.exe 2>&1 | Tee-Object -FilePath collect.log
```
