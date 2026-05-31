CREATE TABLE states (
	states_id SERIAL PRIMARY KEY,
	user_key TEXT NOT NULL,
	bot_state BOOLEAN,
	direction_x INT DEFAULT 0,
	direction_y INT DEFAULT -1,
	last_shot_x INT DEFAULT -1,
	last_shot_y INT DEFAULT -1,
	last_hit_x INT DEFAULT -1,
	last_hit_y INT DEFAULT -1
)