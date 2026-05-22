CREATE TABLE sb_history (
	history_id SERIAL PRIMARY KEY,
	user_id INTEGER REFERENCES users(user_id),
	wins INT DEFAULT 0,
	losses INT DEFAULT 0,
);