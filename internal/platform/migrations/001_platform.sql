CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE games (
    id text PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    name text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'coming_soon', 'disabled')),
    min_players smallint NOT NULL CHECK (min_players > 0),
    max_players smallint NOT NULL CHECK (max_players >= min_players),
    supports_ai boolean NOT NULL DEFAULT false,
    manifest jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO games (id, slug, name, status, min_players, max_players, supports_ai, manifest)
VALUES
    ('who-is-ai', 'who-is-ai', '谁是 AI', 'active', 5, 8, true,
        '{"route":"/#who-is-ai-room","tags":["社交推理","隐藏身份","AI"]}'::jsonb),
    ('bean-sprint', 'bean-sprint', '豆豆百米赛', 'active', 2, 2, false,
        '{"route":"/games/bean-sprint/","tags":["双人","轻策略","竞速"]}'::jsonb),
    ('dumpling-sumo', 'dumpling-sumo', '团子相扑', 'active', 2, 2, false,
        '{"route":"/games/dumpling-sumo/","tags":["双人","配点","对抗"]}'::jsonb)
ON CONFLICT (id) DO UPDATE SET
    slug = EXCLUDED.slug,
    name = EXCLUDED.name,
    status = EXCLUDED.status,
    min_players = EXCLUDED.min_players,
    max_players = EXCLUDED.max_players,
    supports_ai = EXCLUDED.supports_ai,
    manifest = EXCLUDED.manifest,
    updated_at = now();

CREATE TABLE party_rooms (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code varchar(16) NOT NULL UNIQUE,
    host_member_id uuid,
    selected_game_id text NOT NULL REFERENCES games(id),
    active_session_id uuid,
    status text NOT NULL CHECK (status IN ('open', 'in_game', 'closed')),
    version bigint NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE party_room_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id uuid NOT NULL REFERENCES party_rooms(id) ON DELETE CASCADE,
    user_id uuid,
    display_name varchar(32) NOT NULL,
    seat smallint NOT NULL CHECK (seat >= 0),
    role text NOT NULL CHECK (role IN ('host', 'player')),
    connection_status text NOT NULL CHECK (connection_status IN ('online', 'offline', 'left')),
    ready boolean NOT NULL DEFAULT false,
    resume_token_hash bytea NOT NULL,
    joined_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (room_id, id)
);

-- 离开记录保留作为审计历史，但不应继续占用席位或阻止同一凭证重新入房。
CREATE UNIQUE INDEX party_room_members_active_seat
    ON party_room_members(room_id, seat)
    WHERE connection_status <> 'left';

CREATE UNIQUE INDEX party_room_members_active_resume_token
    ON party_room_members(room_id, resume_token_hash)
    WHERE connection_status <> 'left';

CREATE INDEX party_rooms_waiting_by_game
    ON party_rooms(selected_game_id, updated_at DESC)
    WHERE status = 'open' AND active_session_id IS NULL;

ALTER TABLE party_rooms
    ADD CONSTRAINT party_rooms_host_member_fk
    FOREIGN KEY (id, host_member_id)
    REFERENCES party_room_members(room_id, id)
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE game_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id uuid NOT NULL REFERENCES party_rooms(id) ON DELETE CASCADE,
    game_id text NOT NULL REFERENCES games(id),
    sequence bigint NOT NULL CHECK (sequence > 0),
    status text NOT NULL CHECK (status IN ('created', 'confirming', 'running', 'finished', 'abandoned')),
    mode text NOT NULL,
    settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    state_version bigint NOT NULL DEFAULT 0 CHECK (state_version >= 0),
    started_at timestamptz,
    ended_at timestamptz,
    result_summary jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (room_id, sequence),
    UNIQUE (room_id, id)
);

CREATE UNIQUE INDEX game_sessions_one_active_per_room
    ON game_sessions(room_id)
    WHERE status IN ('created', 'confirming', 'running');

ALTER TABLE party_rooms
    ADD CONSTRAINT party_rooms_active_session_fk
    FOREIGN KEY (id, active_session_id)
    REFERENCES game_sessions(room_id, id)
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE game_session_participants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id uuid NOT NULL,
    session_id uuid NOT NULL,
    room_member_id uuid,
    participant_key text NOT NULL,
    display_name varchar(32) NOT NULL,
    kind text NOT NULL CHECK (kind IN ('human', 'bot')),
    seat smallint NOT NULL CHECK (seat >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        (kind = 'human' AND room_member_id IS NOT NULL)
        OR (kind = 'bot' AND room_member_id IS NULL)
    ),
    UNIQUE (session_id, participant_key),
    UNIQUE (session_id, seat),
    UNIQUE (session_id, room_member_id),
    UNIQUE (session_id, id),
    FOREIGN KEY (room_id, session_id)
        REFERENCES game_sessions(room_id, id)
        ON DELETE CASCADE,
    FOREIGN KEY (room_id, room_member_id)
        REFERENCES party_room_members(room_id, id)
        ON DELETE NO ACTION
        DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE game_snapshots (
    session_id uuid NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    version bigint NOT NULL CHECK (version >= 0),
    public_state jsonb NOT NULL,
    server_time timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, version)
);

CREATE TABLE game_private_snapshots (
    session_id uuid NOT NULL,
    version bigint NOT NULL,
    participant_id uuid NOT NULL,
    private_state jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, version, participant_id),
    FOREIGN KEY (session_id, version)
        REFERENCES game_snapshots(session_id, version)
        ON DELETE CASCADE,
    FOREIGN KEY (session_id, participant_id)
        REFERENCES game_session_participants(session_id, id)
        ON DELETE CASCADE
);

CREATE TABLE game_events (
    id bigserial PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES game_sessions(id) ON DELETE CASCADE,
    version bigint,
    round integer,
    actor_participant_id uuid,
    action_id text,
    event_type text NOT NULL,
    scope text NOT NULL CHECK (scope IN ('public', 'private', 'system')),
    target_participant_id uuid,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (session_id, actor_participant_id)
        REFERENCES game_session_participants(session_id, id),
    FOREIGN KEY (session_id, target_participant_id)
        REFERENCES game_session_participants(session_id, id)
);

CREATE UNIQUE INDEX game_events_action_idempotency
    ON game_events(session_id, actor_participant_id, action_id)
    WHERE action_id IS NOT NULL;

CREATE INDEX game_events_session_order
    ON game_events(session_id, id);

CREATE TABLE party_room_messages (
    id bigserial PRIMARY KEY,
    room_id uuid NOT NULL REFERENCES party_rooms(id) ON DELETE CASCADE,
    session_id uuid,
    sender_member_id uuid,
    message_type text NOT NULL CHECK (message_type IN ('chat', 'system', 'game_result')),
    body jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (room_id, session_id)
        REFERENCES game_sessions(room_id, id)
        ON DELETE CASCADE,
    FOREIGN KEY (room_id, sender_member_id)
        REFERENCES party_room_members(room_id, id)
        ON DELETE NO ACTION
        DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX party_room_messages_timeline
    ON party_room_messages(room_id, id);
