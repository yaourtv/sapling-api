package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/yaourtv/sapling-api/internal/config"
	"github.com/yaourtv/sapling-api/internal/db"
	"github.com/yaourtv/sapling-api/internal/handler"
	"github.com/yaourtv/sapling-api/internal/service"
	"github.com/yaourtv/sapling-api/internal/token"
)

const archiveInterval = 6 * time.Hour

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(pool); err != nil {
		return err
	}
	log.Println("migrations applied")

	tokens := token.NewManager(cfg.AccessTokenSecret, cfg.RefreshTokenSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)

	authSvc := service.NewAuthService(pool, tokens, cfg)
	userSvc := service.NewUserService(pool)
	appSvc := service.NewApplicationService(pool)
	statusSvc := service.NewStatusService(pool)
	calendarSvc := service.NewCalendarService(pool)

	go service.RunAutoArchive(ctx, pool, archiveInterval)

	router := handler.NewRouter(handler.Handlers{
		Auth:        handler.NewAuthHandler(authSvc, cfg.RefreshTokenTTL),
		User:        handler.NewUserHandler(userSvc),
		Application: handler.NewApplicationHandler(appSvc),
		Status:      handler.NewStatusHandler(statusSvc),
		Calendar:    handler.NewCalendarHandler(calendarSvc),
		Public:      handler.NewPublicHandler(calendarSvc),
	}, tokens, cfg.CORSOrigin)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on :%s", cfg.Port)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
