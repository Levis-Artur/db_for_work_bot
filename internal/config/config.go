package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	BotToken      string
	WebhookURL    string
	WebhookSecret string
	ListenAddr    string
	DatabaseURL   string
	AccessCode    string
	AdminUserID   int64
}

func MustLoad() Config {
	c := Config{
		BotToken:      strings.TrimSpace(os.Getenv("BOT_TOKEN")),
		WebhookURL:    strings.TrimSpace(os.Getenv("WEBHOOK_URL")),
		WebhookSecret: strings.TrimSpace(os.Getenv("WEBHOOK_SECRET")),
		ListenAddr:    strings.TrimSpace(getenv("LISTEN_ADDR", "127.0.0.1:8080")),
		DatabaseURL:   strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AccessCode:    strings.TrimSpace(os.Getenv("ACCESS_CODE")),
	}
	if v := os.Getenv("ADMIN_USER_ID"); v != "" {
		c.AdminUserID = mustParseInt64(v)
	}
	if c.BotToken == "" || c.DatabaseURL == "" {
		log.Fatal("BOT_TOKEN and DATABASE_URL are required")
	}
	return c
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mustParseInt64(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		log.Fatalf("ADMIN_USER_ID parse: %v", err)
	}
	return n
}
