-- up
create table users
(
    id         uuid      not null default gen_random_uuid(),
    email      text      not null,
    name       text      not null,
    pass_hash  bytea     not null,
    created_at timestamp not null default now(),
    updated_at timestamp not null default now(),

    unique (email),
    unique (name),

    primary key (id)
);

-- down
drop table users;
