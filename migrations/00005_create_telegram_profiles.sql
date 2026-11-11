-- +goose Up
CREATE TABLE telegram_profiles (
    telegram_id        BIGINT      PRIMARY KEY,
    first_name         TEXT        NOT NULL,
    last_name          TEXT,
    username           TEXT,
    language_code      TEXT,
    photo_url          TEXT,
    is_premium         BOOLEAN     NOT NULL DEFAULT false,
    allows_write_to_pm BOOLEAN     NOT NULL DEFAULT false,
    phone_number       TEXT,
    phone_shared_at    TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE telegram_profiles;