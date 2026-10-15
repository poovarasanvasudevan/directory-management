CREATE TABLE configuration_profile_revisions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    tenant_id BIGINT UNSIGNED NOT NULL,
    module_key VARCHAR(64) NOT NULL,
    version BIGINT UNSIGNED NOT NULL,
    values_json JSON NOT NULL,
    modified_by BIGINT UNSIGNED NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_configuration_revision (tenant_id, module_key, version),
    KEY idx_configuration_revision_lookup (tenant_id, module_key, created_at),
    CONSTRAINT fk_configuration_revision_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    CONSTRAINT fk_configuration_revision_modifier FOREIGN KEY (modified_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
