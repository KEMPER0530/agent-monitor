# agent-monitor

AI coding agents such as Codex and Claude Code can run for a long time. `agent-monitor` is a local-first dashboard for checking progress, errors, questions, tool usage, tokens, and rough cost while keeping monitoring optional.

## Local MVP

1. Enable monitoring.

```bash
export AGENT_MONITOR_ENABLED=true
```

2. Record events through the CLI.

```bash
go run ./cmd/agent-monitor --type task --status running --title "Implement MVP" --agent codex --tokens 1200 --tool-calls 4
go run ./cmd/agent-monitor --type question --status blocked --title "Need AWS account id"
```

3. Start the dashboard.

```bash
make run
```

Open http://localhost:8080.

Monitoring is off by default. Priority is:

1. `AGENT_MONITOR_ENABLED=true` or `false`
2. `.agent-monitor` marker file when the env var is missing
3. off when neither exists

When monitoring is off, the CLI exits with code `0` and does not write files or call HTTP.

## Architecture

Local MVP:

```text
Codex / Claude Code -> agent-monitor CLI -> JSONL -> Go Server -> Web Dashboard
```

AWS:

```text
CloudFront -> API Gateway -> Lambda -> DynamoDB
CloudFront -> S3 dashboard
```

Production URL:

```text
https://s3-agent-monitor.kemper0530.com
```

AWS separates persisted data by agent:

- Codex: `agent-monitor-codex-events`
- Claude: `agent-monitor-claude-events`

Lambda environment flags:

- `CODEX_MONITOR_ENABLED=true|false`
- `CLAUDE_MONITOR_ENABLED=true|false`

Dashboard access is protected by the existing Cognito User Pool:

- User Pool: `nuxt-mail-demo` / `ap-northeast-1_7da4pYlPc`
- Hosted UI domain: `https://agent-monitor-kemper0530.auth.ap-northeast-1.amazoncognito.com`
- API Gateway requires a Cognito JWT on `/api/*`

The Go code keeps a small clean architecture split:

- `internal/model`: event and snapshot entities
- `internal/store`: JSONL persistence
- `internal/api`: HTTP handlers
- `internal/config`: monitoring enablement and runtime config

## Test

```bash
make test
```

`make test` targets `cmd` and `internal` so local `infra/node_modules` files are never treated as Go packages.

## AWS CDK

Install dependencies and synthesize:

```bash
cd infra
npm install
npm run synth
```

Deploy to AWS:

```bash
cd infra
npm run deploy
```

After the first CDK deployment, update only S3 and Lambda:

```bash
cd infra
npm run deploy:app
```

`deploy:app` syncs `web/` to S3, updates the ingest Lambda code, and creates a CloudFront invalidation.

Deploy to LocalStack:

```bash
make localstack-up
cd infra
npm install
npm install -g aws-cdk-local
npm run deploy:local
```

LocalStack CloudFront support depends on the LocalStack edition and version. The same CDK stack is used for local and AWS deployments.

## GitHub Actions

`.github/workflows/deploy.yml` runs Go tests and CDK type checks for pull requests. Pushes to `main` deploy to AWS after tests pass.
If `AWS_ROLE_TO_ASSUME` is not configured, the deploy job is skipped after tests pass.
When AWS credentials are configured, the deploy job checks `AgentMonitorStack`: it runs CDK only when the stack does not exist, and uses `deploy:app` after the initial environment exists.

Required configuration:

- Secret: `AWS_ROLE_TO_ASSUME`
- Variable: `AWS_REGION` such as `ap-northeast-1`
