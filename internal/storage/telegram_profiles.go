package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrTelegramProfileNotFound = errors.New("профиль Telegram не найден")

type TelegramProfile struct {
	TelegramID      int64
	FirstName       string
	LastName        string
	Username        string
	LanguageCode    string
	PhotoURL        string
	IsPremium       bool
	AllowsWriteToPM bool
	PhoneNumber     string
	PhoneSharedAt   *time.Time
	UpdatedAt       time.Time
}

type TelegramProfiles struct {
	pool *pgxpool.Pool
}

func NewTelegramProfiles(pool *pgxpool.Pool) *TelegramProfiles {
	return &TelegramProfiles{pool: pool}
}

func (s *TelegramProfiles) Upsert(ctx context.Context, p TelegramProfile) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO telegram_profiles (
			telegram_id, first_name, last_name, username, language_code, photo_url,
			is_premium, allows_write_to_pm
		) VALUES (
			$1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''),
			$7, $8
		)
		ON CONFLICT (telegram_id) DO UPDATE
		SET first_name         = EXCLUDED.first_name,
		    last_name          = EXCLUDED.last_name,
		    username           = EXCLUDED.username,
		    language_code      = EXCLUDED.language_code,
		    photo_url          = EXCLUDED.photo_url,
		    is_premium         = EXCLUDED.is_premium,
		    allows_write_to_pm = EXCLUDED.allows_write_to_pm,
		    updated_at         = now()`,
		p.TelegramID, p.FirstName, p.LastName, p.Username, p.LanguageCode, p.PhotoURL,
		p.IsPremium, p.AllowsWriteToPM,
	)
	if err != nil {
		return fmt.Errorf("сохранение профиля Telegram: %w", err)
	}
	return nil
}

func (s *TelegramProfiles) SavePhone(ctx context.Context, telegramID int64, phone string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE telegram_profiles
		SET phone_number    = $2,
		    phone_shared_at = now(),
		    updated_at      = now()
		WHERE telegram_id = $1`,
		telegramID, phone,
	)
	if err != nil {
		return fmt.Errorf("сохранение телефона: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTelegramProfileNotFound
	}
	return nil
}

func (s *TelegramProfiles) Find(ctx context.Context, telegramID int64) (TelegramProfile, error) {
	var p TelegramProfile
	err := s.pool.QueryRow(ctx, `
		SELECT telegram_id,
		       first_name,
		       COALESCE(last_name, ''),
		       COALESCE(username, ''),
		       COALESCE(language_code, ''),
		       COALESCE(photo_url, ''),
		       is_premium,
		       allows_write_to_pm,
		       COALESCE(phone_number, ''),
		       phone_shared_at,
		       updated_at
		FROM telegram_profiles
		WHERE telegram_id = $1`,
		telegramID,
	).Scan(
		&p.TelegramID, &p.FirstName, &p.LastName, &p.Username, &p.LanguageCode, &p.PhotoURL,
		&p.IsPremium, &p.AllowsWriteToPM, &p.PhoneNumber, &p.PhoneSharedAt, &p.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return TelegramProfile{}, ErrTelegramProfileNotFound
	}
	if err != nil {
		return TelegramProfile{}, fmt.Errorf("поиск профиля Telegram: %w", err)
	}
	return p, nil
}
