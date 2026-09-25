# agent-monitor

`agent-monitor` は、Codex や Claude Code などの AI コーディングエージェントの進捗、異常、質問、ツール利用、トークン、概算コストを確認するためのダッシュボードです。

ローカルでは軽量な Go サーバーと JSONL で動作し、AWS では CloudFront、S3、API Gateway、Lambda、DynamoDB、Cognito を利用して公開できます。

## 公開範囲

このREADMEには本番URL、Cognitoの識別子、API Key、AWSアカウント固有の値を記載しません。
デプロイ先や認証情報は、環境変数、GitHub Secrets、AWS側のStack Outputで管理します。

## AWS構成図

<img width="1575" height="986" alt="aws-architecture svg" src="https://github.com/user-attachments/assets/7e97548e-41d6-4af6-b90a-982386418470" />



実際のURL、User Pool、API Key、テーブル名などの環境固有値は公開READMEには載せず、AWS CDK context、GitHub Secrets、またはローカルの `.agent-monitor.env` で管理します。

## ローカル起動

1. 監視を有効にします。

```bash
export AGENT_MONITOR_ENABLED=true
```

2. ローカルで Codex 用の監視設定を有効にします。

```bash
export AGENT_MONITOR_AGENT=codex
touch .agent-monitor
```

3. CLI からイベントを記録します。

```bash
go run ./cmd/agent-monitor --type task --status running --title "MVPを実装" --agent codex --tokens 1200 --tool-calls 4
go run ./cmd/agent-monitor --type question --status blocked --title "AWSアカウントIDの確認が必要"
```

4. ダッシュボードを起動します。

```bash
make run
```

ブラウザで `http://localhost:8080` を開きます。

## 監視の有効化ルール

監視はデフォルトで無効です。有効化の優先順位は次の通りです。

1. `AGENT_MONITOR_ENABLED=true` または `false`
2. 環境変数がない場合は `.agent-monitor` マーカーファイル
3. どちらもない場合は無効

監視が無効な場合、CLI は終了コード `0` で終了し、ファイル書き込みや HTTP 呼び出しを行いません。

## アーキテクチャ

ローカル構成:

```text
Codex / Claude Code -> agent-monitor CLI -> JSONL -> Go Server -> Web Dashboard
```

AWS構成:

```text
Route53 -> CloudFront -> S3 Dashboard
Route53 -> CloudFront -> API Gateway -> Lambda -> DynamoDB
```

Go 側はクリーンアーキテクチャを意識して、責務を小さく分けています。

- `internal/model`: イベントとスナップショットのエンティティ
- `internal/store`: JSONL 永続化
- `internal/api`: HTTP ハンドラ
- `internal/config`: 監視有効化と実行時設定

## AWS認証

ダッシュボード参照とエージェントからのイベント登録は、用途ごとに認証方式を分けます。

- ダッシュボード参照: Cognitoでログインしたユーザーのみ許可
- イベント登録: エージェント用のAPI Keyを持つクライアントのみ許可
- 認証情報: READMEへ記載せず、AWSとローカル環境変数で管理

静的HTMLはCloudFrontから配信されますが、画面表示時にCognito認証を要求します。監視イベントの登録APIは、ブラウザログインではなくエージェント用API Keyで保護します。

## DynamoDB分離

Codex と Claude のデータは別テーブルに保存します。実際のテーブル名はCDKで管理し、公開READMEには記載しません。

Lambda の有効化フラグは次の2つです。

- `CODEX_MONITOR_ENABLED=true|false`
- `CLAUDE_MONITOR_ENABLED=true|false`

API 呼び出し時は `agent=codex` または `agent=claude` を指定できます。

```text
GET /api/snapshot?agent=codex
GET /api/snapshot?agent=claude
```

## API認証と呼び出し方法

参照APIはCognito Authorizerで保護されています。ダッシュボードのJavaScriptが、画面内ログインで取得したIDトークンを自動で付与します。

ダッシュボードから参照する場合は、`web/app.ts` のカスタムログイン画面からCognitoへ認証し、スナップショット取得APIへIDトークンを自動付与します。

Codexから自動送信する場合は、CognitoログインではなくAPI Gateway API Keyを使います。`AGENTS.md` のルールに従って `cmd/agent-monitor` が実行され、`AGENT_MONITOR_API_URL` と `AGENT_MONITOR_API_KEY` が設定されていればAWS APIへ送信し、未設定ならローカルJSONLへ保存します。

```bash
export AGENT_MONITOR_ENABLED=true
export AGENT_MONITOR_AGENT=codex
export AGENT_MONITOR_API_URL="<監視APIのURL>"
export AGENT_MONITOR_API_KEY="<エージェント登録用API Key>"

go run ./cmd/agent-monitor --type task --status running --title "Codex作業開始" --agent codex
```

ローカルでは同じ内容を `.agent-monitor.env` に置くと、`cmd/agent-monitor` が起動時に自動で読み込みます。

API KeyはAWS環境作成後、AWS CLIやコンソールから取得し、GitHub Secretsまたはローカルの `.agent-monitor.env` に保存します。値をREADMEやコミットに含めないでください。

```bash
API_KEY_ID=$(aws cloudformation describe-stacks \
  --stack-name <stack-name> \
  --region <region> \
  --query "Stacks[0].Outputs[?OutputKey=='IngestApiKeyId'].OutputValue | [0]" \
  --output text)

aws apigateway get-api-key \
  --api-key "$API_KEY_ID" \
  --include-value \
  --region <region> \
  --query value \
  --output text
```

手元から確認する場合も、URLやトークンは環境変数に入れて使います。

```bash
curl -H "Authorization: Bearer ${AGENT_MONITOR_ID_TOKEN}" \
  "${AGENT_MONITOR_API_URL}/api/snapshot?agent=codex"
```

```bash
curl -X POST "${AGENT_MONITOR_API_URL}/api/events?agent=codex" \
  -H "x-api-key: ${AGENT_MONITOR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"type":"task","status":"running","title":"Codexの進捗確認","agent":"codex"}'
```

## テスト

```bash
make test
```

`make test` は `cmd` と `internal` を対象にします。`infra/node_modules` 配下のファイルを Go パッケージとして扱わないためです。

## AWS CDK

依存関係をインストールして、CDK を合成します。

```bash
cd infra
npm install
export AGENT_MONITOR_ZONE_NAME="<Route53のHosted Zone名>"
export AGENT_MONITOR_DASHBOARD_DOMAIN="<ダッシュボード公開ドメイン>"
export AGENT_MONITOR_COGNITO_USER_POOL_ID="<既存Cognito User Pool ID>"
npm run synth
```

環境固有値は、CDK context、環境変数、GitHub Variablesのいずれかで渡します。公開リポジトリには実値を入れません。

AWS に初回デプロイします。

```bash
cd infra
npm run deploy
```

初回 CDK デプロイ後は、S3 と Lambda だけを更新します。

```bash
cd infra
npm run deploy:app
```

`deploy:app` は次を実行します。

1. Lambda の ZIP を作成
2. Lambda コードを更新
3. `web/` を S3 に同期
4. CloudFront のキャッシュを削除

S3ダッシュボードバケットにはObjectCreatedイベント通知も設定します。S3上のファイルが更新されると、`cloudfront_cache_update` Lambda が対象パスのCloudFront invalidationを作成します。

## LocalStack

LocalStack にも同じ CDK スタックをデプロイできます。

```bash
make localstack-up
cd infra
npm install
npm install -g aws-cdk-local
npm run deploy:local
```

CloudFront の対応範囲は LocalStack のエディションとバージョンに依存します。

## GitHub Actions

`.github/workflows/deploy.yml` は Pull Request でテストと CDK 型チェックを実行します。

`main` にマージされた場合、テスト成功後に AWS へデプロイします。

- `AgentMonitorStack` が未作成の場合: `npm run deploy`
- `AgentMonitorStack` が存在する場合: `npm run deploy:app`

必要な GitHub 設定:

- Secret: `AWS_ROLE_TO_ASSUME`
- Variable: `AWS_REGION`
- Variable: `AGENT_MONITOR_ZONE_NAME`
- Variable: `AGENT_MONITOR_DASHBOARD_DOMAIN`
- Variable: `AGENT_MONITOR_COGNITO_USER_POOL_ID`

既存リソース名を固定したい場合だけ、次のVariablesも設定します。未設定の場合はCDKが名前を生成します。

- `AGENT_MONITOR_CODEX_EVENTS_TABLE_NAME`
- `AGENT_MONITOR_CLAUDE_EVENTS_TABLE_NAME`
- `AGENT_MONITOR_INGEST_FUNCTION_NAME`
- `AGENT_MONITOR_INGEST_API_KEY_NAME`
- `AGENT_MONITOR_INGEST_USAGE_PLAN_NAME`

`AWS_ROLE_TO_ASSUME` が未設定の場合、テスト後にデプロイジョブはスキップされます。
