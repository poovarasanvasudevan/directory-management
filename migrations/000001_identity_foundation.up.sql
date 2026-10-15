CREATE TABLE tenants (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(160) NOT NULL,
  slug VARCHAR(100) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id), UNIQUE KEY uq_tenants_slug (slug)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id BIGINT UNSIGNED NOT NULL,
  subject VARCHAR(191) NOT NULL, email VARCHAR(320) NOT NULL, display_name VARCHAR(200) NOT NULL,
  status ENUM('active','disabled','locked') NOT NULL DEFAULT 'active', password_hash VARBINARY(255) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id), UNIQUE KEY uq_users_tenant_subject (tenant_id,subject), UNIQUE KEY uq_users_tenant_email (tenant_id,email),
  CONSTRAINT fk_users_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE roles (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id BIGINT UNSIGNED NOT NULL, name VARCHAR(100) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY(id), UNIQUE KEY uq_roles_tenant_name(tenant_id,name),
  CONSTRAINT fk_roles_tenant FOREIGN KEY(tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE permissions (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, code VARCHAR(120) NOT NULL, PRIMARY KEY(id), UNIQUE KEY uq_permissions_code(code)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE role_permissions (role_id BIGINT UNSIGNED NOT NULL, permission_id BIGINT UNSIGNED NOT NULL, PRIMARY KEY(role_id,permission_id), CONSTRAINT fk_rp_role FOREIGN KEY(role_id) REFERENCES roles(id) ON DELETE CASCADE, CONSTRAINT fk_rp_permission FOREIGN KEY(permission_id) REFERENCES permissions(id) ON DELETE CASCADE) ENGINE=InnoDB;
CREATE TABLE user_roles (user_id BIGINT UNSIGNED NOT NULL, role_id BIGINT UNSIGNED NOT NULL, PRIMARY KEY(user_id,role_id), CONSTRAINT fk_ur_user FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE, CONSTRAINT fk_ur_role FOREIGN KEY(role_id) REFERENCES roles(id) ON DELETE CASCADE) ENGINE=InnoDB;

CREATE TABLE sessions (
  id BINARY(16) NOT NULL, tenant_id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL, token_hash BINARY(32) NOT NULL,
  expires_at DATETIME(6) NOT NULL, revoked_at DATETIME(6) NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id), UNIQUE KEY uq_sessions_token_hash(token_hash), KEY ix_sessions_user_expiry(tenant_id,user_id,expires_at),
  CONSTRAINT fk_sessions_tenant FOREIGN KEY(tenant_id) REFERENCES tenants(id), CONSTRAINT fk_sessions_user FOREIGN KEY(user_id) REFERENCES users(id)
) ENGINE=InnoDB;

CREATE TABLE directories (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id BIGINT UNSIGNED NOT NULL, name VARCHAR(160) NOT NULL,
  provider ENUM('mock','ldap','active_directory') NOT NULL, mode ENUM('read_only','write_enabled') NOT NULL DEFAULT 'read_only',
  config_ciphertext MEDIUMBLOB NULL, enabled BOOLEAN NOT NULL DEFAULT TRUE, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id), UNIQUE KEY uq_directories_tenant_name(tenant_id,name), CONSTRAINT fk_directories_tenant FOREIGN KEY(tenant_id) REFERENCES tenants(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE policies (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id BIGINT UNSIGNED NOT NULL, name VARCHAR(160) NOT NULL,
  priority INT NOT NULL DEFAULT 100, status ENUM('draft','active','disabled') NOT NULL DEFAULT 'draft', version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  rules JSON NOT NULL, created_by BIGINT UNSIGNED NOT NULL, created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id), UNIQUE KEY uq_policies_tenant_name(tenant_id,name), KEY ix_policies_tenant_priority(tenant_id,status,priority),
  CONSTRAINT fk_policies_tenant FOREIGN KEY(tenant_id) REFERENCES tenants(id), CONSTRAINT fk_policies_creator FOREIGN KEY(created_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE audit_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, tenant_id BIGINT UNSIGNED NOT NULL, actor_user_id BIGINT UNSIGNED NULL,
  action VARCHAR(160) NOT NULL, target_type VARCHAR(100) NOT NULL, target_id VARCHAR(191) NULL, outcome ENUM('success','failure') NOT NULL,
  request_id VARCHAR(100) NULL, source_ip VARBINARY(16) NULL, metadata JSON NULL, occurred_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY(id), KEY ix_audit_tenant_time(tenant_id,occurred_at DESC,id DESC), CONSTRAINT fk_audit_tenant FOREIGN KEY(tenant_id) REFERENCES tenants(id), CONSTRAINT fk_audit_actor FOREIGN KEY(actor_user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
