# lumen-mcp-server Architecture Scaffold (Phase 0)

## Layering

- `cmd/`: startup, config loading, graceful shutdown
- `internal/config`: config schema/default/validation
- `internal/domain`: core domain (`tool`, `policy`, `audit`)
- `internal/application`: `authorize`, `invoke`, `session` + `ports`
- `internal/infrastructure`: `jwkscache`, `gatewayclient`, `auditstore`
- `internal/interfaces/http`: admin APIs + middleware chain
- `internal/interfaces/mcp`: stdio MCP transport scaffold
- `internal/platform`: logging + observability

Dependency direction: `interfaces -> application -> domain` and `infrastructure -> application -> domain`.

## Authz Flow

1. Parse bearer token
2. Verify token via `TokenVerifier`
3. Resolve required scope from `auth.tool_scope_map`
4. Authorize (`allow/deny`)
5. Invoke gateway adapter
6. Write audit event

Default stance: deny unless tool has explicit scope mapping.

## Middleware Stack

Order:

1. RequestID
2. Recovery
3. AccessLog
4. Metrics

HTTP admin endpoints:

- `/healthz`
- `/admin/tools`
- `/admin/audit`
- `/admin/tools/invoke`
- `/debug/vars`

## Observability

- Structured logs with `trace_id`
- expvar metrics:
  - `mcp_http_requests_total`
  - `mcp_http_errors_total`
  - `mcp_tool_invokes_total`
  - `mcp_tool_denials_total`
  - `mcp_latency_ms_total`

## TODO Hotspots

- `jwkscache/verifier`: real JWT+JWKS validation and key rotation
- `gatewayclient`: tool-to-gateway endpoint mapping + request schema checks
- `interfaces/mcp`: full MCP lifecycle + tool registry protocol
- `auditstore`: sqlite backend and retention policy
- `session`: active session visibility and lifecycle tracking
