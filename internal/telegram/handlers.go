package telegram

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"mvs-kb-bot/internal/config"
	"mvs-kb-bot/internal/db"
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
}

func NewHandler(bot *tgbotapi.BotAPI, pg *db.DB, cfg config.Config) *Handler {
	h := &Handler{bot: bot, db: pg, cfg: cfg}
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
	if text == "/categories" || text == "/help" {
		active, err := h.isAllowed(ctx, uid)
		if err != nil || !active {
			h.replyHTML(m.Chat.ID, "Доступ обмежено.<br>Введіть код: <code>/start КОД</code>")
			return
		}
		h.showCategories(m.Chat.ID)
		return
	}
	h.replyText(m.Chat.ID, "Використай /start або /categories.")
}

func (h *Handler) handleStart(ctx context.Context, m *tgbotapi.Message, uid int64, code string) {
	if code != "" {
		ok, err := h.db.ActivateByCode(ctx, uid, h.cfg.AccessCode, code)
		if err != nil {
			h.replyText(m.Chat.ID, "Помилка авторизації. Спробуйте пізніше.")
			return
		}
		if !ok {
			h.replyText(m.Chat.ID, "Невірний код доступу.")
			return
		}
		h.replyText(m.Chat.ID, "Доступ підтверджено.")
	}
	active, err := h.isAllowed(ctx, uid)
	if err != nil {
		h.replyText(m.Chat.ID, "Помилка. Спробуйте пізніше.")
		return
	}
	if !active {
		h.replyHTML(m.Chat.ID, "Доступ обмежено.<br>Введіть код: <code>/start КОД</code>")
		return
	}
	h.showCategories(m.Chat.ID)
}

func (h *Handler) isAllowed(ctx context.Context, uid int64) (bool, error) {
	active, _, err := h.db.IsActive(ctx, uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
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
		h.replyText(chatID, "Не вдалося завантажити категорії.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, "Категорії")
	msg.ReplyMarkup = CategoriesKeyboard(cats)
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("send categories failed: %v", err)
	}
}

func (h *Handler) onCallback(q *tgbotapi.CallbackQuery) {
	if q.Message == nil {
		h.answerCallback(q.ID, "Немає чату для відповіді")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	uid := q.From.ID
	chatID := q.Message.Chat.ID
	active, err := h.isAllowed(ctx, uid)
	if err != nil || !active {
		h.answerCallback(q.ID, "Немає доступу")
		return
	}
	data := q.Data
	switch {
	case data == cbHome:
		h.showCategories(chatID)
	case strings.HasPrefix(data, cbCatPrefix):
		catID, err := strconv.ParseInt(strings.TrimPrefix(data, cbCatPrefix), 10, 64)
		if err != nil {
			h.answerCallback(q.ID, "Невірна категорія")
			return
		}
		h.showArticles(chatID, catID, 0)
	case strings.HasPrefix(data, cbCatPagePref):
		parts := strings.Split(strings.TrimPrefix(data, cbCatPagePref), ":")
		if len(parts) != 2 {
			h.answerCallback(q.ID, "Невірна навігація")
			return
		}
		catID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			h.answerCallback(q.ID, "Невірна категорія")
			return
		}
		offset, err := strconv.Atoi(parts[1])
		if err != nil || offset < 0 {
			h.answerCallback(q.ID, "Невірна сторінка")
			return
		}
		h.showArticles(chatID, catID, offset)
	case strings.HasPrefix(data, cbArtPrefix):
		artID, err := strconv.ParseInt(strings.TrimPrefix(data, cbArtPrefix), 10, 64)
		if err != nil {
			h.answerCallback(q.ID, "Невірна стаття")
			return
		}
		h.showArticle(chatID, artID)
	default:
		h.answerCallback(q.ID, "Невідома дія")
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
		h.replyText(chatID, "Не вдалося завантажити вкладки.")
		return
	}
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Вкладки (сторінка %d)", offset/pageSize+1))
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
		h.replyText(chatID, "Не вдалося відкрити матеріал.")
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

func (h *Handler) replyHTML(chatID int64, htmlText string) {
	msg := tgbotapi.NewMessage(chatID, htmlText)
	msg.ParseMode = "HTML"
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("send html failed: %v", err)
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
