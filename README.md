# WirePanel

Linux infrastructure management panel. Modern UX layer for Linux administration.

> No node limits. No feature crippling. Community is production-ready.

See [`docs/EDITIONS.md`](./docs/EDITIONS.md) for the Community/Pro/Team/Enterprise model.
See [`wirepanel_lockspec_v1.md`](./wirepanel_lockspec_v1.md) for locked architecture. **Do not redesign.**

## Architecture

```
Frontend (Vue3 + Tailwind + xterm.js)
    ↓  HTTPS / REST / SSE / WS
Core (Go)
    ↓  WebSocket (Agent dials in)
Agent (Go, root)
    ↓  exec / fs / svc / pty
Modules (TS, Bun/Node — TBD)
    ↓
Linux System
```

- **Browser ↔ Core only** — never browser → Agent.
- **Agent → Core dial** — easier NAT/firewall traversal.
- **1 Agent = 1 node**.

## Layout

```
core/       Go control plane (auth, REST, SSE, WS hub)
agent/      Go execution layer (root, shell/fs/svc/sysinfo/pty)
shared/     WS message types, featgate interfaces, permission catalog,
            JSONL audit impl used by community
pro/        STUB module (real pro lives in private repo for SaaS)
frontend/   Vue3 + Vite + Tailwind panel
docs/       Editions doc
```

## Editions / build model

Edition is **baked in at compile time** via Go build tag.

```bash
# Community (default)
make build
./bin/wirepanel-core              # edition=community, audit=true

# Pro / Team / Enterprise (requires private wirepanel-pro module present at ../pro)
make core-pro
./bin/wirepanel-core-pro
```

Public source only ships the Community path. Pro is a separate private Go module,
injected via the `//go:build pro` tag in `core/internal/featboot/pro_build.go`.
Community builds never touch Pro code.

## Quick start

```bash
make tidy && make build

# terminal A — core
WP_ADMIN_USER=admin WP_ADMIN_PASS=wirepanel \
WP_AGENT_TOKEN=dev-shared-token \
make run-core

# terminal B — agent (root for full Linux mgmt)
WP_CORE_URL=ws://127.0.0.1:8080/api/agent/ws \
WP_AGENT_ID=node1 \
WP_AGENT_TOKEN=dev-shared-token \
make run-agent

# terminal C — frontend dev
make run-frontend   # http://localhost:5173
```

Default login: `admin` / `wirepanel`.

## Permission model (Discord-style roles in Team+)

- **Community / Pro**: single user (owner role, wildcard `*` permission)
- **Team+**: users create roles with explicit permission picks — no preset roles
- Permission catalog at `/api/info`:
  - `agents.read`, `system.read`
  - `services.read`, `services.start`, `services.stop`, `services.restart`, `services.enable`, `services.disable`
  - `fs.read`, `fs.write`, `fs.mkdir`, `fs.delete`
  - `shell.exec`, `terminal`
  - `audit.read`, `audit.rollback`
  - `roles.manage`, `users.manage`

## Safety + rollback (all editions)

- Every destructive op requires `confirm: true` in body; `fs.delete` additionally requires `confirm_path` == absolute path typed-out
- Frontend typed-confirmation modal: user must type the exact target path/name to confirm
- Audit entry (JSONL) per dangerous op with captured pre-image
  - `fs.write` → prior content snapshot
  - `fs.delete` → file/dir moved to trash (`data/audit/trash/<audit-id>`)
  - `fs.mkdir` → existed flag (only reversible if created fresh)
  - `service.ctl` → prior active/enabled state
- `POST /api/audit/{id}/rollback` replays the inverse. `shell.exec` marked irreversible.

## Core API

| Method | Path | Permission | Purpose |
|--------|------|------------|---------|
| GET    | `/healthz` | — | liveness |
| GET    | `/api/info` | — | edition, audit enabled, permission catalog |
| POST   | `/api/auth/login` | — | bcrypt login, JWT |
| GET    | `/api/auth/me` | authed | identity + effective perms |
| GET    | `/api/agents` | `agents.read` | list agents |
| GET    | `/api/agents/{id}/system` | `system.read` | system info snapshot |
| GET    | `/api/agents/{id}/services` | `services.read` | systemd units |
| POST   | `/api/agents/{id}/services/{name}/action` | `services.*` | start/stop/restart/enable/... |
| GET/POST | `/api/agents/{id}/fs/list` | `fs.read` | dir listing |
| POST   | `/api/agents/{id}/fs/{read,write,delete,mkdir}` | `fs.*` | fs ops |
| GET (WS) | `/api/agents/{id}/terminal` | `terminal` | browser PTY |
| POST   | `/api/tasks` | `shell.exec` | dispatch shell task |
| GET (SSE) | `/api/tasks/stream?task_id=` | authed | log + result stream |
| GET    | `/api/audit` | `audit.read` | audit entries |
| GET    | `/api/audit/{id}` | `audit.read` | entry detail |
| POST   | `/api/audit/{id}/rollback` | `audit.rollback` | replay inverse |
| (Team+) | `/api/roles`, `/api/users` CRUD | `roles.manage` / `users.manage` | |
| WS     | `/api/agent/ws` | shared token | agent dial-in |

JWT supplied via `Authorization: Bearer <t>` or `?access_token=<t>` (used by SSE/WS).

## Security

- bcrypt password hash; login rate-limited (5/min/IP token bucket)
- `X-Content-Type-Options`, `X-Frame-Options: DENY`, CSP on non-API paths
- WS origin allowlist via `WP_ALLOWED_ORIGINS`
- Sensitive path denylist for fs ops (`/etc/shadow`, `/root/.ssh`, `/boot`, `/sys`, `/proc/sys`, ...) unless `WP_FS_ALLOW_SENSITIVE=true`
- TLS: `WP_TLS_CERT` + `WP_TLS_KEY`
- `WP_ENV=production` refuses default `WP_JWT_SECRET`

## Env vars (Core)

| Var | Default |
|-----|---------|
| `WP_ENV` | `development` |
| `WP_HTTP_ADDR` | `:8080` |
| `WP_WS_PATH` | `/api/agent/ws` |
| `WP_AGENT_TOKEN` | `dev-shared-token` |
| `WP_JWT_SECRET` | `dev-secret-change-me` |
| `WP_ADMIN_USER` / `WP_ADMIN_PASS` | `admin` / `wirepanel` (bootstrap only, first boot) |
| `WP_DATA_DIR` | `./data` |
| `WP_AUDIT_DIR` | `./data/audit` |
| `WP_ALLOWED_ORIGINS` | `*` |
| `WP_TLS_CERT` / `WP_TLS_KEY` | (none → HTTP) |
| `WP_FS_ALLOW_SENSITIVE` | `false` |

## Status

- [x] Phase 0: scaffold, WS hub, agent registry
- [x] Phase 1: shell exec, SSE log stream, fs ops, service mgr, system info, web terminal (PTY)
- [x] Phase 1: frontend (login, dashboard, agents, detail tabs: overview/terminal/files/services, audit)
- [x] Phase 1: audit log + rollback (community built-in)
- [x] Phase 1: safety (typed-confirmation, sensitive path deny, security headers, rate-limit)
- [x] Phase 1: edition compile-time split (community/pro/team/enterprise, featgate)
- [ ] Phase 2: module loader (Bun) + official modules
- [ ] Phase 2: marketplace (GitHub registry)
- [ ] Phase 2: per-agent token issuance
- [ ] Pro/Team/Enterprise features (private module)

## Non-goals (lockspec)

- not Kubernetes — not orchestrator — not autonomous agents
- no business logic in Agent
- no browser → Agent direct comms
