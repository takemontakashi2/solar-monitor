# FreeBSD用pkg（0.9.14）

現在作成済みのパッケージはFreeBSD 15 / amd64用です。他のメジャーバージョンやCPUでは、その環境でビルドしてください。通常のLinux用バイナリとは別です。

## インストールと起動

以下はrootで実行します。

```sh
pkg add ./solar-monitor-0.9.14-freebsd-15-amd64.pkg
sysrc solar_monitor_enable=YES
service solar_monitor start
```

専用ユーザー・グループ `solar-monitor` を作成し、ログイン不可のユーザーで動かします。既存の同名アカウントは再利用します。インストールだけでは起動しません。

| 用途 | 保存先 |
|---|---|
| 本体 | `/usr/local/bin/solar-monitor` |
| 設定 | `/usr/local/etc/solar-monitor/config.json` |
| DB | `/var/db/solar-monitor/solar.db` |
| サービス定義 | `/usr/local/etc/rc.d/solar_monitor` |
| 説明書 | `/usr/local/share/doc/solar-monitor/` |

設定ディレクトリとDBディレクトリは専用ユーザーが所有します。接続先を自動保存するため、設定にも書き込み権限が必要です。本体とサービス定義はroot所有です。

ブラウザで `http://サーバーのIPアドレス:8080/` を開きます。ログはsyslogのdaemon.infoへ、タグ `solar-monitor` で出力します。標準的なsyslog設定では `/var/log/messages` で確認できます。

```sh
service solar_monitor status
service solar_monitor stop
```

保存間隔はサービスの既定で1時間です。停止時には未保存データを保存します。異常終了時はdaemonが10秒後に再起動します。電源断・強制終了ではメモリ上の未保存分を失います。

## 設定変更

```sh
sysrc solar_monitor_flush_interval=30m
sysrc solar_monitor_listen=:8081
sysrc solar_monitor_flags='-echonet-addr 192.168.100.202'
service solar_monitor restart
```

`solar_monitor_flags` は追加のコマンドライン引数です。必要な場合のみ指定してください。毎回保存にする場合は `solar_monitor_flush_interval=0` を指定します。

## 記録の移行・更新・削除

移行元と移行先を正常停止してから、DBと残っているSQLite補助ファイルを `/var/db/solar-monitor/` へ、設定を `/usr/local/etc/solar-monitor/config.json` へコピーします。既存の移行先データは事前にバックアップしてください。

```sh
chown -R solar-monitor:solar-monitor /var/db/solar-monitor
chown solar-monitor:solar-monitor /usr/local/etc/solar-monitor/config.json
service solar_monitor start
```

更新時も先に停止して保存完了を待ち、設定とDBをバックアップしてから新しいpkgをインストールしてください。更新後の起動は手動で行います。削除時も先に停止します。

```sh
service solar_monitor stop
pkg delete solar-monitor
```

設定・DB・専用アカウントは削除しません。バックアップや再導入のために保持します。

## ビルドと確認状況

FreeBSD上でGo、Python 3、pkgを用意し、リポジトリのルートで実行します。

```sh
python3.12 packaging/freebsd/build.py
```

Pythonのコマンド名は環境に合わせてください。`dist/` にpkgとSHA-256ファイルを生成します。ビルド時にシステムへのインストールやユーザー作成は行いません。

FreeBSD 15 / amd64でpkg生成、収録ファイル・所有者・ABI・シェル構文、およびGoのテスト（通常停止時の保存を含む）を確認しています。rootによる実際のpkg追加・削除と、専用ユーザーでの実機通信は未確認です。
