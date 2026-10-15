# PRODUCT.md implementation status

This file is the acceptance ledger for `PRODUCT.md`. A feature is implemented only when it has a working API or runtime integration, UI where required, authorization, validation, tests, and durable storage where the contract requires durability. Demo fixtures and in-memory stores are identified explicitly and are not production-complete.

Last reviewed: 2026-10-15 (Configuration revision milestone).

## Working now

- Go/Chi server, embedded React UI (React Router + URL `?tab=` state), `/api/v1` routing, request IDs, recovery, timeouts, security headers, health/readiness endpoints, structured startup logging, and graceful shutdown.
- Original responsive administrator shell built with `@kiteapps/ui` and Space Grotesk; nav aligned to ADSelfService Plus admin IA (Dashboard, Configuration, Policy Configuration, Directory Users, Reports, Audit, Admin).
- Protected administrator APIs using a constant-time bearer-token bootstrap mechanism. This is development access control, not the final session/RBAC design.
- Directory provider defaults to LDAP from `config.yaml` (`mode: ldap`, `writeEnabled: false`). Connection test, OU discovery, group search, and user search hit AD; password reset/unlock remain blocked until writes are enabled with LDAPS.
- MySQL `directory_desk` database with migrate-on-startup; durable policies (OU/group targets + precedence), audit events, settings, and configuration profiles.
- Dashboard aggregates from AD sample + audit + policies (`GET /api/v1/admin/dashboard/`); AD bind failures surface as attention items without hard-failing the page.
- First report pack: locked/expired users from AD; password-reset and unlock activity from MySQL audit; MFA reports honestly marked unavailable.
- Capability catalog statuses reflect durable storage / AD integration honesty (no fake “configured” MFA).
- All audited Configuration navigation modules expose server-defined, validated forms backed by MySQL configuration profiles. Successful saves create immutable numbered revisions and configuration audit events; validation failures are audited without persisting rejected values.
- The Configuration frontend tracks unsaved changes, disables redundant writes, displays the current revision, and exposes recent revision metadata.
- Configuration writes use optimistic concurrency: stale browser state receives an explicit conflict instead of silently overwriting a newer administrator change. Administrators can restore an earlier revision through a two-step frontend action; restoration creates a new immutable revision and audit event.
- Docker/Compose assets, environment template, OpenAPI document, Go tests, TypeScript checks, and production frontend build.

## Phase 1 gaps

`PRODUCT.md` defines Phase 1 as complete only with sessions, RBAC, encrypted credentials, jobs, migrations, authorization, audit, UI states, and automated tests. The following remain:

- Database connection pool, migration runner, MySQL repositories, upgrade tests, and transaction/concurrency tests.
- Redis-backed revocable sessions, CSRF protection, role and scoped-permission enforcement, privileged MFA, login/logout, and administrator identity attribution.
- Encryption or secret-manager references for directory and integration credentials. Credentials must not remain directly in `config.yaml` for production.
- Persistent policy engine with OU/group targets, exclusions, priority/conflict handling, history, and enforcement on recovery operations.
- Directory synchronization and background workers with durable jobs, retries, outbox delivery, and aggregate refresh state.
- Production report queries, domain/OU/time/status/source filters, pagination, scheduling, saved views, and audited exports.
- Real dashboard aggregates, drill-downs, licenses, freshness/failure states, and background refresh.
- Idempotency controls and distributed rate limiting for password reset and account unlock.

## Later phases not implemented

- Employee portal and public login, forgot/reset password, unlock, profile, applications, groups, directory, request history, and session-management routes.
- TOTP, recovery codes, WebAuthn/passkeys, security questions, email/SMS verification, authenticator enrollment, and MFA enforcement/reporting.
- Password-policy enforcement, expiry notifications, localized templates, delivery providers, retries, and history.
- Groups, application assignments, approval workflows, technicians, delegated administration, mobile deployment, and endpoint-agent lifecycle.
- SAML/OIDC applications, conditional access evaluation/simulation, JIT provisioning, and enterprise provider adapters.
- Failover, reverse proxy, certificate rotation, mail/SMS providers, scheduler management, encrypted backup/restore, integrations, licensing, and complete security-posture scoring.
- OpenTelemetry metrics/tracing, production CORS policy, security-event response, retention/redaction/export/deletion workflows, CI pipeline, and production hardening.

## Verification commands

```bash
go test ./...
go vet ./...
cd frontend
bunx tsc --noEmit
bun run build
```

The local browser smoke test covers administrator authentication, directory-user loading, an audited mock unlock, and audit-event rendering. No control backed only by a mock or in-memory success should be described as production-complete.
