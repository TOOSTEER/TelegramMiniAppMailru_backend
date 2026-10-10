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

	_ = m.Save(ctx, "s1", "v1")
	_ = m.Save(ctx, "s2", "v2")

	if v, err := m.Pop(ctx, "s1"); err != nil || v != "v1" {
		t.Fatalf("Pop(s1) = %q, %v; want v1, nil", v, err)
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
