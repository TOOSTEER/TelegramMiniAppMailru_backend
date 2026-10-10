package vkidsession

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/storage"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkid"
)

type fakeStore struct {
	linked   bool
	fresh    bool
	token    string
	saved    storage.TokenUpdate
	unlinked bool
}

func (f *fakeStore) ValidAccessToken(ctx context.Context, _ int64, _ time.Duration, refresh storage.RefreshFunc) (string, error) {
	if !f.linked {
		return "", storage.ErrUserNotFound
	}
	if f.fresh {
		return f.token, nil
	}
	upd, err := refresh(ctx, "old-rt", "dev")
	if err != nil {
		return "", err
	}
	f.saved = upd
	return upd.AccessToken, nil
}

func (f *fakeStore) UnlinkVKID(context.Context, int64) (string, error) {
	f.unlinked = true
	return f.token, nil
}

type fakeRefresher struct {
	tokens vkid.Tokens
	err    error
	calls  int
}

func (f *fakeRefresher) Refresh(context.Context, string, string) (vkid.Tokens, error) {
	f.calls++
	return f.tokens, f.err
}

func TestAccessToken(t *testing.T) {
	newTokens := vkid.Tokens{AccessToken: "new-at", RefreshToken: "new-rt", ExpiresIn: time.Hour, Scope: "email"}

	tests := map[string]struct {
		store        *fakeStore
		vk           *fakeRefresher
		wantToken    string
		wantErr      error
		wantCalls    int
		wantUnlinked bool
		wantLog      string
	}{
		"свежий токен": {
			store:     &fakeStore{linked: true, fresh: true, token: "at"},
			vk:        &fakeRefresher{},
			wantToken: "at",
		},
		"истёк — обновляем": {
			store:     &fakeStore{linked: true},
			vk:        &fakeRefresher{tokens: newTokens},
			wantToken: "new-at",
			wantCalls: 1,
			wantLog:   "токены VK ID обновлены",
		},
		"не привязан": {
			store:   &fakeStore{},
			vk:      &fakeRefresher{},
			wantErr: ErrNotLinked,
		},
		"refresh token отклонён": {
			store:        &fakeStore{linked: true},
			vk:           &fakeRefresher{err: &vkid.APIError{Code: "invalid_grant"}},
			wantErr:      ErrRelinkRequired,
			wantCalls:    1,
			wantUnlinked: true,
			wantLog:      "refresh token отклонён VK ID, привязка удалена",
		},
		"VK ID недоступен": {
			store:     &fakeStore{linked: true},
			vk:        &fakeRefresher{err: errors.New("timeout")},
			wantCalls: 1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			core, logs := observer.New(zap.InfoLevel)
			s := New(tt.store, tt.vk, time.Minute, zap.New(core))

			token, err := s.AccessToken(context.Background(), 42)

			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if name == "VK ID недоступен" && (err == nil || errors.Is(err, ErrRelinkRequired)) {
				t.Errorf("временный сбой не должен приводить к отвязке, err = %v", err)
			}
			if token != tt.wantToken {
				t.Errorf("token = %q, want %q", token, tt.wantToken)
			}
			if tt.vk.calls != tt.wantCalls {
				t.Errorf("обращений к VK ID = %d, want %d", tt.vk.calls, tt.wantCalls)
			}
			if tt.store.unlinked != tt.wantUnlinked {
				t.Errorf("отвязка = %v, want %v", tt.store.unlinked, tt.wantUnlinked)
			}
			if tt.wantLog != "" && logs.FilterMessage(tt.wantLog).Len() != 1 {
				t.Errorf("в логе нет %q", tt.wantLog)
			}
		})
	}
}
