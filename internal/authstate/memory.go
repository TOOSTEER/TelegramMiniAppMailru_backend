package authstate

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("state не найден, уже использован или просрочен")

type Entry struct {
	Verifier   string
	TelegramID int64
}

type item struct {
	entry     Entry
	expiresAt time.Time
}

type Memory struct {
	ttl   time.Duration
	now   func() time.Time
	mu    sync.Mutex
	items map[string]item
}

func NewMemory(ttl time.Duration) *Memory {
	return &Memory{ttl: ttl, now: time.Now, items: make(map[string]item)}
}

func (m *Memory) Save(_ context.Context, state string, entry Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	for k, it := range m.items {
		if now.After(it.expiresAt) {
			delete(m.items, k)
		}
	}

	m.items[state] = item{entry: entry, expiresAt: now.Add(m.ttl)}
	return nil
}

func (m *Memory) Pop(_ context.Context, state string) (Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	it, ok := m.items[state]
	delete(m.items, state)

	if !ok || m.now().After(it.expiresAt) {
		return Entry{}, ErrNotFound
	}
	return it.entry, nil
}
