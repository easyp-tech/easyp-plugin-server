-- up
create table data
(
    id   uuid not null,
    data text not null,
    primary key (id)
);

-- down
drop table data;
