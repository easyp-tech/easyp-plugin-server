package migrations_test

import "github.com/easyp-tech/service/internal/database/migrations"

var (
	path = "./testdata"

	fullMigrations = migrations.Migrations{
		{
			Version: 1,
			Name:    "create_user_table",
			Up:      "create table users(    id         uuid      not null default gen_random_uuid(),    email      text      not null,    name       text      not null,    pass_hash  bytea     not null,    created_at timestamp not null default now(),    updated_at timestamp not null default now(),    unique (email),    unique (name),    primary key (id));",
			Down:    "drop table users;",
		},
		{
			Version: 2,
			Name:    "create_session_table",
			Up:      "create table sessions(    id         uuid      not null,    token      text      not null,    ip         inet      not null,    user_agent text      not null,    user_id    uuid      not null,    created_at timestamp not null default now(),    updated_at timestamp not null default now(),    unique (token),    primary key (id));",
			Down:    "drop table sessions;",
		},
		{
			Version: 3,
			Name:    "create_data_table",
			Up:      "create table data(    id   uuid not null,    data text not null,    primary key (id));",
			Down:    "drop table data;",
		},
		{
			Version: 4,
			Name:    "create_ip_table",
			Up:      "create table ip(    id uuid not null,    ip inet not null,    primary key (id));",
			Down:    "drop table ip;",
		},
	}
)
