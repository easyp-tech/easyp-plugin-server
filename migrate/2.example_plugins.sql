-- up
insert into plugins (id, group_name, name, version, config, created_at)
values (gen_random_uuid(), 'protobuf', 'go', 'v1.36.10',
        '{"docker": {"network": "none", "memory": "128m", "cpus": "1.0", "user": "nobody"}}',
        now()),
       (gen_random_uuid(), 'grpc', 'go', 'v1.5.1',
        '{"docker": {"network": "none", "memory": "128m", "cpus": "1.0", "user": "nobody"}}',
        now()),
       (gen_random_uuid(), 'community', 'pseudomuto-doc', 'v1.5.1',
        '{"docker": {"network": "none", "memory": "256m", "cpus": "1.0", "user": "nobody"}}',
        now()),
       (gen_random_uuid(), 'grpc-ecosystem', 'openapiv2', 'v2.27.3',
        '{"docker": {"network": "none", "memory": "128m", "cpus": "1.0", "user": "nobody"}}',
        now()),
       (gen_random_uuid(), 'grpc-ecosystem', 'gateway', 'v2.27.3',
        '{"docker": {"network": "none", "memory": "128m", "cpus": "1.0", "user": "nobody"}}',
        now());

-- down
truncate table  plugins;
