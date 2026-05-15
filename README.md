# Lumen MCP Server

[Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server that exposes Lumen Gateway's Admin API as 22 MCP tools. Supports both Streamable HTTP and stdio transports, with OAuth 2.0 token-based authorization and per-tool scope enforcement.

Designed for AI-assisted gateway management -- connect Claude Code (or any MCP client) and manage routes, services, upstreams, and plugins through natural language.

## Features

- **22 MCP Tools** covering routes, services, upstreams, plugins, global rules, bundles, history, stats, and latency/timeout tuning
- **Streamable HTTP Transport** (`/mcp` endpoint) with SSE
- **Stdio Transport** for CLI integration (with built-in OAuth login flow)
- **OAuth 2.0 Protected Resource** (RFC 9728) with JWKS-based JWT verification
- **Per-tool Scope Authorization** -- each tool maps to a required scope
- **Audit Logging** to PostgreSQL, SQLite, or stdout
- **MCP OAuth Discovery** -- `WWW-Authenticate` header with `resource_metadata` link
- **Hexagonal Architecture** matching the rest of the Lumen suite

## Quick Start

### 前置条件

MCP Server 依赖以下服务，确保它们已启动：

| 服务 | 用途 | 默认端口 |
|------|------|----------|
| lumen-gateway | 被管理的 API 网关 | 18080 |
| lumen-OAuth | OAuth 授权服务（签发 / 验证 token） | 9080 |
| PostgreSQL | 审计日志持久化 | 5432 |

最简单的方式是从项目根目录一键启动全部服务：

```bash
cd api-gateway
docker compose up -d --build
```

启动后所有服务自动健康检查，MCP Server 在 `http://localhost:9280` 就绪。

### 仅启动 MCP Server（本地开发）

如果依赖服务已在运行，可以只启动 MCP：

```bash
# Docker 方式
docker compose up -d mcp

# 或本地直接运行（需要 Go 1.25+）
go run ./cmd/lumen-mcp-server --config configs/fullstack/mcp.config.yaml
```

> 本地运行时需将 config 中的 `gateway.base_url` 和 `oauth.jwks_url` 改为 `localhost` 地址。

### 验证服务就绪

```bash
# 健康检查
curl http://localhost:9280/healthz
# {"status":"ok"}

# OAuth Protected Resource 元数据
curl http://localhost:9280/.well-known/oauth-protected-resource
# {"authorization_servers":["http://localhost:9080"],"resource":"lumen-mcp",...}

# MCP 端点（无 token 应返回 401）
curl -X POST http://localhost:9280/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"initialize","id":1}'
# 401 + WWW-Authenticate: Bearer resource_metadata="..."
```

## 在 Claude Code 中添加 MCP

### 方法一：命令行添加（推荐）

```bash
claude mcp add lumen-gateway --transport http http://localhost:9280/mcp
```

### 方法二：手动编辑配置文件

编辑项目级配置 `.claude/settings.local.json`（仅当前项目生效）：

```json
{
  "mcpServers": {
    "lumen-gateway": {
      "type": "url",
      "url": "http://localhost:9280/mcp"
    }
  }
}
```

或编辑全局配置 `~/.claude/settings.json`（所有项目生效）：

```json
{
  "mcpServers": {
    "lumen-gateway": {
      "type": "url",
      "url": "http://localhost:9280/mcp"
    }
  }
}
```

### 首次连接 OAuth 授权流程

添加 MCP 后重启 Claude Code，首次调用工具时会触发 OAuth 授权：

1. Claude Code 请求 `/mcp` → 收到 `401`
2. 自动发现 OAuth 服务器（通过 `WWW-Authenticate` → Protected Resource Metadata → Authorization Server Metadata）
3. 通过 DCR 动态注册客户端（无需手动配置 client_id）
4. 浏览器打开登录页 `http://localhost:9080/login`
5. 输入账号密码（默认管理员 `admin@example.com` / `admin`，或自行注册的账号）
6. 授权同意页确认权限
7. 授权完成，Claude Code 获得 access_token，MCP 工具可用

授权完成后，Claude Code 会缓存 token，后续连接自动刷新，无需重复登录。

### 验证 MCP 工具可用

在 Claude Code 中输入：

```
/mcp
```

应看到 `lumen-gateway` 服务和 20 个可用工具。也可以直接对话使用：

```
列出网关当前所有路由
```

### Stdio 模式（用于其他 MCP 客户端）

```bash
go run ./cmd/lumen-mcp-server --config configs/fullstack/mcp.config.yaml --stdio
```

如果未配置 `auth.static_bearer`，启动时会自动打开浏览器完成 OAuth 登录（DCR + PKCE），获取 token 后进入 stdio 模式。

## MCP Tools

| Tool | Description | Required Scope |
|------|-------------|----------------|
| `list_routes` | List gateway routes | `routes:read` |
| `get_route` | Get a specific route | `routes:read` |
| `put_route` | Create/update route | `routes:write` |
| `patch_route` | Patch route | `routes:write` |
| `delete_route` | Delete route | `routes:write` |
| `list_services` | List gateway services | `services:read` |
| `put_service` | Create/update service | `services:write` |
| `list_upstreams` | List upstreams | `upstreams:read` |
| `put_upstream` | Create/update upstream | `upstreams:write` |
| `list_plugin_configs` | List plugin configs | `plugins:read` |
| `put_plugin_config` | Create/update plugin config | `plugins:write` |
| `list_global_rules` | List global rules | `global_rules:read` |
| `put_global_rule` | Create/update global rule | `global_rules:write` |
| `list_plugins` | List plugin catalog | `plugins:read` |
| `preview_import` | Preview import bundle (dry run) | `gateway:bundle:apply` |
| `apply_import` | Apply import bundle | `gateway:bundle:apply` |
| `export_bundle` | Export current config bundle | `routes:read` |
| `history_list` | List config change history | `routes:read` |
| `history_rollback` | Rollback to previous config | `admin:dangerous` |
| `get_schema` | Get control schema | `routes:read` |
| `get_stats` | Get gateway stats | `metrics:read` |
| `analyze_latency` | Analyze upstream latency and recommend timeout values | `metrics:read` |
| `tune_upstream_timeout` | Tune upstream timeout from latency analysis (supports dry-run) | `routes:write` |

## OAuth Flow

The MCP server implements RFC 9728 (OAuth 2.0 Protected Resource Metadata):

```
Client                        MCP Server                    OAuth Server
  |                               |                              |
  |--- POST /mcp --------------->|                              |
  |<-- 401 + WWW-Authenticate --|                              |
  |    resource_metadata="..."   |                              |
  |                               |                              |
  |--- GET /.well-known/oauth-protected-resource ------------->|
  |<-- { authorization_servers: ["http://localhost:9080"] } ---|
  |                               |                              |
  |--- GET /.well-known/oauth-authorization-server ----------->|
  |<-- { registration_endpoint, authorization_endpoint, ... } -|
  |                               |                              |
  |--- POST /oauth/register (DCR) --------------------------->|
  |<-- { client_id } ---------------------------------------------|
  |                               |                              |
  |--- Browser: /oauth/authorize (PKCE) --------------------->|
  |<-- code via callback ------------------------------------------|
  |                               |                              |
  |--- POST /oauth/token (code exchange) --------------------->|
  |<-- { access_token, refresh_token } -------------------------|
  |                               |                              |
  |--- POST /mcp (Bearer token) ->|                              |
  |<-- MCP response --------------|                              |
```

## Configuration

```yaml
server:
  http_listen: ":9280"
  public_base_url: http://localhost:9280

oauth:
  issuer: http://localhost:9080
  audience: lumen-mcp
  jwks_url: http://oauth:9080/.well-known/jwks.json

gateway:
  base_url: http://gateway:18080
  admin_api_key: your-admin-key

auth:
  tool_scope_map:
    list_routes: routes:read
    put_route: routes:write
    # ... (see configs/fullstack/mcp.config.yaml for full map)

audit:
  backend: postgres         # postgres | sqlite | stdout
  postgres_url: postgres://user:pass@host:5432/db?sslmode=disable
```

## Architecture

```
cmd/lumen-mcp-server/         Entry point (--stdio or HTTP mode)
internal/
  domain/
    tool/                     Tool definition
    audit/                    Audit event
    policy/                   Authorization policy
  application/
    invoke/                   Tool invocation (verify token + authorize + call gateway)
    authorize/                Per-tool scope check
    session/                  MCP session management
    ports/                    Interface definitions
  infrastructure/
    gatewayclient/            HTTP client for Lumen Gateway Admin API
    jwkscache/                JWKS-based JWT verifier with caching
    auditstore/               PostgreSQL / SQLite / stdout audit store
    oauthlogin/               Browser-based OAuth login (DCR + PKCE)
  interfaces/
    http/                     HTTP handlers + middleware + routes
    mcp/                      MCP server (go-sdk wrapper, streamable HTTP + stdio)
  platform/                   Logging, observability
  bootstrap/                  App wiring
```

## Admin API

In addition to the MCP endpoint, the server exposes REST endpoints for the admin UI:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/tools` | List available tools |
| POST | `/admin/tools/invoke` | Invoke a tool (with bearer token) |
| GET | `/admin/audit` | List audit events |
| GET | `/.well-known/oauth-protected-resource` | Protected resource metadata |
