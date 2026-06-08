CREATE TABLE IF NOT EXISTS memory (
    memory_id  SERIAL PRIMARY KEY,
    states_id  INT NOT NULL,
    coord_x    INT NOT NULL,
    coord_y    INT NOT NULL,
    FOREIGN KEY (states_id) REFERENCES states(states_id) ON DELETE CASCADE
);