-- up
create table plugins
(
    id         uuid      not null default gen_random_uuid(),
    group_name text      not null,
    name       text      not null,
    version    text      not null,
    config     jsonb     not null default '{}',
    created_at timestamp not null default now(),

    unique (group_name, name, version),
    primary key (id)
);

-- down
drop table plugins;
