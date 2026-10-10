package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func uniqueIDs(t *testing.T, pool *pgxpool.Pool) (telegramID, vkUserID int64) {
	t.Helper()
	base := time.Now().UnixNano() % 1_000_000_000_000
	telegramID, vkUserID = -base, -base

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM users WHERE telegram_id IN ($1, $2) OR vk_user_id IN ($1, $2)`, telegramID, telegramID-1)
	})
	return telegramID, vkUserID
}

func link(tgID, vkID int64, email, accessToken string) VKIDLink {
	return VKIDLink{
		TelegramID:   tgID,
		VKUserID:     vkID,
		Name:         "Иван Хвостов",
		Email:        email,
		AccessToken:  accessToken,
		RefreshToken: "rt-" + accessToken,
		DeviceID:     "device",
		Scope:        "vkid.personal_info email",
		ExpiresIn:    time.Hour,
	}
}

func TestUsersLinkAndFind(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUsers(pool)
	tgID, vkID := uniqueIDs(t, pool)

	if _, err := users.FindByTelegramID(ctx, tgID); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("FindByTelegramID() до привязки: err = %v, want ErrUserNotFound", err)
	}

	if err := users.LinkVKID(ctx, link(tgID, vkID, "ivan@mail.ru", "at-1")); err != nil {
		t.Fatalf("LinkVKID() error = %v", err)
	}

	u, err := users.FindByTelegramID(ctx, tgID)
	if err != nil {
		t.Fatalf("FindByTelegramID() error = %v", err)
	}
	if u.VKUserID != vkID || u.Name != "Иван Хвостов" || u.Email != "ivan@mail.ru" || u.LinkedAt.IsZero() {
		t.Errorf("FindByTelegramID() = %+v", u)
	}

	var accessToken string
	var ttl time.Duration
	err = pool.QueryRow(ctx, `
		SELECT s.access_token, s.expires_at - now()
		FROM vkid_sessions s JOIN users u ON u.id = s.user_id
		WHERE u.telegram_id = $1`, tgID).Scan(&accessToken, &ttl)
	if err != nil {
		t.Fatal(err)
	}
	if accessToken != "at-1" || ttl < 59*time.Minute || ttl > time.Hour {
		t.Errorf("сессия: access_token = %q, осталось %v; want at-1 и около часа", accessToken, ttl)
	}
}

func TestUsersRelink(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUsers(pool)
	tgID, vkID := uniqueIDs(t, pool)

	if err := users.LinkVKID(ctx, link(tgID, vkID, "old@mail.ru", "at-1")); err != nil {
		t.Fatal(err)
	}
	if err := users.LinkVKID(ctx, link(tgID, vkID-1, "", "at-2")); err != nil {
		t.Fatalf("повторная привязка другого аккаунта: %v", err)
	}

	u, err := users.FindByTelegramID(ctx, tgID)
	if err != nil {
		t.Fatal(err)
	}
	if u.VKUserID != vkID-1 || u.Email != "" {
		t.Errorf("после перепривязки = %+v, want vk_user_id %d без почты", u, vkID-1)
	}

	var rows int
	var accessToken string
	err = pool.QueryRow(ctx, `
		SELECT count(*) OVER (), s.access_token
		FROM vkid_sessions s JOIN users u ON u.id = s.user_id
		WHERE u.telegram_id = $1`, tgID).Scan(&rows, &accessToken)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 1 || accessToken != "at-2" {
		t.Errorf("сессий = %d, access_token = %q; want одна сессия с at-2", rows, accessToken)
	}
}

func TestUsersVKIDAlreadyLinkedToAnotherTelegram(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUsers(pool)
	tgID, vkID := uniqueIDs(t, pool)

	if err := users.LinkVKID(ctx, link(tgID, vkID, "", "at-1")); err != nil {
		t.Fatal(err)
	}

	err := users.LinkVKID(ctx, link(tgID-1, vkID, "", "at-2"))
	if !errors.Is(err, ErrVKIDAlreadyLinked) {
		t.Fatalf("LinkVKID() для другого telegram_id: err = %v, want ErrVKIDAlreadyLinked", err)
	}
	if _, err := users.FindByTelegramID(ctx, tgID-1); !errors.Is(err, ErrUserNotFound) {
		t.Error("второй пользователь Telegram не должен был сохраниться")
	}
}
