package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"spotreba/server/internal/api"
	"spotreba/server/internal/config"
	"spotreba/server/internal/db"
	"spotreba/server/internal/mail"
	"spotreba/server/internal/maintenance"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: spotreba <serve|migrate|check-reminders>")
	}
	switch args[0] {
	case "migrate":
		return migrate()
	case "serve":
		return serve()
	case "check-reminders":
		return checkReminders()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func checkReminders() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.SMTPAddress == "" {
		return errors.New("SMTP_ADDRESS is required for maintenance reminders")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()
	today, err := time.ParseInLocation(time.DateOnly, time.Now().In(cfg.AppTZ).Format(time.DateOnly), cfg.AppTZ)
	if err != nil {
		return err
	}
	sender := &mail.SMTP{Address: cfg.SMTPAddress, Port: cfg.SMTPPort, Username: cfg.SMTPUser, Password: cfg.SMTPPass, From: cfg.MailerFrom}
	_, err = maintenance.CheckAndDeliver(ctx, database, today, sender)
	return err
}

func migrate() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	database, err := db.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()
	return db.Migrate(ctx, database)
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()
	handler, err := api.New(database, cfg)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownContext)
	}
}
