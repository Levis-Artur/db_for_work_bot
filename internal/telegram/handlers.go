package telegram

import (
	"context"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"db_for_work_bot/internal/config"
	"db_for_work_bot/internal/db"
)

const (
	cbCatPrefix   = "cat:"
	cbArtPrefix   = "art:"
	cbCatPagePref = "catpage:"
	cbHome        = "home"
	opTimeout     = 3 * time.Second
	pageSize      = 10
)

type Handler struct {
	bot *tgbotapi.BotAPI
	db  *db.DB
	cfg config.Config

	adminMu       sync.RWMutex
	adminSessions map[int64]AdminSession
}

func NewHandler(bot *tgbotapi.BotAPI, pg *db.DB, cfg config.Config) *Handler {
	h := &Handler{
		bot:           bot,
		db:            pg,
		cfg:           cfg,
		adminSessions: make(map[int64]AdminSession),
	}
	if cfg.AdminUserID != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		defer cancel()
		if err := pg.EnsureAdmin(ctx, cfg.AdminUserID); err != nil {
			log.Printf("ensure admin failed: %v", err)
		}
	}
	return h
}

func (h *Handler) HandleUpdate(upd tgbotapi.Update) {
	switch {
	case upd.Message != nil:
		h.onMessage(upd.Message)
	case upd.CallbackQuery != nil:
		h.onCallback(upd.CallbackQuery)
	}
}

func (h *Handler) onMessage(m *tgbotapi.Message) {
	if m.From == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	uid := m.From.ID
	if err := h.db.TouchSeen(ctx, uid); err != nil {
		log.Printf("touch seen failed: %v", err)
	}
	text := strings.TrimSpace(m.Text)
	if strings.HasPrefix(text, "/start") {
		h.handleStart(ctx, m, uid, strings.TrimSpace(strings.TrimPrefix(text, "/start")))
		return
	}
	if text == "/admin" {
		h.handleAdminEntry(ctx, m.Chat.ID, uid)
		return
	}
	if text == "/cancel" {
		if h.handleAdminCancel(m.Chat.ID, uid) {
			return
		}
	}
	if text == "/categories" || text == "/help" {
		active, err := h.isAllowed(ctx, uid)
		if err != nil || !active {
			h.requestAccessCode(m.Chat.ID, "Access is restricted. Enter access code:")
			return
		}
		h.clearAdminSession(uid)
		h.showCategories(m.Chat.ID)
		return
	}
	active, err := h.isAllowed(ctx, uid)
	if err != nil {
		h.replyText(m.Chat.ID, "Temporary error. Try again later.")
		return
	}
	if !active {
		if text == "" || strings.HasPrefix(text, "/") {
			h.requestAccessCode(m.Chat.ID, "Enter access code:")
			return
		}
		ok, err := h.db.ActivateByCode(ctx, uid, h.cfg.AccessCode, text)
		if err != nil {
			h.replyText(m.Chat.ID, "Authorization failed. Try again later.")
			return
		}
		if !ok {
			h.requestAccessCode(m.Chat.ID, "Invalid code. Try again:")
			return
		}
		h.replyText(m.Chat.ID, "Access granted.")
		h.showCategories(m.Chat.ID)
		return
	}
	if h.handleAdminMessageInput(ctx, m.Chat.ID, uid, text) {
		return
	}
	h.replyText(m.Chat.ID, "Use /categories.")
}

func (h *Handler) handleStart(ctx context.Context, m *tgbotapi.Message, uid int64, code string) {
	h.clearAdminSession(uid)
	if code == "" {
		active, err := h.isAllowed(ctx, uid)
		if err != nil {
			h.replyText(m.Chat.ID, "Temporary error. Try again later.")
			return
		}
		if !active {
			h.requestAccessCode(m.Chat.ID, "Enter access code:")
			return
		}
		h.showCategories(m.Chat.ID)
		return
	}
	ok, err := h.db.ActivateByCode(ctx, uid, h.cfg.AccessCode, code)
	if err != nil {
		h.replyText(m.Chat.ID, "Authorization failed. Try again later.")
		return
	}
	if !ok {
		h.requestAccessCode(m.Chat.ID, "Invalid code. Try again:")
		return
	}
	h.replyText(m.Chat.ID, "Access granted.")
	h.showCategories(m.Chat.ID)
}

func (h *Handler) requestAccessCode(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.ForceReply{
		ForceReply:            true,
		Selective:             true,
		InputFieldPlaceholder: "Access code",
	}
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("request access code failed: %v", err)
	}
}

func (h *Handler) isAllowed(ctx context.Context, uid int64) (bool, error) {
	active, _, err := h.db.IsActive(ctx, uid)
	if err != nil {
		return false, err
	}
	return active, nil
}

func (h *Handler) showCategories(chatID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	cats, err := h.db.ListCategories(ctx)
	if err != nil {
		log.Printf("list categories failed: %v", err)
		h.replyText(chatID, "Failed to load categories.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, "Categories")
	msg.ReplyMarkup = CategoriesKeyboard(cats)
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("send categories failed: %v", err)
	}
}

func (h *Handler) onCallback(q *tgbotapi.CallbackQuery) {
	if q.Message == nil || q.From == nil {
		h.answerCallback(q.ID, "No chat to reply")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	uid := q.From.ID
	chatID := q.Message.Chat.ID
	data := q.Data
	if strings.HasPrefix(data, cbAdmPrefix) {
		h.onAdminCallback(ctx, q, chatID, uid)
		return
	}
	active, err := h.isAllowed(ctx, uid)
	if err != nil || !active {
		h.answerCallback(q.ID, "No access")
		return
	}
	switch {
	case data == cbHome:
		h.showCategories(chatID)
	case strings.HasPrefix(data, cbCatPrefix):
		catID, err := strconv.ParseInt(strings.TrimPrefix(data, cbCatPrefix), 10, 64)
		if err != nil {
			h.answerCallback(q.ID, "Invalid category")
			return
		}
		h.showArticles(chatID, catID, 0)
	case strings.HasPrefix(data, cbCatPagePref):
		parts := strings.Split(strings.TrimPrefix(data, cbCatPagePref), ":")
		if len(parts) != 2 {
			h.answerCallback(q.ID, "Invalid navigation")
			return
		}
		catID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			h.answerCallback(q.ID, "Invalid category")
			return
		}
		offset, err := strconv.Atoi(parts[1])
		if err != nil || offset < 0 {
			h.answerCallback(q.ID, "Invalid page")
			return
		}
		h.showArticles(chatID, catID, offset)
	case strings.HasPrefix(data, cbArtPrefix):
		artID, err := strconv.ParseInt(strings.TrimPrefix(data, cbArtPrefix), 10, 64)
		if err != nil {
			h.answerCallback(q.ID, "Invalid article")
			return
		}
		h.showArticle(chatID, artID)
	default:
		h.answerCallback(q.ID, "Unknown action")
		return
	}
	h.answerCallback(q.ID, "")
}

func (h *Handler) showArticles(chatID, catID int64, offset int) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	arts, err := h.db.ListArticlesByCategory(ctx, catID, pageSize, offset)
	if err != nil {
		log.Printf("list articles failed: %v", err)
		h.replyText(chatID, "Failed to load articles.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Articles (page %d)", offset/pageSize+1))
	msg.ReplyMarkup = ArticlesKeyboard(catID, offset, pageSize, arts)
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("send articles failed: %v", err)
	}
}

func (h *Handler) showArticle(chatID, artID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	a, err := h.db.GetArticle(ctx, artID)
	if err != nil {
		h.replyText(chatID, "Failed to open article.")
		return
	}
	chunks := splitByLen("<b>"+escapeHTML(a.Title)+"</b><br><br>"+a.Body, 3500)
	for i, ch := range chunks {
		msg := tgbotapi.NewMessage(chatID, ch)
		msg.ParseMode = "HTML"
		if i == len(chunks)-1 {
			msg.ReplyMarkup = BackToCategoriesKeyboard()
		}
		if _, err := h.bot.Send(msg); err != nil {
			log.Printf("send article chunk failed: %v", err)
			return
		}
	}
}

func (h *Handler) replyText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("send text failed: %v", err)
	}
}

func (h *Handler) answerCallback(id, text string) {
	if _, err := h.bot.Request(tgbotapi.NewCallback(id, text)); err != nil {
		log.Printf("answer callback failed: %v", err)
	}
}

func escapeHTML(s string) string { return html.EscapeString(s) }

func splitByLen(s string, maxRunes int) []string {
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return []string{s}
	}
	runes := []rune(s)
	chunks := make([]string, 0, len(runes)/maxRunes+1)
	for len(runes) > maxRunes {
		chunks = append(chunks, string(runes[:maxRunes]))
		runes = runes[maxRunes:]
	}
	if len(runes) > 0 {
		chunks = append(chunks, string(runes))
	}
	return chunks
}
