CREATE TABLE apps (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX apps_active_idx ON apps (id) WHERE deleted_at IS NULL;

CREATE TABLE tenants (
    id          UUID PRIMARY KEY,
    app_id      UUID NOT NULL REFERENCES apps (id),
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX tenants_active_idx ON tenants (id) WHERE deleted_at IS NULL;
CREATE INDEX tenants_app_id_idx ON tenants (app_id) WHERE deleted_at IS NULL;

CREATE TABLE users (
    id          UUID PRIMARY KEY,
    app_id      UUID NOT NULL REFERENCES apps (id),
    email       TEXT NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX users_app_email_active_uq
    ON users (app_id, email)
    WHERE deleted_at IS NULL;

CREATE INDEX users_app_id_idx ON users (app_id) WHERE deleted_at IS NULL;

CREATE TABLE roles (
    id          UUID PRIMARY KEY,
    app_id      UUID NOT NULL REFERENCES apps (id),
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX roles_active_idx ON roles (id) WHERE deleted_at IS NULL;
CREATE INDEX roles_app_id_idx ON roles (app_id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX roles_app_name_active_uq
    ON roles (app_id, (metadata->>'name'))
    WHERE deleted_at IS NULL AND metadata ? 'name';

CREATE TABLE roleassignments (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users (id),
    role_id     UUID NOT NULL REFERENCES roles (id),
    tenant_id   UUID NOT NULL REFERENCES tenants (id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX roleassignments_active_uq
    ON roleassignments (user_id, role_id, tenant_id)
    WHERE deleted_at IS NULL;

CREATE INDEX roleassignments_user_id_idx
    ON roleassignments (user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX roleassignments_tenant_id_idx
    ON roleassignments (tenant_id)
    WHERE deleted_at IS NULL;

CREATE INDEX roleassignments_role_id_idx
    ON roleassignments (role_id)
    WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION check_roleassignment_app_consistency()
RETURNS TRIGGER AS $$
DECLARE
    user_app_id UUID;
    role_app_id UUID;
    tenant_app_id UUID;
BEGIN
    SELECT app_id INTO user_app_id FROM users WHERE id = NEW.user_id;
    SELECT app_id INTO role_app_id FROM roles WHERE id = NEW.role_id;
    SELECT app_id INTO tenant_app_id FROM tenants WHERE id = NEW.tenant_id;

    IF user_app_id IS DISTINCT FROM role_app_id
       OR user_app_id IS DISTINCT FROM tenant_app_id THEN
        RAISE EXCEPTION 'user, role, and tenant must belong to the same app';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_roleassignment_app_consistency
    BEFORE INSERT OR UPDATE ON roleassignments
    FOR EACH ROW EXECUTE FUNCTION check_roleassignment_app_consistency();

CREATE TABLE api_keys (
    id          UUID PRIMARY KEY,
    app_id      UUID NOT NULL REFERENCES apps (id),
    key_hash    TEXT NOT NULL,
    name        TEXT,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);

CREATE INDEX api_keys_app_id_idx ON api_keys (app_id) WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX api_keys_hash_uq ON api_keys (key_hash);

CREATE TABLE admin_api_keys (
    id          UUID PRIMARY KEY,
    key_hash    TEXT NOT NULL,
    name        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX admin_api_keys_hash_uq ON admin_api_keys (key_hash);
