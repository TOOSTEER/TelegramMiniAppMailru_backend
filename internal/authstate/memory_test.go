package authstate

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemory(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	m := NewMemory(10 * time.Minute)
	m.now = func() time.Time { return now }

	_ = m.Save(ctx, "s1", Entry{Verifier: "v1", TelegramID: 1})
	_ = m.Save(ctx, "s2", Entry{Verifier: "v2", TelegramID: 2})

	if e, err := m.Pop(ctx, "s1"); err != nil || e != (Entry{Verifier: "v1", TelegramID: 1}) {
		t.Fatalf("Pop(s1) = %+v, %v; want v1/1, nil", e, err)
	}
	if _, err := m.Pop(ctx, "s1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("повторный Pop(s1): err = %v, want ErrNotFound", err)
	}
	if _, err := m.Pop(ctx, "unknown"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Pop(unknown): err = %v, want ErrNotFound", err)
	}

	now = now.Add(11 * time.Minute)
	if _, err := m.Pop(ctx, "s2"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Pop(s2) после TTL: err = %v, want ErrNotFound", err)
	}
}
