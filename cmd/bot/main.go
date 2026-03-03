package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"mvs-kb-bot/internal/config"
	"mvs-kb-bot/internal/db"
	"mvs-kb-bot/internal/telegram"
)

func main() {
	cfg := config.MustLoad()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pg, err := db.New(dbCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer pg.Close()
	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatalf("bot init: %v", err)
	}
	h := telegram.NewHandler(bot, pg, cfg)
	if strings.TrimSpace(cfg.WebhookURL) != "" {
		if err := runWebhook(ctx, bot, h, cfg); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("webhook mode failed: %v", err)
		}
		return
	}
	if err := runPolling(ctx, bot, h); err != nil {
		log.Fatalf("polling mode failed: %v", err)
	}
}

func runWebhook(ctx context.Context, bot *tgbotapi.BotAPI, h *telegram.Handler, cfg config.Config) error {
	wh, err := tgbotapi.NewWebhook(cfg.WebhookURL)
	if err != nil {
		return err
	}
	if _, err := bot.Request(wh); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/tg/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var upd tgbotapi.Update
		if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		h.HandleUpdate(upd)
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("webhook mode: listening on %s", cfg.ListenAddr)
		errCh <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return err
	}
}

func runPolling(ctx context.Context, bot *tgbotapi.BotAPI, h *telegram.Handler) error {
	if _, err := bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false}); err != nil {
		return err
	}
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := bot.GetUpdatesChan(u)
	log.Print("polling mode: bot started")
	for {
		select {
		case <-ctx.Done():
			return nil
		case upd, ok := <-updates:
			if !ok {
				return nil
			}
			h.HandleUpdate(upd)
		}
	}
}
