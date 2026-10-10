package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound      = errors.New("пользователь не найден")
	ErrVKIDAlreadyLinked = errors.New("аккаунт VK ID уже привязан к другому пользователю Telegram")
)

const uniqueViolation = "23505"

type VKIDLink struct {
	TelegramID   int64
	VKUserID     int64
	Name         string
	Email        string
	AccessToken  string
	RefreshToken string
	DeviceID     string
	Scope        string
	ExpiresIn    time.Duration
}

type User struct {
	TelegramID int64
	VKUserID   int64
	Name       string
	Email      string
	LinkedAt   time.Time
}

type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{pool: pool}
}

func (s *Users) LinkVKID(ctx context.Context, l VKIDLink) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("начало транзакции: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO users (telegram_id, vk_user_id, email, name)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''))
		ON CONFLICT (telegram_id) DO UPDATE
		SET vk_user_id = EXCLUDED.vk_user_id,
		    email      = EXCLUDED.email,
		    name       = EXCLUDED.name,
		    updated_at = now()
		RETURNING id`,
		l.TelegramID, l.VKUserID, l.Email, l.Name,
	).Scan(&userID)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == "users_vk_user_id_key" {
		return ErrVKIDAlreadyLinked
	}
	if err != nil {
		return fmt.Errorf("сохранение пользователя: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO vkid_sessions (user_id, access_token, refresh_token, device_id, scope, expires_at)
		VALUES ($1, $2, $3, $4, $5, now() + make_interval(secs => $6))
		ON CONFLICT (user_id) DO UPDATE
		SET access_token  = EXCLUDED.access_token,
		    refresh_token = EXCLUDED.refresh_token,
		    device_id     = EXCLUDED.device_id,
		    scope         = EXCLUDED.scope,
		    expires_at    = EXCLUDED.expires_at,
		    updated_at    = now()`,
		userID, l.AccessToken, l.RefreshToken, l.DeviceID, l.Scope, l.ExpiresIn.Seconds(),
	)
	if err != nil {
		return fmt.Errorf("сохранение сессии VK ID: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("фиксация транзакции: %w", err)
	}
	return nil
}

func (s *Users) FindByTelegramID(ctx context.Context, telegramID int64) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT telegram_id, vk_user_id, COALESCE(name, ''), COALESCE(email, ''), updated_at
		FROM users
		WHERE telegram_id = $1`,
		telegramID,
	).Scan(&u.TelegramID, &u.VKUserID, &u.Name, &u.Email, &u.LinkedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("поиск пользователя: %w", err)
	}
	return u, nil
}
