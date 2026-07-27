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

CREATE TABLE resources (
    id          UUID PRIMARY KEY,
    app_id      UUID NOT NULL REFERENCES apps (id),
    name        TEXT NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX resources_active_idx ON resources (id) WHERE deleted_at IS NULL;
CREATE INDEX resources_app_id_idx ON resources (app_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX resources_app_name_active_uq
    ON resources (app_id, name)
    WHERE deleted_at IS NULL;

CREATE TABLE actions (
    id          UUID PRIMARY KEY,
    app_id      UUID NOT NULL REFERENCES apps (id),
    name        TEXT NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX actions_active_idx ON actions (id) WHERE deleted_at IS NULL;
CREATE INDEX actions_app_id_idx ON actions (app_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX actions_app_name_active_uq
    ON actions (app_id, name)
    WHERE deleted_at IS NULL;

CREATE TABLE permissions (
    id           UUID PRIMARY KEY,
    role_id      UUID NOT NULL REFERENCES roles (id),
    resource_id  UUID NOT NULL REFERENCES resources (id),
    action_id    UUID NOT NULL REFERENCES actions (id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE UNIQUE INDEX permissions_active_uq
    ON permissions (role_id, resource_id, action_id)
    WHERE deleted_at IS NULL;

CREATE INDEX permissions_role_id_idx
    ON permissions (role_id)
    WHERE deleted_at IS NULL;

CREATE INDEX permissions_resource_id_idx
    ON permissions (resource_id)
    WHERE deleted_at IS NULL;

CREATE INDEX permissions_action_id_idx
    ON permissions (action_id)
    WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION check_permission_app_consistency()
RETURNS TRIGGER AS $$
DECLARE
    role_app_id UUID;
    resource_app_id UUID;
    action_app_id UUID;
BEGIN
    SELECT app_id INTO role_app_id FROM roles WHERE id = NEW.role_id;
    SELECT app_id INTO resource_app_id FROM resources WHERE id = NEW.resource_id;
    SELECT app_id INTO action_app_id FROM actions WHERE id = NEW.action_id;

    IF role_app_id IS DISTINCT FROM resource_app_id
       OR role_app_id IS DISTINCT FROM action_app_id THEN
        RAISE EXCEPTION 'role, resource, and action must belong to the same app';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_permission_app_consistency
    BEFORE INSERT OR UPDATE ON permissions
    FOR EACH ROW EXECUTE FUNCTION check_permission_app_consistency();

