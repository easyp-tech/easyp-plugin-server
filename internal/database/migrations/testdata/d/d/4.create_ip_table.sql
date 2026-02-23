-- up
create table ip
(
    id uuid not null,
    ip inet not null,
    primary key (id)
);

-- down
drop table ip;
