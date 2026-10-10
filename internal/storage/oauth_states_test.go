package storage

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/authstate"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL не задан, интеграционные тесты пропущены")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, config.DB{URL: dsn, ConnectTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	if err := Migrate(ctx, pool, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestOAuthStates(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	s := NewOAuthStates(pool, time.Minute)

	state := "test-" + t.Name() + "-" + time.Now().Format(time.RFC3339Nano)
	if err := s.Save(ctx, state, "the-verifier"); err != nil {
		t.Fatal(err)
	}

	var stored int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM oauth_states WHERE state_hash = $1`, []byte(state)).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 0 {
		t.Error("state хранится в открытом виде, а должен только хэш")
	}

	if v, err := s.Pop(ctx, state); err != nil || v != "the-verifier" {
		t.Fatalf("Pop() = %q, %v; want the-verifier, nil", v, err)
	}
	if _, err := s.Pop(ctx, state); !errors.Is(err, authstate.ErrNotFound) {
		t.Errorf("повторный Pop(): err = %v, want ErrNotFound", err)
	}
	if _, err := s.Pop(ctx, "never-saved"); !errors.Is(err, authstate.ErrNotFound) {
		t.Errorf("Pop(неизвестный): err = %v, want ErrNotFound", err)
	}
}

func TestOAuthStatesExpired(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	s := NewOAuthStates(pool, 10*time.Millisecond)

	state := "test-" + t.Name() + "-" + time.Now().Format(time.RFC3339Nano)
	if err := s.Save(ctx, state, "v"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	if _, err := s.Pop(ctx, state); !errors.Is(err, authstate.ErrNotFound) {
		t.Errorf("Pop() просроченного: err = %v, want ErrNotFound", err)
	}

	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM oauth_states WHERE state_hash = $1`, hashState(state)).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Error("просроченный state остался в таблице после Pop")
	}
}

func TestOAuthStatesConcurrentPop(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	s := NewOAuthStates(pool, time.Minute)

	state := "test-" + t.Name() + "-" + time.Now().Format(time.RFC3339Nano)
	if err := s.Save(ctx, state, "v"); err != nil {
		t.Fatal(err)
	}

	const n = 10
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		success int
	)
	for range n {
		wg.Go(func() {
			if _, err := s.Pop(ctx, state); err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	if success != 1 {
		t.Errorf("успешных Pop = %d из %d, want ровно 1", success, n)
	}
}
