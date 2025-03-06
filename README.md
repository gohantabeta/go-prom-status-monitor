# go-prom-status-monitor

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Prometheus, Go, React, and Nginx を使用したシンプルなサービスステータスモニター。
## 制作目的

このプロジェクトは、以下のような課題を解決するために開発されました。

- 過去のハッカソン成果物や個人的にホスティングしているツールなど、必ずしも常に稼働しているとは限らないサービスが存在する。
- これらのサービスを内外部に公開する際、利用者が稼働していないサービスにアクセスを試み、時間を浪費してしまう可能性がある。

このサービスステータスモニターは、各サービスの稼働状況を可視化し、利用者が事前に状況を把握できるようにすることで、上記のような時間の浪費を防ぐことを目的としています。

## 概要
**⚠️ 注意: このプロジェクトは現在開発中です。以下の説明は、最終的な目標とする機能について記述したものであり、現時点ではまだ完全に動作しません**

このプロジェクトは、以下の技術スタックを使用して構築されたサービスステータスダッシュボードを提供します。

- **バックエンド:** Go
- **フロントエンド:** React
- **監視:** Prometheus
- **リバースプロキシ:** Nginx
- **コンテナ:** Docker, Docker Compose

ローカル IP 上のサーバーで稼働するサービスの稼働状況を Prometheus で監視し、Go で Prometheus API を用いて稼働状況を取得、React フロントエンドで表示します。稼働中のサービスについては、React で表示したボタンをクリックすることで Nginx によるリバースプロキシ経由で各サービスへアクセスできます。

## 構成
Use code with caution.
Markdown
go-prom-status-monitor/
├── docker-compose.yml
├── nginx/
│ └── nginx.conf
├── prometheus/
│ ├── prometheus.yml
│ └── targets.json (必要に応じて)
├── go-backend/
│ ├── main.go
│ ├── go.mod
│ └── go.sum
├── react-frontend/
│ ├── package.json
│ ├── public/
│ │ └── index.html
│ └── src/
│ ├── App.js
│ ├── components/
│ │ └── ServiceStatus.js
│ └── index.js
└── .env (必要に応じて)



## 使い方
**⚠️ 注意: このプロジェクトは現在開発中です。以下の説明は、最終的な目標とする手順について記述したものであり、現時点ではまだ完全に動作しません**
1.  **前提条件:**
    - Docker および Docker Compose がインストールされていること。

2.  **クローン:**
    ```bash
    git clone <リポジトリのURL>
    cd go-prom-status-monitor
    ```

3.  **設定:**
    - `prometheus/prometheus.yml`: 監視対象のサービスを設定します。
    - `nginx/nginx.conf`: 必要に応じてリバースプロキシの設定を調整します。
    - `.env`: 環境変数を設定します 。

4.  **起動:**
    ```bash
    docker-compose up --build
    ```

5.  **アクセス:**
   -  ブラウザで `http://localhost` (または Docker 環境に応じた URL) にアクセスします。

## CI/CD (GitHub Actions)

`main` ブランチ、`staging`ブランチへのプッシュ時に、自動的にビルドとデプロイが行われます。
詳細は `.github/workflows`ディレクトリ内のYAMLファイルを参照してください。

## ライセンス

このプロジェクトは [MIT License](LICENSE) のもとで公開されています。

## TODO (今後の拡張予定)

- より詳細な設定方法の説明
- テストの追加
- 認証機能の追加
- UI の改善
- アラート機能
