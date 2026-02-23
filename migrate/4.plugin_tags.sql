-- up
alter table plugins add column tags text[] not null default '{}';

create index idx_plugins_tags on plugins using gin (tags);

-- down
drop index if exists idx_plugins_tags;

alter table plugins drop column if exists tags;
