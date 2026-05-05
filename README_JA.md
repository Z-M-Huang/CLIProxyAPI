# CLI Proxy API

[English](README.md) | [中文](README_CN.md) | 日本語

> **フォークについて。** 本リポジトリは [Z-M-Huang](https://github.com/Z-M-Huang) による [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) のフォークで、追加機能（現在は **Prompt Rules** — 送信リクエストへのコンテンツレベルの注入／除去。再構築されたロギングサポートは v0.2.0 で予定）を含み、Docker イメージは `zhironghuang/cli-proxy-api` で再公開しています。上流の改善は定期的にマージしています。元のプロジェクトは上記リンクをご参照ください。

CLI向けのOpenAI/Gemini/Claude/Codex互換APIインターフェースを提供するプロキシサーバーです。

OAuth経由でOpenAI Codex（GPTモデル）およびClaude Codeもサポートしています。

ローカルまたはマルチアカウントのCLIアクセスを、OpenAI（Responses含む）/Gemini/Claude互換のクライアントやSDKで利用できます。

## スポンサーシップ

本フォークはスポンサーシップの募集や受け入れを行っていません。元のプロジェクト（[router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)）は包括的なスポンサーリストとパートナー特典を維持しています — 現在のスポンサーや関連するプロモーションコードについては上流の README をご参照ください。

## 概要

- CLIモデル向けのOpenAI/Gemini/Claude互換APIエンドポイント
- OAuthログインによるOpenAI Codexサポート（GPTモデル）
- OAuthログインによるClaude Codeサポート
- プロバイダールーティングによるAmp CLIおよびIDE拡張機能のサポート
- ストリーミングおよび非ストリーミングレスポンス
- 関数呼び出し/ツールのサポート
- マルチモーダル入力サポート（テキストと画像）
- ラウンドロビン負荷分散による複数アカウント対応（Gemini、OpenAI、Claude）
- シンプルなCLI認証フロー（Gemini、OpenAI、Claude）
- Generative Language APIキーのサポート
- AI Studioビルドのマルチアカウント負荷分散
- Gemini CLIのマルチアカウント負荷分散
- Claude Codeのマルチアカウント負荷分散
- OpenAI Codexのマルチアカウント負荷分散
- 設定によるOpenAI互換アップストリームプロバイダー（例：OpenRouter）
- プロンプトルール：上流に転送する前にシステムプロンプトや最後のユーザーメッセージへ常駐の指示を注入、または不要な定型文を正規表現で除去。モデルとソース形式でスコープし、リクエスト間で冪等
- プロキシ埋め込み用の再利用可能なGo SDK（`docs/sdk-usage.md`を参照）

## はじめに

CLIProxyAPIガイド：[https://help.router-for.me/](https://help.router-for.me/)

## 管理API

[MANAGEMENT_API.md](https://help.router-for.me/management/api)を参照

## 使用量統計

このフォークは、上流が v6.10.0 で削除したインメモリの使用量統計を復元し、各レコードに `request_id` フィールドを追加してディスク上のリクエスト単位ログとの相関を可能にしました。統計は管理 UI の「使用量」タブと、以下の管理 API から確認できます：

- `GET /v0/management/usage` — 現在のスナップショット
- `GET /v0/management/usage/export` — バックアップ/移行
- `POST /v0/management/usage/import` — エクスポートからの復元
- `GET /v0/management/request-log-by-id/:id` — リクエスト単位のログファイル取得（「リクエストイベント詳細」モーダルが使用）

インメモリ以外の永続化（SQLite やより長期の保持など）が必要な場合は、管理 API を利用する次のスタンドアロンプロジェクトが選択肢になります。

### [CPA Usage Keeper](https://github.com/Willxup/cpa-usage-keeper)

CLIProxyAPI 向けの独立した使用量永続化・可視化サービス。管理 API のスナップショットを定期的に SQLite へ取り込み、集計 API とダッシュボードを提供します。

### [CLIProxyAPI Usage Dashboard](https://github.com/zhanglunet/cliproxyapi-usage-dashboard)

CLIProxyAPI 向けのローカル優先の使用量・クォータダッシュボード。Redis 互換の使用量キューからリクエスト使用量を SQLite に保存し、アカウント別・モデル別の日次および直近時間枠の使用量を可視化し、Codex 5h/7d クォータ残量をローカル Web UI で表示します。

## Amp CLIサポート

CLIProxyAPIは[Amp CLI](https://ampcode.com)およびAmp IDE拡張機能の統合サポートを含んでおり、Google/ChatGPT/ClaudeのOAuthサブスクリプションをAmpのコーディングツールで使用できます：

- Ampの APIパターン用のプロバイダールートエイリアス（`/api/provider/{provider}/v1...`）
- OAuth認証およびアカウント機能用の管理プロキシ
- 自動ルーティングによるスマートモデルフォールバック
- 利用できないモデルを代替モデルにルーティングする**モデルマッピング**（例：`claude-opus-4.5` → `claude-sonnet-4`）
- localhostのみの管理エンドポイントによるセキュリティファーストの設計

特定のバックエンド系統のリクエスト/レスポンス形状が必要な場合は、統合された `/v1/...` エンドポイントよりも provider-specific のパスを優先してください。

- messages 系のバックエンドには `/api/provider/{provider}/v1/messages`
- モデル単位の generate 系エンドポイントには `/api/provider/{provider}/v1beta/models/...`
- chat-completions 系のバックエンドには `/api/provider/{provider}/v1/chat/completions`

これらのパスはプロトコル面の選択には役立ちますが、同じクライアント向けモデル名が複数バックエンドで再利用されている場合、それだけで推論実行系が一意に固定されるわけではありません。実際の推論ルーティングは、引き続きリクエスト内の model/alias 解決に従います。厳密にバックエンドを固定したい場合は、一意な alias や prefix を使うか、クライアント向けモデル名の重複自体を避けてください。

**→ [Amp CLI統合ガイドの完全版](https://help.router-for.me/agent-client/amp-cli.html)**

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
