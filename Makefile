.PHONY: run test build cli cdk-synth localstack-up localstack-deploy

# Bare make must finish; starting the server requires an explicit make run.
.DEFAULT_GOAL := build

run:
	go run ./cmd/server

# Node依存を入れた後でもCDK内部テンプレートを拾わないよう、Goコード配下だけを対象にします。
test:
	go test ./cmd/... ./internal/...

build:
	go build ./cmd/server
	go build ./cmd/agent-monitor

cli:
	go run ./cmd/agent-monitor --help

cdk-synth:
	cd infra && npm install && npm run synth

localstack-up:
	docker compose -f infra/localstack/docker-compose.yml up -d

localstack-deploy:
	cd infra && npm install && npm run deploy:local
