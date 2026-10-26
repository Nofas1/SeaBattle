ALTER TABLE users
    ADD COLUMN provider TEXT NOT NULL DEFAULT 'local',
    ADD COLUMN provider_id TEXT,
    ADD CONSTRAINT uq_users_provider_provider_id UNIQUE (provider, provider_id);

ALTER TABLE users
    ALTER COLUMN password DROP NOT NULL;
