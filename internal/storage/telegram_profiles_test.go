package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func uniqueTelegramID(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	id := -time.Now().UnixNano() % 1_000_000_000_000

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM telegram_profiles WHERE telegram_id = $1`, id)
	})
	return id
}

func profile(telegramID int64) TelegramProfile {
	return TelegramProfile{
		TelegramID:      telegramID,
		FirstName:       "Иван",
		LastName:        "Хвостов",
		Username:        "ivan",
		LanguageCode:    "ru",
		PhotoURL:        "https://t.me/i/userpic/320/abc.jpg",
		IsPremium:       true,
		AllowsWriteToPM: true,
	}
}

func TestTelegramProfilesUpsertAndFind(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	profiles := NewTelegramProfiles(pool)
	tgID := uniqueTelegramID(t, pool)

	if _, err := profiles.Find(ctx, tgID); !errors.Is(err, ErrTelegramProfileNotFound) {
		t.Fatalf("Find() до Upsert: err = %v, want ErrTelegramProfileNotFound", err)
	}

	if err := profiles.Upsert(ctx, profile(tgID)); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	p, err := profiles.Find(ctx, tgID)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if p.TelegramID != tgID || p.FirstName != "Иван" || p.LastName != "Хвостов" ||
		p.Username != "ivan" || p.LanguageCode != "ru" || p.PhotoURL == "" ||
		!p.IsPremium || !p.AllowsWriteToPM {
		t.Errorf("Find() = %+v", p)
	}
	if p.PhoneNumber != "" || p.PhoneSharedAt != nil {
		t.Errorf("телефон не должен появляться после Upsert: %+v", p)
	}
	if p.UpdatedAt.IsZero() {
		t.Error("updated_at не заполнен")
	}
}

func TestTelegramProfilesUpsertKeepsPhone(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	profiles := NewTelegramProfiles(pool)
	tgID := uniqueTelegramID(t, pool)

	if err := profiles.Upsert(ctx, profile(tgID)); err != nil {
		t.Fatal(err)
	}
	if err := profiles.SavePhone(ctx, tgID, "+79991234567"); err != nil {
		t.Fatal(err)
	}

	p := profile(tgID)
	p.FirstName = "Пётр"
	if err := profiles.Upsert(ctx, p); err != nil {
		t.Fatal(err)
	}

	got, err := profiles.Find(ctx, tgID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FirstName != "Пётр" || got.PhoneNumber != "+79991234567" || got.PhoneSharedAt == nil {
		t.Errorf("после Upsert = %+v", got)
	}
}

func TestTelegramProfilesSavePhone(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	profiles := NewTelegramProfiles(pool)
	tgID := uniqueTelegramID(t, pool)

	if err := profiles.SavePhone(ctx, tgID, "+79991234567"); !errors.Is(err, ErrTelegramProfileNotFound) {
		t.Fatalf("SavePhone() без профиля: err = %v, want ErrTelegramProfileNotFound", err)
	}

	if err := profiles.Upsert(ctx, profile(tgID)); err != nil {
		t.Fatal(err)
	}
	if err := profiles.SavePhone(ctx, tgID, "+79991234567"); err != nil {
		t.Fatal(err)
	}

	got, err := profiles.Find(ctx, tgID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PhoneNumber != "+79991234567" || got.PhoneSharedAt == nil {
		t.Errorf("после SavePhone = %+v", got)
	}
}
