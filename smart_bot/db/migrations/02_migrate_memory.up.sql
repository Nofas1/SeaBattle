CREATE TABLE memory (
	memory_id SERIAL PRIMARY KEY,
	states_id INT NOT NULL,
	coord_x INT,
	coord_y INT,
	FOREIGN KEY states_id REFERENCES states(states_id)
)
