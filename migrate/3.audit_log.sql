-- up
create table audit_log
(
    id              uuid        not null default gen_random_uuid(),
    operation_type  text        not null,
    plugin_name     text,
    caller_address  text        not null,
    status          text        not null,
    error_code      text,
    error_message   text,
    duration_ms     bigint      not null,
    metadata        jsonb       not null default '{}',
    created_at      timestamptz not null default now(),

    primary key (id)
);

create index if not exists idx_audit_log_created_at on audit_log (created_at);
create index if not exists idx_audit_log_operation_type on audit_log (operation_type);

-- down
drop table audit_log;