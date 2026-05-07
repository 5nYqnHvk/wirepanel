<div align="center">

# WirePanel

**Modern Linux infrastructure management panel.**

*No node limits. No feature crippling. Production-ready out of the box.*

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue)](#license)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://go.dev)
[![Vue](https://img.shields.io/badge/Vue-3-42b883?logo=vue.js)](https://vuejs.org)
[![Status](https://img.shields.io/badge/status-alpha-orange)]()

</div>

---

WirePanel is a unified control plane for managing Linux servers — packages, services, files, networking, configs — through a single web UI. Think *WinBox for Linux infrastructure*.

It is **intentionally not** a Kubernetes replacement, an orchestrator, or an automation engine. Linux stays Linux; WirePanel is the management layer on top.

## Features

- **Unlimited nodes.** One agent per machine, dial-in over a single WebSocket. NAT/firewall friendly.
- **Web terminal.** Real PTY through the browser via Core proxy (`xterm.js`).
- **File manager.** Browse, edit, create, delete files on every node — with a typed-confirmation guard for destructive ops.
- **Service manager.** systemd unit listing + start / stop / restart / enable / disable.
- **Realtime task streaming.** Dispatch shell commands; stream stdout/stderr over SSE.
- **Audit log + rollback.** Every dangerous op writes an append-only JSONL entry with a captured pre-image. `fs.write`, `fs.delete`, `fs.mkdir`, `service.ctl` are reversible by design — `POST /api/audit/{id}/rollback` replays the inverse.
- **Trash, not unlink.** `fs.delete` moves files into an audit trash directory. Restore via rollback.
- **Sensitive-path denylist.** `/etc/shadow`, `/root/.ssh`, `/boot`, `/sys`, `/proc/sys`, etc. are refused unless explicitly opted into.
- **JWT auth.** bcrypt password hashing, login rate-limit (5/min/IP), `Authorization: Bearer` or `?access_token=` (for SSE/WS).
- **Security headers + CORS allowlist.** `X-Frame-Options: DENY`, CSP on the frontend, configurable origin allowlist for WS.

## Architecture

WirePanel is split into four logical layers. The split is enforced — Core never executes Linux commands, Agent never makes business decisions.

```
                         ┌─────────────────────────┐
                         │     Vue 3 + Tailwind    │
                         │       (Browser)         │
                         └────────────┬────────────┘
                                      │  HTTPS / REST / SSE
                                      │  WebSocket (web terminal)
                                      ▼
                         ┌─────────────────────────┐
                         │      Core (Go)          │
                         │  auth · RBAC · audit    │
                         │  WS hub · API gateway   │
                         └────────────┬────────────┘
                                      │
                       Agent dials in (NAT-friendly)
                                      │
            ┌─────────────────────────┼─────────────────────────┐
            ▼                         ▼                         ▼
      ┌─────────────┐           ┌─────────────┐           ┌─────────────┐
      │ Agent (Go)  │           │ Agent (Go)  │           │ Agent (Go)  │
      │  root       │           │  root       │           │  root       │
      │  node-1     │           │  node-2     │           │  node-N     │
      └──────┬──────┘           └──────┬──────┘           └──────┬──────┘
             │                         │                         │
             ▼                         ▼                         ▼
      ┌─────────────┐           ┌─────────────┐           ┌─────────────┐
      │   Linux     │           │   Linux     │           │   Linux     │
      │  systemd    │           │  systemd    │           │  systemd    │
      │  apt/dnf    │           │  apt/dnf    │           │  apt/dnf    │
      │  fs · net   │           │  fs · net   │           │  fs · net   │
      └─────────────┘           └─────────────┘           └─────────────┘
```

### Layer responsibilities

| Layer | Role | Boundary |
|---|---|---|
| **Frontend** (Vue 3) | UX, role-aware UI, typed-confirmation for destructive ops | Talks to **Core only**. Never directly to Agent. |
| **Core** (Go) | Auth, RBAC, audit, WS hub, REST/SSE gateway, agent registry | Sends commands to Agents. **No Linux execution. No distro logic.** |
| **Agent** (Go, root) | Executes shell, fs ops, systemctl, captures system info, runs PTY | Strictly follows Core instructions. **No business logic. No autonomy.** |
| **Modules** (TS, Bun) — *roadmap* | Service installers, config managers (WireGuard, Docker, Nginx, …) | Run inside Agent. UI shipped to Frontend via shared component system. |

### Communication model

- **Direction:** Agent → Core (the agent dials, not the other way around).
- **Why:** no inbound port on the agent host, easier NAT traversal, simpler firewalling.
- **Protocol:** WebSocket framed JSON envelopes (`{id, type, payload}`). Initial auth via shared agent token; per-agent token issuance is on the roadmap.
- **Browser → Agent is impossible by design.** All UI traffic terminates at Core; Core proxies the necessary frames to the relevant agent. The web terminal is a Core-proxied PTY — the browser never holds a connection to the agent.

### Repository layout

```
wirepanel/
├── core/              Go control plane
│   ├── cmd/wirepanel-core/     entry point
│   └── internal/
│       ├── agents/             registry
│       ├── api/                REST handlers, terminal proxy, middleware
│       ├── auth/               JWT + bcrypt + rate-limit + RBAC
│       ├── config/             env-driven config
│       ├── featboot/           edition gate (community // pro build-tag)
│       ├── fspath/             sensitive-path denylist
│       └── ws/                 agent dial-in hub
│
├── agent/             Go execution layer (runs as root on each node)
│   ├── cmd/wirepanel-agent/
│   └── internal/{client,exec,fs,svc,sysinfo,term,config}
│
├── shared/            Cross-module types + reference implementations
│   ├── proto/         WS message envelope + task kinds
│   ├── perms/         permission catalog
│   └── featgate/      Provider/Audit/Users/Roles interfaces +
│                      JSONL audit logger used by Community
│
├── frontend/          Vue 3 + Vite + Tailwind + xterm.js
│   └── src/{api,stores,router,views,components}
│
├── pro/               STUB module (real impl lives in private repo)
└── docs/              Edition model, ADRs (TBD)
```

## Quick start

### Prerequisites

- Go ≥ 1.25
- Bun (or Node 20+) for the frontend
- Linux with systemd for the agent (Debian/Ubuntu/RHEL/Arch/Alpine)

### Build

```bash
git clone https://github.com/5nYqnHvk/wirepanel
cd wirepanel
make tidy
make build
```

Three binaries / artifacts will land in `bin/` and `frontend/dist/`.

### Run (development)

```bash
# terminal A — Core
WP_ADMIN_USER=admin WP_ADMIN_PASS=wirepanel \
WP_AGENT_TOKEN=dev-shared-token \
make run-core

# terminal B — Agent (run as root for full system access)
sudo -E env \
  WP_CORE_URL=ws://127.0.0.1:8080/api/agent/ws \
  WP_AGENT_ID=node1 \
  WP_AGENT_TOKEN=dev-shared-token \
  make run-agent

# terminal C — Frontend dev server (proxies /api → :8080)
make run-frontend     # http://localhost:5173
```

Default login: `admin` / `wirepanel`. **Change it immediately.**

### Production

- Set `WP_ENV=production` (refuses default JWT secret).
- Provide TLS via `WP_TLS_CERT` + `WP_TLS_KEY` or terminate TLS upstream.
- Restrict origins via `WP_ALLOWED_ORIGINS=https://panel.example.com`.
- Rotate `WP_JWT_SECRET` and `WP_AGENT_TOKEN`.
- Build the frontend (`bun run build`) and serve `frontend/dist` from a reverse proxy that forwards `/api` to Core.

## Configuration

### Core

| Variable | Default | Purpose |
|---|---|---|
| `WP_ENV` | `development` | `production` enforces non-default JWT secret |
| `WP_HTTP_ADDR` | `:8080` | listen address |
| `WP_WS_PATH` | `/api/agent/ws` | agent dial-in path |
| `WP_AGENT_TOKEN` | `dev-shared-token` | shared bootstrap token agents present at handshake |
| `WP_JWT_SECRET` | `dev-secret-change-me` | HS256 signing secret |
| `WP_ADMIN_USER` / `WP_ADMIN_PASS` | `admin` / `wirepanel` | bootstrap-only, first boot creates the owner |
| `WP_DATA_DIR` | `./data` | persistent state (users.json, etc.) |
| `WP_AUDIT_DIR` | `./data/audit` | JSONL audit log + trash |
| `WP_ALLOWED_ORIGINS` | `*` | comma-separated CORS / WS origin allowlist |
| `WP_TLS_CERT` / `WP_TLS_KEY` | (unset) | enables HTTPS |
| `WP_FS_ALLOW_SENSITIVE` | `false` | overrides the sensitive-path denylist |

### Agent

| Variable | Default |
|---|---|
| `WP_CORE_URL` | `ws://127.0.0.1:8080/api/agent/ws` |
| `WP_AGENT_ID` | machine hostname |
| `WP_AGENT_TOKEN` | `dev-shared-token` |

## API

Auth on every `/api/*` endpoint except `/api/auth/login` and `/api/info`.
JWT is supplied via `Authorization: Bearer <t>` or `?access_token=<t>` (SSE / WS).

| Method | Path | Permission | Notes |
|---|---|---|---|
| `GET` | `/healthz` | — | liveness |
| `GET` | `/api/info` | — | edition, audit enabled, permission catalog |
| `POST` | `/api/auth/login` | — | bcrypt → JWT |
| `GET` | `/api/auth/me` | authed | identity + effective permissions |
| `GET` | `/api/agents` | `agents.read` | list registered agents |
| `GET` | `/api/agents/{id}/system` | `system.read` | snapshot |
| `GET` | `/api/agents/{id}/services` | `services.read` | systemd units |
| `POST` | `/api/agents/{id}/services/{name}/action` | `services.*` | start/stop/restart/enable/disable, **`confirm: true` required** |
| `GET / POST` | `/api/agents/{id}/fs/list` | `fs.read` | dir listing |
| `POST` | `/api/agents/{id}/fs/read` | `fs.read` | file content (≤8 MB) |
| `POST` | `/api/agents/{id}/fs/write` | `fs.write` | **`confirm: true`** |
| `POST` | `/api/agents/{id}/fs/mkdir` | `fs.mkdir` | **`confirm: true`** |
| `POST` | `/api/agents/{id}/fs/delete` | `fs.delete` | **`confirm: true` + `confirm_path` matching the absolute path** |
| `GET` (WS) | `/api/agents/{id}/terminal` | `terminal` | browser PTY through Core |
| `POST` | `/api/tasks` | `shell.exec` | dispatch shell, **irreversible**, requires `confirm: true` |
| `GET` (SSE) | `/api/tasks/stream?task_id=` | authed | log + result stream |
| `GET` | `/api/audit` | `audit.read` | recent entries |
| `GET` | `/api/audit/{id}` | `audit.read` | entry detail |
| `POST` | `/api/audit/{id}/rollback` | `audit.rollback` | replays inverse op |
| `*` | `/api/roles`, `/api/users` | Team+ only | Discord-style RBAC |
| WS | `/api/agent/ws` | shared token | agent dial-in |

## Editions

WirePanel ships in four editions — the split is **value-based, not crippling**. Community is **production-ready** for daily use on unlimited nodes.

| Tier | Audience | What's added |
|---|---|---|
| **Community** | self-hosters, hobbyists | full ops + audit + basic rollback; single user |
| **Pro** | indie sysadmins, consultants | + Intelligence Layer: config history & diff, smart change analysis, health insights, recommendations, historical analytics |
| **Team** | small companies | + 5 users, Discord-style RBAC, shared audit |
| **Enterprise** | compliance-driven orgs | + unlimited users, SSO, compliance, policy controls, private registries, retention |

The edition is **baked in at compile time** via Go build tags — there is no runtime feature flag to flip:

```bash
# Community (default; this repo)
go build

# Pro / Team / Enterprise — requires the private wirepanel-pro module to be present
go build -tags pro
```

Public source contains only Community. Pro is a separate private Go module published as a SaaS offering. See [`docs/EDITIONS.md`](./docs/EDITIONS.md) for the full breakdown.

## Safety model

Three layers prevent the kind of "I just `rm -rf /etc` by accident" disaster you'd expect from a root-privileged management tool:

1. **Server-side typed-confirmation.** Every dangerous endpoint requires `confirm: true` in the body. `fs.delete` additionally requires `confirm_path` to match the absolute resolved path; a mismatch is a `400`.
2. **Frontend typed-confirmation modal.** The user must literally type the target path/service-name/audit-id-prefix into a text field before the destructive button enables.
3. **Rollback by default.** `fs.write` snapshots prior content; `fs.delete` moves into an audit-scoped trash directory; `fs.mkdir` records whether the path was newly created; `service.ctl` records prior `is-active` / `is-enabled`. One API call (`/api/audit/{id}/rollback`) reverses any of these.

Irreversible ops (currently only `shell.exec`) are clearly flagged in the audit log and the UI as non-rollbackable.

## Roadmap

**Phase 1 — Operational MVP** *(done)*
- WS hub, agent registration, multi-server dashboard
- Web terminal, file manager, service manager, system snapshot
- Realtime task system + SSE log stream
- Audit log + rollback (community built-in)
- Edition compile-time split

**Phase 2 — Modules**
- Bun-based module loader running inside the Agent
- Official modules: WireGuard, Docker, Nginx, PostgreSQL, Node.js
- GitHub-registry-backed module marketplace
- Per-agent token issuance (replace the bootstrap shared token)

**Phase 3 — Pro Intelligence Layer** *(private module)*
- Config history + diff, smart change analysis, recommendations
- Health insights and historical analytics

## Contributing

WirePanel adheres to a **locked architecture**. Before opening a structural PR, read the architecture section above. Specifically:

- The Core does **not** execute Linux commands.
- The Agent does **not** make decisions.
- The Browser does **not** talk to the Agent directly.
- We are **not** building Kubernetes, an orchestrator, or autonomous agents.

Bug fixes, new modules, frontend polish, distro coverage, and documentation are very welcome.

## License

AGPL-3.0. See [`LICENSE`](./LICENSE).

The `pro/` module in this repository is a public stub — the real Pro/Team/Enterprise implementation lives in a separate private repository and is not covered by AGPL.
