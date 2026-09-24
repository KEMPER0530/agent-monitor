.PHONY: run test build cli cdk-synth localstack-up localstack-deploy

run:
	go run ./cmd/server

test:
	go test ./...

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

