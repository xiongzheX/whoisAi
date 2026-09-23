CREATE TABLE poker_players (
    id text PRIMARY KEY,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE poker_hands (
    id text PRIMARY KEY,
    room_code text NOT NULL,
    hand_number integer NOT NULL CHECK (hand_number > 0),
    ended_at timestamptz NOT NULL,
    result text NOT NULL
);
CREATE TABLE poker_hand_players (
    hand_id text NOT NULL REFERENCES poker_hands(id),
    seat smallint NOT NULL CHECK (seat BETWEEN 0 AND 8),
    player_id text REFERENCES poker_players(id),
    display_name text NOT NULL,
    is_bot boolean NOT NULL,
    start_chips integer NOT NULL CHECK (start_chips > 0),
    end_chips integer NOT NULL CHECK (end_chips >= 0),
    payout integer NOT NULL CHECK (payout >= 0),
    PRIMARY KEY (hand_id, seat),
    UNIQUE (hand_id, player_id),
    CHECK ((is_bot AND player_id IS NULL) OR (NOT is_bot AND player_id IS NOT NULL))
);
CREATE INDEX poker_hand_players_history ON poker_hand_players (player_id, hand_id) WHERE player_id IS NOT NULL;
CREATE INDEX poker_hands_recent ON poker_hands (ended_at DESC, id DESC);
