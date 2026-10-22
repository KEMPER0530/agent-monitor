# Security remediation (2026-10-22)

The local HTTP API now binds to loopback by default, rejects cross-origin browser requests, limits event payloads to 1 MiB and rejects trailing JSON. The HTTP server has read/write/header/idle timeouts. Remote exposure still requires an authenticated TLS reverse proxy.

Go is updated from 1.20 to 1.26.10. CDK dependencies are locked and CI uses npm ci. The Node audit checks development dependencies at high severity.

Validation: go test ./cmd/... ./internal/...; govulncheck ./cmd/... ./internal/... (zero findings); npm ci and npm run build in infra; npm audit (zero findings). The scoped Go paths avoid CDK's bundled Go template files under node_modules.
