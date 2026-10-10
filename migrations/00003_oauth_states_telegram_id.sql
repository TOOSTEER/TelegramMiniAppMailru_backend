-- +goose Up
DELETE FROM oauth_states;
ALTER TABLE oauth_states ADD COLUMN telegram_id BIGINT NOT NULL;

-- +goose Down
ALTER TABLE oauth_states DROP COLUMN telegram_id;
