-- up
create table plugins
(
    id         uuid      not null default gen_random_uuid(),
    group_name text      not null,
    name       text      not null,
    version    text      not null,
    created_at timestamp not null default now(),

    unique (group_name, name, version),
    primary key (id)
);

insert into plugins (id, group_name, name, version, created_at)
values (gen_random_uuid(), 'protobuf', 'go', 'v1.36.10', now()),
       (gen_random_uuid(), 'grpc', 'go', 'v1.5.1', now()),
       (gen_random_uuid(), 'community', 'pseudomuto-doc', 'v1.5.1', now());

-- down
drop table plugins;
