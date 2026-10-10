package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/migrations"
)

func NewPool(ctx context.Context, cfg config.DB) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("разбор DATABASE_URL: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("подключение к Postgres: %w", err)
	}

	return pool, nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool, log *zap.Logger) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("инициализация миграций: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("применение миграций: %w", err)
	}

	for _, r := range results {
		log.Info("миграция применена",
			zap.String("file", r.Source.Path),
			zap.Duration("duration", r.Duration),
		)
	}

	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("версия схемы: %w", err)
	}
	log.Info("схема БД актуальна", zap.Int64("version", version), zap.Int("applied", len(results)))

	return nil
}
