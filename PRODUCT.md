Master Development Prompt: Enterprise Identity Self-Service Platform
1. Project objective

Build a production-oriented enterprise identity self-service application inspired by the functionality of https://demo.adselfserviceplus.com/.

The product should provide secure employee password self-service, account unlock, MFA, directory integrations, SSO, user and group management, approval workflows, audit reports, and an enterprise administration console.

Develop an original implementation with its own branding and UI.

2. Mandatory technology stack
   Backend
   Language: Go (Golang)
   HTTP router: Chi (github.com/go-chi/chi/v5)
   Database: MySQL 8.4 LTS
   Database driver: github.com/go-sql-driver/mysql
   Database access: sqlc with database/sql, or a consistent repository pattern using database/sql
   Database migrations: golang-migrate/migrate or an equivalent maintained migration tool
   Cache and job coordination: Redis
   API: REST with JSON
   API specification: OpenAPI 3.x
   Authentication: secure server-side sessions or a carefully designed token-based architecture
   Logging: Go log/slog
   Validation: explicit request validation and domain-level validation
   Observability: OpenTelemetry-compatible logs, metrics, and tracing

Use Chi middleware for request IDs, structured request logging, panic recovery, timeouts, CORS, and other appropriate HTTP middleware.

Frontend
React
TypeScript in strict mode
Vite
@kiteapps/ui as the primary UI component library
React Router
TanStack Query
React Hook Form
Zod
Responsive CSS and accessible design tokens

Inspect the installed @kiteapps/ui package and its actual documentation and exports before using its components. Never invent unsupported component names or properties.

Infrastructure
MySQL 8.4 LTS
Redis
Docker and Docker Compose
SMTP-compatible email provider
GitHub Actions or equivalent CI pipeline
3. Architecture

Use a modular monolith with clean separation between transport, business logic, persistence, integrations, and background jobs.

Recommended request flow:

React frontend → Chi router and middleware → HTTP handlers → domain services → repositories → MySQL.

Use Redis for rate limiting, caching, distributed job coordination, and other justified use cases. MySQL remains the durable source of truth for application data.

Keep business logic independent of Chi handlers and database implementation details.

4. Recommended repository structure
   identity-selfservice/
   ├── cmd/
   │   ├── api/
   │   │   └── main.go
   │   └── worker/
   │       └── main.go
   ├── internal/
   │   ├── config/
   │   ├── server/
   │   ├── middleware/
   │   ├── auth/
   │   ├── authorization/
   │   ├── tenants/
   │   ├── users/
   │   ├── password/
   │   ├── mfa/
   │   ├── directories/
   │   ├── policies/
   │   ├── applications/
   │   ├── groups/
   │   ├── workflows/
   │   ├── notifications/
   │   ├── reports/
   │   ├── audit/
   │   ├── integrations/
   │   ├── jobs/
   │   ├── database/
   │   └── crypto/
   ├── migrations/
   ├── queries/
   ├── api/
   │   └── openapi.yaml
   ├── web/
   │   ├── src/
   │   │   ├── app/
   │   │   ├── components/
   │   │   ├── features/
   │   │   ├── layouts/
   │   │   ├── routes/
   │   │   ├── hooks/
   │   │   ├── services/
   │   │   ├── types/
   │   │   └── styles/
   │   └── package.json
   ├── deploy/
   │   └── docker/
   ├── tests/
   ├── docker-compose.yml
   ├── Dockerfile
   ├── .env.example
   ├── go.mod
   └── README.md


Organize packages by business capability. Avoid circular dependencies, global mutable state, and unnecessarily complicated abstractions.

5. MySQL database requirements

Use MySQL 8.4 LTS with InnoDB, utf8mb4, UTC timestamps, foreign keys, indexes, and explicit transaction boundaries.

Design normalized tables for:

tenants
users
user_profiles
roles
permissions
user_roles
sessions
directories
directory_sync_jobs
directory_objects
mfa_enrollments
recovery_requests
policies
policy_assignments
enterprise_applications
application_assignments
groups
group_memberships
access_requests
approval_workflows
workflow_stages
workflow_instances
workflow_decisions
notification_templates
notification_jobs
notification_deliveries
audit_events
integration_configs
security_events
MySQL implementation rules
Use BIGINT identifiers or UUIDs stored using an explicitly documented representation.
Use DATETIME(6) with a consistent UTC convention for timestamps.
Add tenant-aware indexes and uniqueness constraints where appropriate.
Use prepared statements and parameterized queries exclusively for dynamic user input.
Use transactions for password recovery, approval decisions, group changes, and other multi-record operations.
Use SELECT ... FOR UPDATE or atomic conditional updates when concurrency control is required.
Configure connection pooling, connection lifetime, query timeouts, and health checks.
Use keyset pagination for large datasets when appropriate.
Implement versioned database migrations and test both fresh installation and upgrades.
Use an outbox pattern for reliable delivery of notifications and background jobs when needed.
Never store plaintext passwords, OTPs, recovery tokens, session secrets, or unencrypted integration credentials.
Do not depend on SQLite or PostgreSQL-specific SQL features.

Test transaction rollback, deadlocks, concurrent requests, unique constraints, and migration behavior against MySQL itself.

6. Chi HTTP server

Build the HTTP server using Chi.

Use:

chi.NewRouter()
Explicit route registration
Versioned API routes under /api/v1
Middleware for request IDs, logging, recovery, timeouts, and security controls
Authentication and authorization middleware
Route-level permission checks
JSON request decoding with payload limits
Centralized error responses
Graceful shutdown with context cancellation
Health and readiness endpoints

Suggested route organization:

r := chi.NewRouter()

r.Use(middleware.RequestID)
r.Use(middleware.RealIP)
r.Use(middleware.Recoverer)
r.Use(middleware.Timeout(30 * time.Second))

r.Get("/healthz", healthHandler)
r.Get("/readyz", readinessHandler)

r.Route("/api/v1", func(r chi.Router) {
r.Route("/auth", authRoutes)
r.Route("/recovery", recoveryRoutes)

    r.Group(func(r chi.Router) {
        r.Use(authenticationMiddleware)

        r.Get("/me", getCurrentUser)
        r.Route("/me/profile", profileRoutes)
        r.Route("/me/security", securityRoutes)
        r.Route("/me/requests", requestRoutes)

        r.Group(func(r chi.Router) {
            r.Use(adminAuthorizationMiddleware)

            r.Route("/admin/users", adminUserRoutes)
            r.Route("/admin/directories", directoryRoutes)
            r.Route("/admin/policies", policyRoutes)
            r.Route("/admin/audit-events", auditRoutes)
        })
    })
})


Treat this as illustrative routing structure. Implement the route functions and middleware with explicit dependencies, typed service interfaces, and appropriate authorization checks.

Do not register protected routes without real authentication and authorization implementations.

7. Product modules
   A. Employee self-service

Implement:

Login and logout.
Forgot password.
Password reset and authenticated password change.
Account unlock.
MFA enrollment and management.
Recovery codes.
Profile editing.
Group membership requests.
Enterprise application shortcuts.
Employee directory.
Organization chart.
Active session management.
Request status and history.
B. Administrator console

Implement:

Identity activity dashboard.
User and account management.
Directory connections and synchronization.
Password and account recovery policies.
MFA enforcement.
Group and application assignment management.
Approval workflows.
Notification configuration.
Audit reports and CSV export.
Tenant and portal settings.
C. Active Directory and LDAP

Create a directory-provider interface with:

Mock directory provider for local development.
LDAP/LDAPS connector.
Microsoft Active Directory support.
Connection testing.
Directory search and synchronization.
User and group lookup.
Password changes through supported directory mechanisms.
Account unlock through supported directory operations.
Explicit read-only and write-enabled modes.

Validate TLS certificates, encrypt integration secrets, restrict directory permissions, and audit every sensitive write operation.

D. MFA and enterprise authentication

Implement TOTP, recovery codes, and WebAuthn/passkeys where supported.

Provide extension points for SAML 2.0, OIDC, and additional enterprise identity providers.

Use standards-compliant libraries. Validate redirect URIs, issuer, audience, signatures, state, nonce, and token expiry as appropriate.

E. Policies and workflows

Create a backend policy engine supporting:

Password and account recovery permissions.
MFA requirements.
Directory attribute editing rules.
Group membership restrictions.
User and organizational unit targeting.
Approval requirements.
Policy priorities and change history.

Implement workflow states, approvers, approval decisions, notifications, retries, and audit history.

Enforce all policies on the server.

8. Frontend implementation

Use @kiteapps/ui for supported components, and React with TypeScript for the UI.

Create two separate experiences sharing the same design system:

Employee portal

/portal
/portal/password
/portal/security
/portal/profile
/portal/applications
/portal/groups
/portal/directory
/portal/requests
/portal/settings

Administrator console

/admin/dashboard
/admin/users
/admin/directories
/admin/policies
/admin/mfa
/admin/applications
/admin/groups
/admin/workflows
/admin/notifications
/admin/reports
/admin/audit
/admin/integrations
/admin/settings

Public routes

/login
/forgot-password
/reset-password
/unlock-account
/mfa/verify

Build a responsive sidebar, top navigation, breadcrumbs, reusable tables, filters, dialogs, form components, status indicators, and dashboard charts.

Use real API responses for production dashboards. Demo fixtures must be clearly isolated from production behavior.

9. API conventions

All APIs use /api/v1 and JSON.

Implement consistent:

Request and response schemas.
Input validation.
Pagination and filtering.
Error codes.
HTTP status codes.
Request IDs.
OpenAPI documentation.
Authentication and authorization requirements.
Idempotency for sensitive operations where needed.

Example endpoints:

POST   /api/v1/auth/login
POST   /api/v1/auth/logout
POST   /api/v1/recovery/password
POST   /api/v1/recovery/password/verify
POST   /api/v1/recovery/password/complete
POST   /api/v1/recovery/unlock

GET    /api/v1/me
PATCH  /api/v1/me/profile
POST   /api/v1/me/password/change
GET    /api/v1/me/requests

GET    /api/v1/admin/dashboard
GET    /api/v1/admin/users
GET    /api/v1/admin/directories
POST   /api/v1/admin/directories
POST   /api/v1/admin/directories/{id}/test
POST   /api/v1/admin/directories/{id}/sync
GET    /api/v1/admin/policies
POST   /api/v1/admin/policies
GET    /api/v1/admin/audit-events


Generate and maintain an OpenAPI specification that matches the implemented handlers.

10. Security requirements

Follow OWASP guidance and use secure defaults.

Implement:

Argon2id for locally managed password hashes.
Secure session management and session revocation.
HttpOnly, Secure, and appropriate SameSite cookie attributes.
CSRF protection for cookie-authenticated state-changing requests.
Strict CORS allowlists.
Rate limiting for authentication and recovery.
MFA for privileged users and sensitive operations.
RBAC and server-side policy enforcement.
Tenant isolation.
SQL injection prevention.
Encryption of integration secrets.
Secure recovery-token storage and one-time consumption.
TLS for production traffic and directory connections.
Audit logging for sensitive operations.
Secret redaction and safe error responses.
Dependency and container security scanning.

Use secure random tokens and store token hashes when appropriate. Never log passwords, tokens, OTPs, cookies, or authorization headers.

11. Docker and configuration

Provide Docker Compose services for:

MySQL 8.4 LTS.
Redis.
Go API.
Go background worker.
React development server or production frontend.

Configure the application using environment variables, including:

APP_ENV=development
HTTP_ADDR=:8080

MYSQL_HOST=mysql
MYSQL_PORT=3306
MYSQL_DATABASE=identity_selfservice
MYSQL_USER=identity_app
MYSQL_PASSWORD=change_me
MYSQL_PARAMS=parseTime=true&charset=utf8mb4

REDIS_ADDR=redis:6379

SESSION_SECRET=replace_with_a_secure_random_value
ENCRYPTION_KEY=configure_through_a_secure_secret_manager

SMTP_HOST=
SMTP_PORT=587
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM=

DIRECTORY_MODE=mock


This is an illustrative configuration template. Implement proper secret loading and validation, and never commit production credentials.

Use a dedicated least-privilege MySQL application account rather than the root account. Provide persistent MySQL storage, health checks, migration execution, and graceful shutdown.

12. Testing

Backend:

Go unit tests.
Chi handler tests.
MySQL integration tests.
Migration tests.
Redis integration tests.
Authorization tests.
Tenant isolation tests.
Password recovery and MFA tests.
Directory adapter tests.
Workflow concurrency tests.

Frontend:

Component tests.
Form validation tests.
API integration tests.
Route protection tests.
Playwright end-to-end tests.
Accessibility tests.

Security:

Cross-tenant access attempts.
Unauthorized administrative actions.
Recovery token reuse.
MFA bypass attempts.
CSRF behavior.
Rate limiting.
Session revocation.
SQL injection and validation cases.

Run the tests against MySQL 8.4 LTS. Do not substitute another database for database-specific integration testing.

13. Implementation milestones
    Inspect the repository and @kiteapps/ui APIs.
    Set up Go, Chi, MySQL, Redis, migrations, and Docker.
    Implement users, tenants, authentication, sessions, RBAC, and audit logging.
    Build the shared frontend layout and employee portal.
    Implement password recovery, account unlock, and MFA.
    Add mock and real LDAP/Active Directory providers.
    Implement policy management and directory administration.
    Add groups, approval workflows, enterprise applications, and SSO.
    Complete reporting, notifications, and operational monitoring.
    Execute security tests, frontend end-to-end tests, and deployment validation.

After every milestone, run formatting, compilation, tests, and relevant integration checks.

14. Deliverables and acceptance criteria

Deliver complete source code, MySQL migrations, seed scripts, OpenAPI documentation, frontend components, tests, Docker configuration, .env.example, and a README.

The product is complete only when:

Go compiles and tests pass.
React builds successfully.
MySQL migrations run correctly.
Authentication and account recovery execute real backend workflows.
Directory operations work with the mock provider and each claimed real integration.
Administrative policies are enforced by the backend.
Sensitive actions produce audit events.
Tenant isolation is verified.
The UI is responsive and accessible.
No core feature falsely reports success without completing its operation.
Known limitations and unimplemented integrations are documented.

Begin by inspecting the existing repository and @kiteapps/ui package. Present the architecture, database schema, API plan, and milestone checklist first. Then implement the foundation and proceed module by module. Show changed files, actual test commands and results, and remaining gaps at every milestone.

15. Live demo audit and detailed functional requirements

Audit basis

These requirements were recorded from the authenticated ADSelfService Plus online demo at https://demo.adselfserviceplus.com/ on 2026-10-14. The demo identified itself as version 7.0, build 7001. Its public Administrator launcher authenticated the session as `admin` without exposing credentials. The site pins the browser session to that role, so the separate Domain User launcher could not be audited in the same session. Some administrative mutations are intentionally disabled in the demo; a disabled demo control is not evidence that the corresponding workflow can be omitted.

This is a behavioral inventory, not a requirement to copy ManageEngine branding, assets, styling, text, or implementation. Build an original product using DESIGN.md and @kiteapps/ui.

15.1 Global shell and access model

- Identity-provider context switcher for Active Directory and future Microsoft Entra ID providers.
- Primary navigation: Dashboard, Reports, Configuration, Admin, and Support.
- Global employee search, current-user menu, domain settings, domain selector, and severity-grouped notifications.
- Current-user actions: personalisation, organisation chart, mobile access, password change, help, and sign out.
- Server-enforced route and action permissions; hiding a menu item is not authorization.
- Responsive, keyboard-accessible navigation with loading, empty, error, and stale-data states.
- Consistent list behavior: search, filters, sorting, pagination, page size, refresh, selection, and authorized bulk actions.
- Confirmation and audit events for destructive or security-sensitive operations.

Roles must include employee/domain user, technician, auditor/read-only administrator, directory administrator, security administrator, and product administrator. Custom roles must combine granular permissions with domain/OU scope. Privileged accounts require MFA.

15.2 Dashboard

Provide domain-scoped widgets for:

- Soon-to-expire and expired passwords.
- Enrolled and non-enrolled users.
- Used and remaining licenses.
- Endpoint MFA license usage.
- Locked-out, soon-to-expire, and password-expired user counts.
- Enrollment distribution.
- Password reset, unlock, self-update, change-password, and MFA-failure activity.
- Shortcuts to reset/unlock, self-update/change, expiration notifications, SSO/password sync, conditional access, MFA methods, mobile deployment, employee search, mail groups, approval workflows, cached credentials, and endpoint agents.

Widgets must expose last refresh, failure/stale state, and permission-aware drill-down. Background jobs update aggregates without blocking HTTP requests.

15.3 Policy configuration

Administrators can create, view, clone, edit, prioritize, enable, disable, and delete policies. Each policy has a name, description, domain, priority, target users/groups/OUs, exclusions, permissions, MFA rules, allowed channels, version, modifier, timestamps, and immutable change history.

Self-service permissions are reset password, unlock account, change password, and self-update. Advanced policy settings include automatic reset/unlock. The list shows actions, advanced settings, name, permissions, domain, modifier, and modified time. The backend detects conflicting assignments and evaluates precedence deterministically.

15.4 Multi-factor authentication

MFA configuration is policy-scoped with separate enrollment, reset/unlock, endpoint-login, application-login, and advanced settings. The provider model must support:

- Security questions, email codes, and SMS codes.
- Standards-compatible TOTP, including Google and Microsoft Authenticator use cases.
- Microsoft Entra ID MFA, Duo, RSA SecurID, and RADIUS adapters.
- Push, supported device biometrics, and QR authentication.
- Product mobile-app TOTP, SAML, and Active Directory security questions.
- YubiKey, smart card, custom TOTP, Zoho OneAuth-compatible extension, and FIDO2/WebAuthn passkeys.

Security-question settings include administrator-defined and user-defined question counts, editable question sets, min/max question length, min/max answer length, and partial-enrollment rules.

Every authenticator needs configured/unconfigured/disabled state, connection/setup validation, enrollment counts, policy assignment, recovery behavior, secure rotation where applicable, and audit events. Secret material and recovery codes are one-time display only.

15.5 Password lifecycle

Expiration notifications:

- Directory/policy/OU targeting and exclusions.
- Repeated thresholds before expiration, post-expiry reminders, and optional manager/admin escalation.
- Email, SMS, and push templates with locale, variables, preview, and test delivery.
- Timezone-aware schedules, retries, suppression, and delivery history.

Password policy enforcer:

- Minimum/maximum length and required uppercase, lowercase, number, special, and Unicode counts.
- Starting/ending character restrictions, repetition, sequences, patterns, username/display-name, dictionaries, breached-password checks, history, and age.
- Optional complexity override at a configured longer length and configurable number of rules that must pass.
- User-facing policy preview and enforcement for web, endpoint-agent, and supported administrative reset paths.
- Machine-readable validation failures that do not expose directory internals.

15.6 Enterprise applications and conditional access

Application management shows application name, provider/service, assigned policies, protocol/module, identity-provider details, certificate expiry, and actions. Implement provider adapters for SAML 2.0 and OIDC/OAuth, including metadata, redirect/ACS validation, attribute mapping, JIT provisioning, certificate/key rotation, policy assignment, test login, and access audit.

Conditional access provides rule configuration, assignment, precedence, readable flow visualization, and simulation. Conditions include identity/group, network/IP, trusted device, platform, access channel, application, time, and extensible location/risk signals. Actions include allow, deny, require MFA, and require a selected authenticator. Evaluations are deterministic, server-side, and audited.

15.7 Directory self-service

- Self-update layouts define allowed attributes, visibility, editability, required state, validation, and approvals.
- Employee search and organisation chart define searchable/visible attributes, manager relationships, and privacy controls.
- Mail-group subscription defines discoverable groups, join/leave rules, ownership, approval, and history.
- Directory writes are permission-checked, auditable, and idempotent where practical.

15.8 Administrative tools

Quick enrollment:

- Force enrollment using supported logon scripts.
- Notify users by email, SMS, or push.
- Import validated CSV or approved external-database data.
- Preview, dry run, validation report, progress, cancellation, partial failures, and result export.

Endpoint agents:

- Track Windows Credential Provider/GINA, macOS, and Linux installations, versions, and heartbeat.
- Support documented/manual, Group Policy, SCCM, and command-line deployment paths.
- Configure presentation and behavior, deployment schedules, cached-credential updates over VPN, and password-sync agents.
- Extension points for Outlook Web Access, SharePoint, and Citrix access surfaces.

Mobile deployment:

- Target domains, OUs, groups, and users.
- Show user, contact channel, deployment state, last notification, configuration state, retries, and diagnostics.
- Schedule notifications and encrypt APNs/provider credentials.

Approval workflows:

- Independently enable workflows for reset/unlock, directory self-update, and mail-group subscription.
- Associate workflows with policies and configure stages, approver sources, quorum, escalation, delegation, reminders, timeout, rejection, cancellation, and resubmission.
- Prevent self-approval by default and preserve requester/approver history.

Technicians:

- Manage technician identities, roles, domain/OU delegation, report access, and allowed actions.
- Supply least-privilege built-in roles and custom roles; log delegated or impersonated actions.

15.9 Security centre

Security hardening calculates an explainable score and remediation status for HTTPS, secure TLS versions, secure cipher suites, encrypted keystore password, changed default administrator password, LDAPS validation, employee-search privacy, mail TLS, mobile certificate pinning, encrypted backups, endpoint certificate validation, and security-update registration.

Password strengtheners support strength scoring and forced change at next logon. Security Q&A strengtheners prevent answer reuse and question words in answers, optionally display one question at a time or a random subset, optionally make answers case-sensitive, and hide entered answers.

Anti-hacking controls include per-user/per-IP/global rate limits, brute-force and dictionary protection, temporary blocking, idle/absolute session timeout, CAPTCHA extension points, self-service action alerts, inactive-account restrictions, incident review, unblock workflows, and audit history.

15.10 Reports catalogue

All reports support applicable domain, OU/group, time, status, actor, source/IP, access mode, and policy filters; server-side generation; pagination; scheduling; exports; saved views; and permission checks.

User reports:

- Locked Out Users Report.
- Soon-To-Expire Password Users Report.
- Password Expired Users Report.

Password self-service reports:

- Reset Password Audit Report.
- Unlock Account Audit Report.
- Endpoint-agent Reset Password Audit Report.
- Endpoint-agent Unlock Account Audit Report.
- Self-Update Audit Report.
- Change Password Audit Report.
- Password/Account Expiry Notifications Delivery Report.

MFA reports:

- MFA Enrolled Users Report and MFA Non-Enrolled Users Report.
- MFA Enrollment Audit Report and MFA Usage Audit Report.
- MFA Usage for Machines/VPN/OWA and from Mobile Devices.
- MFA Failures, Trusted Browser, and Trusted Machines reports.
- Security Questions and Backup Code Usage reports.
- Offline MFA Enrolled Machines, FIDO2 Passkeys, and Blocked Users reports.

Endpoint-agent reports:

- Agent Installed Machines Report.
- Agent Installation Failures Report.

Other reports:

- User Attempts Audit Report.
- Licensed Users Report.
- Notification Delivery Report.
- Push Registered Devices Report.
- Application Access Audit Report.
- JIT Provisioning Audit Report.

Common actions are generate, cancel long generation, schedule, export authorized columns, choose page size, and save a filtered view. Scheduled/exported files can be password-protected. Generation, export, and delivery are audited.

Required schemas include:

- Password actions: user, time, source host, IP, attempt count, action type, policy, status, and failure category.
- MFA enrollment: account/display name, contact channels, OU, enrollment/modified time, status, authenticators, and backup-code state.
- MFA usage: user, attempted time/from, IP, action, authenticator, access mode, policy, and status.
- Agent machines: name, OS, IP, directory location, version, last contact, and MFA status.

15.11 Administration settings

Logon settings:

- General, SSO, and smart-card tabs.
- Hide administrator login, restrict it by network, configure directory selectors, and allow approved unique login attributes.
- Versioned user disclaimer, username-enumeration defense, CAPTCHA scopes with accessible audio, and visibility of help/mobile links.
- Sanitized portal customization with a recovery path.

Personalisation:

- Display, portal/mobile rebranding, administrator password change, and language customization.
- Locale, welcome-name attribute, time/date format, timezone, and session-expiry settings.

Enterprise essentials:

- Failover/secure-gateway topology with load balancing and primary/secondary health.
- Reverse proxy protocol, port, context, access URL, target URL, and validation.

Product settings:

- Connection: HTTP/HTTPS ports, public URL, certificate rotation, keystore protection, LDAPS, proxy, advanced settings, and restart-required state.
- Mail/SMS: SMTP or provider API, sender/admin addresses, test delivery, TLS, basic/OAuth authentication, SMS adapters, contact mappings, and diagnostics.
- Dashboard updater: scheduler name, directories, frequency, next/last run, duration, status, result, and manual run. Jobs include AD/Entra synchronization, locked-out users, expired passwords, and soon-to-expire passwords.
- Backup: daily/weekly/monthly schedules, timezone, local/network destination, retention, encryption, backup-now, history, integrity status, and restore documentation.
- Site-based DC updater: directory/OU tree, preferred and fallback controllers, connectivity test, and site-versus-policy priority.
- Integrations: adapter-based configuration for ADManager Plus, ServiceDesk Plus, Splunk, Syslog, breached-password services, Log360, and ServiceDesk Plus Cloud. Each has encrypted credentials, test, timeout/retry, health, and audit.
- Windows service: installation/status guidance and operational health without unauthorized remote service control.
- Security/privacy: password protection for reports/backups, explicit usage-statistics consent, retention, export, deletion, and redaction policies.

License management:

- Entitlement and consumption summary by capability.
- Restrict/derestrict users individually or in bulk, with domain, reason, preview, confirmation, schedule, and audit.
- Explain that restriction removes enrollment data, frees Endpoint MFA entitlement, and blocks Endpoint MFA; execute atomically.

15.12 Support workspace

Show application version/build, documentation, issue-reporting, community and knowledge-base links, edition information, and configured support contacts. Isolate external-feed failures. Quote, pricing, live chat, and other marketing links are optional and outside core acceptance.

15.13 Additional domain model

The audited scope adds:

- policy_versions, policy_targets, policy_exclusions, and policy_permissions.
- authenticator_configs, authenticator_enrollments, backup_codes, trusted_browsers, trusted_machines, and blocked_users.
- conditional_access_rules, conditions, actions, and evaluations.
- directory_attributes, self_update_layouts, directory_write_requests, and directory_controller_preferences.
- endpoint_agents, agent_deployments, agent_heartbeats, and cached_credential_jobs.
- report_definitions, report_runs, report_schedules, report_exports, and saved_report_views.
- scheduler_definitions/runs, notification_channels/attempts, certificates, secret_references, backup_jobs/artifacts, and integration_health_checks.
- license_entitlements, license_assignments, restricted_users/schedules, security_controls/evaluations/scores, and security_incidents.

Use encrypted secret references where appropriate. Audit storage is append-only, tamper-evident, tenant-scoped, and governed by documented retention.

15.14 Delivery priorities

Phase 1 — operational core: administrator shell, directory connection, dashboard, policies, user search, reset/unlock, audit events, and user reports, with sessions, RBAC, encrypted credentials, and jobs.

Phase 2 — self-service and MFA: employee portal, self-update, enrollment, TOTP, passkeys, recovery codes, password policy, notifications, and MFA reports.

Phase 3 — delegated administration: workflows, technicians, groups, directory layouts, agent/mobile tracking, report scheduling, and exports.

Phase 4 — enterprise access and resilience: SAML/OIDC, conditional access, provider adapters, integrations, failover, proxy, backup/restore, licensing, and full security posture management.

Each phase is accepted only when APIs, migrations, authorization, audit events, jobs, UI states, and automated tests are complete. A control backed only by mock success is not implemented.
