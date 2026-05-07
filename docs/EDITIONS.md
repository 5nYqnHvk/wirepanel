# WirePanel Editions

> "**No node limits. No feature crippling.**"

WirePanel ships in four editions. The split is **value-based**, not feature-crippled.

## Community — Free, full operational access

Everything you need to run Linux infrastructure on N machines, no artificial limits.

| Capability | Included |
|---|---|
| Unlimited nodes / agents | ✅ |
| Multi-distro (Debian / RHEL / Arch / Alpine) | ✅ |
| Terminal (web PTY) | ✅ |
| File manager | ✅ |
| Service manager (systemctl) | ✅ |
| Task execution (shell + structured) | ✅ |
| Realtime logs (SSE / WS) | ✅ |
| All official modules | ✅ |
| Module marketplace | ✅ |
| Basic audit log (append-only JSONL) | ✅ |
| Basic rollback (inverse ops, pre-image trash) | ✅ |
| Single user (admin / owner) | ✅ |

**Selling point:** Community is **production-ready**. Self-hosters get the entire operational stack with zero crippling.

## Pro — Intelligence Layer (solo power users)

Pro adds analysis features on top of Community. **No multi-user.** Same single-user model.

- **Config History + Diff** — every config write produces a versioned diff (`nginx.conf old vs new`)
- **Smart Change Analysis** — syntax risk, dependency impact, possible conflicts before apply
- **Config Rollback** — beyond basic inverse-ops; structured snapshot rollback per config
- **Health Insights** — memory-pressure trends, service instability, unusual restart frequency
- **Recommendations** — `nginx worker count too low`, `postgres tuning suggestion`, `disk cleanup`
- **Historical Analytics** — service trends, usage pattern, failure history

Pro = "**think, analyze, manage deeper**." For indie sysadmins / consultants running their own boxes.

## Team — Pro + collaboration

Everything in Pro plus multi-user collaboration up to **5 users**.

- Up to 5 users
- **Discord-style RBAC** — create roles from scratch, assign permissions explicitly, no preset roles
- Shared audit (centralized log of all team members' actions)

## Enterprise — Team + governance

Everything in Team plus enterprise governance.

- Unlimited users
- SSO (SAML / OIDC)
- Compliance reports (SOC2, HIPAA-friendly trails)
- Policy controls (mandatory approval flows, restricted command lists)
- Private module registries
- Retention controls (audit log retention windows, data residency)

## Build / Distribution model

The split is **compile-time**, not runtime flags.

- Public open-source repo: `core/`, `agent/`, `shared/`, `frontend/`
  - Builds Community by default: `go build`
- Private SaaS repo: `wirepanel-pro` (separate Go module, **not public**)
  - Provides Intelligence + Team + Enterprise impls behind a `Provider` interface (`shared/featgate`)
  - Pulled in only with `-tags pro`
  - Build command: `go build -tags pro`
  - If the private dep is missing, the pro build **fails to compile** — there is no fake-pro mode

This means:
- Anyone can fork / inspect Community in full
- Pro features cannot be reverse-engineered from public source — they live in a separate private module
- One binary per tier; tier is baked into the binary at build time

## Sales positioning

| Tier | Audience | Pitch |
|---|---|---|
| Community | self-hosters, hobbyists | "All Linux management, free, no limits" |
| Pro | indie sysadmins, consultants | "Add intelligence, see *why*, get suggestions" |
| Team | small companies | "Pro plus collaborate with up to 5 people" |
| Enterprise | compliance-driven orgs | "SSO, policy, audit retention, private registries" |
