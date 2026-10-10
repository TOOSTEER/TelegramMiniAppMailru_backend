package storage

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

func TestUsersUnlink(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUsers(pool)
	tgID, vkID := uniqueIDs(t, pool)

	if err := users.LinkVKID(ctx, link(tgID, vkID, "", "at-1")); err != nil {
		t.Fatal(err)
	}

	accessToken, err := users.UnlinkVKID(ctx, tgID)
	if err != nil || accessToken != "at-1" {
		t.Fatalf("UnlinkVKID() = %q, %v; want at-1, nil", accessToken, err)
	}
	if _, err := users.FindByTelegramID(ctx, tgID); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("после отвязки FindByTelegramID(): err = %v, want ErrUserNotFound", err)
	}

	var sessions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM vkid_sessions WHERE access_token = 'at-1'`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Error("сессия VK ID осталась после отвязки")
	}

	if _, err := users.UnlinkVKID(ctx, tgID); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("повторная отвязка: err = %v, want ErrUserNotFound", err)
	}

	if err := users.LinkVKID(ctx, link(tgID-1, vkID, "", "at-2")); err != nil {
		t.Errorf("после отвязки аккаунт VK ID должен быть свободен для другого Telegram: %v", err)
	}
}

func expireSession(t *testing.T, pool *pgxpool.Pool, telegramID int64) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		UPDATE vkid_sessions SET expires_at = now() - interval '1 minute'
		WHERE user_id = (SELECT id FROM users WHERE telegram_id = $1)`, telegramID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidAccessTokenFresh(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUsers(pool)
	tgID, vkID := uniqueIDs(t, pool)
	if err := users.LinkVKID(ctx, link(tgID, vkID, "", "at-1")); err != nil {
		t.Fatal(err)
	}

	token, err := users.ValidAccessToken(ctx, tgID, time.Minute, func(context.Context, string, string) (TokenUpdate, error) {
		t.Error("refresh не должен вызываться для свежего токена")
		return TokenUpdate{}, nil
	})
	if err != nil || token != "at-1" {
		t.Errorf("ValidAccessToken() = %q, %v; want at-1", token, err)
	}

	if _, err := users.ValidAccessToken(ctx, tgID-1, time.Minute, nil); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("для непривязанного: err = %v, want ErrUserNotFound", err)
	}
}

func TestValidAccessTokenRefreshes(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUsers(pool)
	tgID, vkID := uniqueIDs(t, pool)
	if err := users.LinkVKID(ctx, link(tgID, vkID, "", "at-1")); err != nil {
		t.Fatal(err)
	}
	expireSession(t, pool, tgID)

	token, err := users.ValidAccessToken(ctx, tgID, time.Minute, func(_ context.Context, rt, dev string) (TokenUpdate, error) {
		if rt != "rt-at-1" || dev != "device" {
			t.Errorf("refresh(%q, %q), want rt-at-1, device", rt, dev)
		}
		return TokenUpdate{AccessToken: "at-2", RefreshToken: "rt-2", ExpiresIn: time.Hour}, nil
	})
	if err != nil || token != "at-2" {
		t.Fatalf("ValidAccessToken() = %q, %v; want at-2", token, err)
	}

	var accessToken, refreshToken, scope string
	var ttl time.Duration
	err = pool.QueryRow(ctx, `
		SELECT s.access_token, s.refresh_token, s.scope, s.expires_at - now()
		FROM vkid_sessions s JOIN users u ON u.id = s.user_id WHERE u.telegram_id = $1`, tgID).
		Scan(&accessToken, &refreshToken, &scope, &ttl)
	if err != nil {
		t.Fatal(err)
	}
	if accessToken != "at-2" || refreshToken != "rt-2" || scope != "vkid.personal_info email" || ttl < 59*time.Minute {
		t.Errorf("после обновления: %q %q %q %v", accessToken, refreshToken, scope, ttl)
	}
}

func TestValidAccessTokenRefreshErrorKeepsOldTokens(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUsers(pool)
	tgID, vkID := uniqueIDs(t, pool)
	if err := users.LinkVKID(ctx, link(tgID, vkID, "", "at-1")); err != nil {
		t.Fatal(err)
	}
	expireSession(t, pool, tgID)

	wantErr := errors.New("VK ID недоступен")
	_, err := users.ValidAccessToken(ctx, tgID, time.Minute, func(context.Context, string, string) (TokenUpdate, error) {
		return TokenUpdate{}, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}

	var refreshToken string
	if err := pool.QueryRow(ctx, `
		SELECT s.refresh_token FROM vkid_sessions s JOIN users u ON u.id = s.user_id WHERE u.telegram_id = $1`, tgID).
		Scan(&refreshToken); err != nil {
		t.Fatal(err)
	}
	if refreshToken != "rt-at-1" {
		t.Errorf("refresh_token = %q, want прежний rt-at-1", refreshToken)
	}
}

func TestValidAccessTokenConcurrentRefreshOnce(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUsers(pool)
	tgID, vkID := uniqueIDs(t, pool)
	if err := users.LinkVKID(ctx, link(tgID, vkID, "", "at-1")); err != nil {
		t.Fatal(err)
	}
	expireSession(t, pool, tgID)

	var (
		calls atomic.Int32
		wg    sync.WaitGroup
	)
	refresh := func(_ context.Context, rt, _ string) (TokenUpdate, error) {
		if calls.Add(1) > 1 {
			t.Errorf("повторное обновление со старым refresh_token %q убило бы сессию в VK ID", rt)
		}
		time.Sleep(200 * time.Millisecond)
		return TokenUpdate{AccessToken: "at-2", RefreshToken: "rt-2", ExpiresIn: time.Hour}, nil
	}

	const n = 5
	tokens := make([]string, n)
	errs := make([]error, n)
	for i := range n {
		wg.Go(func() {
			tokens[i], errs[i] = users.ValidAccessToken(ctx, tgID, time.Minute, refresh)
		})
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("обновлений = %d, want ровно 1", got)
	}
	for i := range n {
		if errs[i] != nil || tokens[i] != "at-2" {
			t.Errorf("запрос %d: %q, %v; want at-2", i, tokens[i], errs[i])
		}
	}
}
