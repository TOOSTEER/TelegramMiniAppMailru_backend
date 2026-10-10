package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/authstate"
)

type OAuthStates struct {
	pool *pgxpool.Pool
	ttl  time.Duration
}

func NewOAuthStates(pool *pgxpool.Pool, ttl time.Duration) *OAuthStates {
	return &OAuthStates{pool: pool, ttl: ttl}
}

func (s *OAuthStates) Save(ctx context.Context, state string, entry authstate.Entry) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO oauth_states (state_hash, code_verifier, telegram_id, expires_at)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4))`,
		hashState(state), entry.Verifier, entry.TelegramID, s.ttl.Seconds(),
	)
	if err != nil {
		return fmt.Errorf("сохранение state: %w", err)
	}
	return nil
}

func (s *OAuthStates) Pop(ctx context.Context, state string) (authstate.Entry, error) {
	var (
		entry authstate.Entry
		valid bool
	)

	err := s.pool.QueryRow(ctx, `
		DELETE FROM oauth_states
		WHERE state_hash = $1
		RETURNING code_verifier, telegram_id, expires_at > now()`,
		hashState(state),
	).Scan(&entry.Verifier, &entry.TelegramID, &valid)

	if errors.Is(err, pgx.ErrNoRows) {
		return authstate.Entry{}, authstate.ErrNotFound
	}
	if err != nil {
		return authstate.Entry{}, fmt.Errorf("чтение state: %w", err)
	}
	if !valid {
		return authstate.Entry{}, authstate.ErrNotFound
	}

	return entry, nil
}

func hashState(state string) []byte {
	sum := sha256.Sum256([]byte(state))
	return sum[:]
}
