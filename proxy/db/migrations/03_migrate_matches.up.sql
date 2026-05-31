CREATE TABLE matches (
	match_id SERIAL PRIMARY KEY,
	user_id INTEGER REFERENCES sb_history(history_id),
	user_win BOOLEAN,
	match_date TIMESTAMPTZ
);