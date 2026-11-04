UPDATE users
SET password = ''
WHERE password IS NULL;

ALTER TABLE users
    ALTER COLUMN password SET NOT NULL;

ALTER TABLE users
    DROP CONSTRAINT uq_users_provider_provider_id,
    DROP COLUMN provider_id,
    DROP COLUMN provider;
