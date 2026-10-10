-- +goose Up
CREATE TABLE oauth_states (
    state_hash    BYTEA       PRIMARY KEY,
    code_verifier TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX oauth_states_expires_at_idx ON oauth_states (expires_at);

-- +goose Down
DROP TABLE oauth_states;
