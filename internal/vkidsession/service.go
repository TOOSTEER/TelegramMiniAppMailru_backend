package vkidsession

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/storage"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkid"
)

var (
	ErrNotLinked      = errors.New("аккаунт VK ID не привязан")
	ErrRelinkRequired = errors.New("сессия VK ID недействительна, аккаунт нужно привязать заново")
)

type store interface {
	ValidAccessToken(ctx context.Context, telegramID int64, minTTL time.Duration, refresh storage.RefreshFunc) (string, error)
	UnlinkVKID(ctx context.Context, telegramID int64) (string, error)
}

type refresher interface {
	Refresh(ctx context.Context, refreshToken, deviceID string) (vkid.Tokens, error)
}

type Service struct {
	store         store
	vk            refresher
	refreshBefore time.Duration
	log           *zap.Logger
}

func New(store store, vk refresher, refreshBefore time.Duration, log *zap.Logger) *Service {
	return &Service{store: store, vk: vk, refreshBefore: refreshBefore, log: log}
}

func (s *Service) AccessToken(ctx context.Context, telegramID int64) (string, error) {
	refreshed := false
	token, err := s.store.ValidAccessToken(ctx, telegramID, s.refreshBefore,
		func(ctx context.Context, refreshToken, deviceID string) (storage.TokenUpdate, error) {
			t, err := s.vk.Refresh(ctx, refreshToken, deviceID)
			if err != nil {
				return storage.TokenUpdate{}, err
			}
			refreshed = true
			return storage.TokenUpdate{
				AccessToken:  t.AccessToken,
				RefreshToken: t.RefreshToken,
				Scope:        t.Scope,
				ExpiresIn:    t.ExpiresIn,
			}, nil
		})

	if errors.Is(err, storage.ErrUserNotFound) {
		return "", ErrNotLinked
	}

	var apiErr *vkid.APIError
	if errors.As(err, &apiErr) && apiErr.Code == "invalid_grant" {
		s.log.Warn("refresh token отклонён VK ID, привязка удалена",
			zap.Int64("telegram_id", telegramID),
			zap.String("description", apiErr.Description),
		)
		if _, err := s.store.UnlinkVKID(ctx, telegramID); err != nil && !errors.Is(err, storage.ErrUserNotFound) {
			return "", fmt.Errorf("удаление недействительной привязки: %w", err)
		}
		return "", ErrRelinkRequired
	}

	if err != nil {
		return "", fmt.Errorf("access token VK ID: %w", err)
	}

	if refreshed {
		s.log.Info("токены VK ID обновлены", zap.Int64("telegram_id", telegramID))
	}
	return token, nil
}
