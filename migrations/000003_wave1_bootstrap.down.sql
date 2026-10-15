ALTER TABLE audit_events DROP COLUMN actor;
DELETE FROM users WHERE id = 1 AND tenant_id = 1;
DELETE FROM tenants WHERE id = 1;
