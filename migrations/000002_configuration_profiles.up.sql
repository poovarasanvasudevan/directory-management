CREATE TABLE configuration_profiles (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    tenant_id BIGINT UNSIGNED NOT NULL,
    module_key VARCHAR(64) NOT NULL,
    values_json JSON NOT NULL,
    version BIGINT UNSIGNED NOT NULL DEFAULT 1,
    modified_by BIGINT UNSIGNED NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_configuration_profiles_tenant_module (tenant_id, module_key),
    KEY idx_configuration_profiles_updated (tenant_id, updated_at),
    CONSTRAINT fk_configuration_profiles_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    CONSTRAINT fk_configuration_profiles_modifier FOREIGN KEY (modified_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
