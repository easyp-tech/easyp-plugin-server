-- +goose Up
CREATE TABLE plugins
(
    id         UUID      NOT NULL DEFAULT gen_random_uuid(),
    group_name TEXT      NOT NULL,
    name       TEXT      NOT NULL,
    version    TEXT      NOT NULL,
    config     JSONB     NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    tags       TEXT[]    NOT NULL DEFAULT '{}',

    UNIQUE (group_name, name, version),
    PRIMARY KEY (id)
);

CREATE INDEX idx_plugins_tags ON plugins USING gin (tags);

CREATE TABLE audit_log
(
    id             UUID        NOT NULL DEFAULT gen_random_uuid(),
    operation_type TEXT        NOT NULL,
    plugin_name    TEXT,
    caller_address TEXT        NOT NULL,
    status         TEXT        NOT NULL,
    error_code     TEXT,
    error_message  TEXT,
    duration_ms    BIGINT      NOT NULL,
    metadata       JSONB       NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log (created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_operation_type ON audit_log (operation_type);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS plugins;
