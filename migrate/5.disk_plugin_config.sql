-- up
UPDATE plugins SET config = jsonb_build_object(
    'command', jsonb_build_array(
        '/plugins/' || group_name || '/' || name || '/' || version || '/plugin'
    )
) WHERE config ? 'docker';

-- down
-- Обратная миграция невозможна без оригинальных Docker-конфигов.
