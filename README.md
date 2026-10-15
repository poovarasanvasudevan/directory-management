# Directory Desk

Directory Desk is an original enterprise identity self-service platform implemented as a Go/Chi modular monolith with an embedded React and KiteFrame frontend. [PRODUCT.md](PRODUCT.md) is the product contract; this repository is being delivered milestone by milestone.

## Architecture

Current request flow:

`React → Chi middleware → versioned handler → directory service interface → mock or LDAP adapter`

The foundation keeps transport, configuration, directory integration, and UI delivery separate. MySQL is the planned durable source of truth and Redis is reserved for rate limiting, sessions, caching, and jobs. The initial schema establishes tenant-aware users, RBAC, sessions, directories, policies, and audit events.

## Implemented

- Embedded React administrator console using `@kiteapps/ui`
- Sanitized Active Directory configuration, connection testing, and OU discovery
- Mock directory provider for local development and LDAP/LDAPS/StartTLS provider
- `/api/v1` routing with request IDs, real-IP handling, recovery, timeouts, security headers, and administrator authorization
- Health and readiness endpoints
- Structured `slog` startup/error logging and graceful shutdown
- Initial MySQL 8.4 migration with tenant-aware indexes and constraints
- OpenAPI for every currently implemented API
- Protected categorized reports with filtering and backend CSV export
- Protected directory-user search, safely gated password reset/account unlock, and an audit-event screen
- Typed configuration forms for the audited configuration menu, backed by server validation
- Dockerfile, Compose definition, and environment template

The bootstrap administrator bearer token is milestone-one access control, not the final authentication system. Database-backed users, revocable sessions, RBAC, and CSRF protection must replace it before production deployment.

## Local development

Local development reads directly from `config.yaml`; a `.env` file is not required. Configure these values before starting:

```yaml
app:
  environment: development

server:
  address: ":8080"

activeDirectory:
  mode: mock
  writeEnabled: true
```

Then run:

```bash
go run ./pkg/cmd
```

Open `http://localhost:8080`. This portal is an administrator console; API routes under `/api/v1/admin` are open to the application without an API token.

For LDAP mode, set `activeDirectory.mode: ldap` and configure the connection and bind credentials in `config.yaml`. Restrict this file to the application account, for example with `chmod 600 config.yaml`. Production deployments should replace plaintext credentials with encrypted secret references or a secret manager.

## API

- `GET /healthz`
- `GET /readyz`
- `GET /api/v1/admin/directories/config`
- `POST /api/v1/admin/directories/test`
- `GET /api/v1/admin/directories/organizational-units`
- `GET /api/v1/admin/reports/`
- `GET /api/v1/admin/reports/{reportID}`
- `GET /api/v1/admin/reports/{reportID}/export`
- `GET /api/v1/admin/users/`
- `POST /api/v1/admin/users/{userID}/password-reset`
- `POST /api/v1/admin/users/{userID}/unlock`
- `GET /api/v1/admin/audit-events/`

See [api/openapi.yaml](api/openapi.yaml).

## Verification

```bash
go test ./...
go vet ./...
cd frontend
bunx tsc --noEmit
bun run build
```

Docker Compose validation and MySQL migration execution require Docker, which is not available in the current development environment.

## Milestones

- [x] Repository and KiteFrame inspection
- [x] Chi/React foundation, versioned routing, middleware, mock/LDAP directory adapters
- [x] Initial MySQL schema, OpenAPI, Docker assets, environment template
- [ ] MySQL connection layer, migration runner, Redis health/readiness, integration tests
- [ ] Tenant users, Argon2id login, revocable sessions, RBAC, CSRF, audit writer
- [ ] Employee portal and route protection
- [ ] Password recovery, account unlock, directory writes, rate limiting
- [ ] TOTP, recovery codes, WebAuthn/passkeys
- [ ] Persistent policy engine and directory synchronization
- [ ] Groups, applications, approvals, notifications, SSO extensions
- [ ] Reporting, CSV export, worker, OpenTelemetry, production hardening

No unchecked milestone is presented as implemented or production-ready. See [IMPLEMENTATION_STATUS.md](IMPLEMENTATION_STATUS.md) for the requirement-by-requirement acceptance ledger.
