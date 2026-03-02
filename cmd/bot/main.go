package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"mvs-kb-bot/internal/config"
	"mvs-kb-bot/internal/db"
	"mvs-kb-bot/internal/telegram"
)

func main() {
	cfg := config.MustLoad()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pg, err := db.New(ctx, cfg.DatabaseURL)
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
		runWebhook(bot, h, cfg)
		return
	}
	runPolling(bot, h)
}

func runWebhook(bot *tgbotapi.BotAPI, h *telegram.Handler, cfg config.Config) {
	wh, err := tgbotapi.NewWebhook(cfg.WebhookURL)
	if err != nil {
		log.Fatalf("webhook config: %v", err)
	}
	if _, err := bot.Request(wh); err != nil {
		log.Fatalf("set webhook: %v", err)
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
	log.Printf("webhook mode: listening on %s", cfg.ListenAddr)
	log.Fatal(srv.ListenAndServe())
}

func runPolling(bot *tgbotapi.BotAPI, h *telegram.Handler) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := bot.GetUpdatesChan(u)
	log.Print("polling mode: bot started")
	for upd := range updates {
		h.HandleUpdate(upd)
	}
}
