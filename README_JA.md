# CLI Proxy API

[English](README.md) | [中文](README_CN.md) | 日本語

> **フォークについて。** 本リポジトリは [Z-M-Huang](https://github.com/Z-M-Huang) による [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) のフォークで、Prompt Rules、SQLite による使用量／リクエスト履歴の永続化、設定可能なプロバイダーヘッダーを追加し、Docker イメージを `zhironghuang/cli-proxy-api` として再公開しています。上流の改善は定期的にマージしています。元のプロジェクトは上記リンクをご参照ください。

CLI向けのOpenAI/Gemini/Claude/Codex/Grok互換APIインターフェースを提供するプロキシサーバーです。

OAuth経由でOpenAI Codex（GPTモデル）およびClaude Codeもサポートしています。

ローカルまたはマルチアカウントのアクセスを、OpenAI（Responses含む）、Gemini、Claude、Grok互換のクライアントやSDKで利用できます。

## スポンサーシップ

本フォークはスポンサーシップの募集や受け入れを行っていません。元のプロジェクト（[router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)）は包括的なスポンサーリストとパートナー特典を維持しています — 現在のスポンサーや関連するプロモーションコードについては上流の README をご参照ください。

## 概要

- CLIモデル向けのOpenAI/Gemini/Claude/Grok互換APIエンドポイント
- OAuthログインによるOpenAI Codexサポート（GPTモデル）
- OAuthログインによるClaude Codeサポート
- OAuthログインによるGrok Buildサポート
- プロバイダーが対応する場合のストリーミング、非ストリーミング、WebSocketレスポンス
- 関数呼び出し/ツールのサポート
- プロバイダーが対応する画像・動画を含むマルチモーダル入力
- ラウンドロビン負荷分散による複数アカウント対応
- ルーティング、認証、実行、リクエスト／レスポンスのインターセプトに対応する動的ネイティブプラグイン
- Generative Language APIキーのサポート
- AI Studioビルドのマルチアカウント負荷分散
- Claude Codeのマルチアカウント負荷分散
- OpenAI Codexのマルチアカウント負荷分散
- Grok Buildのマルチアカウント負荷分散
- 設定によるOpenAI互換アップストリームプロバイダー（例：OpenRouter）
- プロンプトルール：上流に転送する前にシステムプロンプトや最後のユーザーメッセージへ常駐の指示を注入、または不要な定型文を正規表現で除去。モデルとソース形式でスコープし、リクエスト間で冪等
- プロキシ埋め込み用の再利用可能なGo SDK（`docs/sdk-usage.md`を参照）

## はじめに

CLIProxyAPIガイド：[https://help.router-for.me/](https://help.router-for.me/)

## 管理API

[MANAGEMENT_API.md](https://help.router-for.me/management/api)を参照

## 使用量統計

このフォークは、上流が v6.10.0 で削除した使用量統計を復元し、使用量イベントとリクエスト単位の履歴を SQLite に永続化します。各レコードの `request_id` を使って管理 UI からリクエスト詳細を関連付けられます。

- `GET /v0/management/usage` と `GET /v0/management/usage/overview` — 永続化された15分ロールアップから集計スナップショットを取得
- `GET /v0/management/usage/events` — リクエスト単位イベントのフィルタ付きページ取得
- `GET /v0/management/usage/export` — 保持期間内のリクエスト単位バックアップ/移行スナップショット
- `POST /v0/management/usage/import` — エクスポートからの復元
- `GET /v0/management/request-log-by-id/:id` — 永続化されたリクエスト履歴の取得

`usage-statistics-enabled` はイベント記録、`request-log` は完全なリクエスト／レスポンス履歴、`usage-database-path` はデータベースパス（既定値 `./data/usage.sqlite`）、`usage-event-retention-days` はリクエスト単位イベントの保持日数（`0` は無期限）を制御します。イベントを削除した後もロールアップと重複排除キーは保持されるため、ダッシュボードの合計値は維持されます。イベント一覧とエクスポートには保持期間内のリクエストのみが含まれます。Docker Compose は `./data` をマウントするため、コンテナ再構築後もデータが残ります。`logging-to-file` は引き続きローテーションされるアプリケーションログ専用です。

### [CPA Usage Keeper](https://github.com/Willxup/cpa-usage-keeper)

CLIProxyAPI 向けの独立した使用量永続化・可視化サービス。管理 API のスナップショットを定期的に SQLite へ取り込み、集計 API とダッシュボードを提供します。

### [CLIProxyAPI Usage Dashboard](https://github.com/zhanglunet/cliproxyapi-usage-dashboard)

CLIProxyAPI 向けのローカル優先の使用量・クォータダッシュボード。Redis 互換の使用量キューからリクエスト使用量を SQLite に保存し、アカウント別・モデル別の日次および直近時間枠の使用量を可視化し、Codex 5h/7d クォータ残量をローカル Web UI で表示します。

### [CPA-Manager](https://github.com/seakee/CPA-Manager)

リクエスト単位の監視とコスト推定を備えたCLIProxyAPI向けのフル管理センターです。CPA-Managerは、収集したリクエストをアカウント、モデル、チャネル、レイテンシ、ステータス、Token使用量ごとに追跡し、編集可能なモデル価格とLiteLLM価格のワンクリック同期でコストを推定します。SQLiteでイベントを永続化し、Codexアカウントプール向けに一括検査、クォータ判定、異常アカウント検出、クリーンアップ提案、ワンクリック実行を提供し、日常的なマルチアカウント運用に適しています。

## SDKドキュメント

- 使い方：[docs/sdk-usage.md](docs/sdk-usage.md)
- 上級（エグゼキューターとトランスレーター）：[docs/sdk-advanced.md](docs/sdk-advanced.md)
- アクセス：[docs/sdk-access.md](docs/sdk-access.md)
- ウォッチャー：[docs/sdk-watcher.md](docs/sdk-watcher.md)
- カスタムプロバイダーの例：`examples/custom-provider`

## コントリビューション

[CONTRIBUTING.md](./CONTRIBUTING.md) を参照してください。本フォーク固有のブランチモデル、上流との同期ワークフロー、カスタマイズ範囲、リリースプロセスが記載されています。基本的な流れは標準的な fork → ブランチ作成 → PR ですが、PR の送信先は上流ではなく本リポジトリの `dev` ブランチです。

## ライセンス

本プロジェクトはMITライセンスの下でライセンスされています - 詳細は[LICENSE](LICENSE)ファイルを参照してください。
