-- +goose Up
CREATE TABLE vkid_sessions (
    user_id       BIGINT      PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    access_token  TEXT        NOT NULL,
    refresh_token TEXT        NOT NULL,
    device_id     TEXT        NOT NULL,
    scope         TEXT        NOT NULL DEFAULT '',
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE vkid_sessions;
