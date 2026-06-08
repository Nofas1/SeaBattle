CREATE TABLE IF NOT EXISTS states (
    states_id    SERIAL PRIMARY KEY,
    user_key     TEXT NOT NULL,
    bot_state    BOOLEAN NOT NULL DEFAULT FALSE,
    direction_x  INT NOT NULL DEFAULT 0,
    direction_y  INT NOT NULL DEFAULT 0,
    last_shot_x  INT NOT NULL DEFAULT -1,
    last_shot_y  INT NOT NULL DEFAULT -1,
    last_hit_x   INT NOT NULL DEFAULT -1,
    last_hit_y   INT NOT NULL DEFAULT -1
);