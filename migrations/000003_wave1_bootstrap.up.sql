INSERT INTO tenants (id, name, slug)
VALUES (1, 'Default', 'default')
ON DUPLICATE KEY UPDATE name = VALUES(name);

INSERT INTO users (id, tenant_id, subject, email, display_name, status)
VALUES (1, 1, 'admin', 'admin@localhost', 'Administrator', 'active')
ON DUPLICATE KEY UPDATE display_name = VALUES(display_name);

ALTER TABLE audit_events
  ADD COLUMN actor VARCHAR(160) NULL AFTER actor_user_id;

INSERT INTO policies (tenant_id, name, priority, status, version, rules, created_by)
SELECT 1, 'Self-service users', 10, 'active', 1,
  CAST('{"scope":"All employees","domain":"ADSELFSERVICE","permissions":["Reset Password","Unlock Account","Self Update","Change Password"],"ouDNs":[],"groupDNs":[],"uiStatus":"Active","modifiedBy":"admin"}' AS JSON),
  1
WHERE NOT EXISTS (SELECT 1 FROM policies WHERE tenant_id = 1 AND name = 'Self-service users');
