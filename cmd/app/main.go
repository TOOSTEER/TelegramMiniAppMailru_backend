package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/config"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/httpapi"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/logger"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/storage"
	"github.com/TOOSTEER/TelegramMiniAppMailru_backend/internal/vkid"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "чтение .env:", err)
		os.Exit(1)
	}

	cfg, cfgErr := config.Load()

	log, err := logger.New(cfg.Env == config.EnvProduction, cfg.LogLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "создание логгера:", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	if cfgErr != nil {
		log.Fatal("некорректная конфигурация", zap.Error(cfgErr))
	}

	if err := run(cfg, log); err != nil {
		log.Fatal("приложение завершилось с ошибкой", zap.Error(err))
	}
}

func run(cfg config.Config, log *zap.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := storage.NewPool(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := storage.Migrate(ctx, pool, log.Named("migrations")); err != nil {
		return err
	}

	if cfg.Env == config.EnvProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	router, err := httpapi.NewRouter(cfg, httpapi.Deps{
		Log:    log,
		DB:     pool,
		VKID:   vkid.New(cfg.VKID),
		States: storage.NewOAuthStates(pool, cfg.VKID.StateTTL),
	})
	if err != nil {
		return err
	}

	httpErrorLog, err := zap.NewStdLogAt(log.Named("http"), zapcore.ErrorLevel)
	if err != nil {
		return fmt.Errorf("логгер для http.Server: %w", err)
	}

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ErrorLog:          httpErrorLog,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("HTTP-сервер запущен", zap.String("addr", cfg.HTTP.Addr), zap.String("env", cfg.Env))
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("HTTP-сервер: %w", err)
	case <-ctx.Done():
	}

	log.Info("получен сигнал остановки, завершаем работу")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("остановка HTTP-сервера: %w", err)
	}

	log.Info("HTTP-сервер остановлен")
	return nil
}
