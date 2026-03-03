package telegram

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"db_for_work_bot/internal/db"
)

const (
	cbAdmPrefix = "adm:"
	cbAdmMenu   = "adm:menu"
	cbAdmClose  = "adm:close"
	cbAdmCancel = "adm:cancel"
	cbAdmCats   = "adm:cats"
	cbAdmArts   = "adm:arts"

	cbAdmCatAdd          = "adm:cat:add"
	cbAdmCatEditPrefix   = "adm:cat:edit:"
	cbAdmCatTogglePrefix = "adm:cat:toggle:"
	cbAdmCatUpPrefix     = "adm:cat:up:"
	cbAdmCatDownPrefix   = "adm:cat:down:"

	cbAdmArtsCatPrefix = "adm:arts:cat:"

	cbAdmArtAddPrefix       = "adm:art:add:"
	cbAdmArtEditTitlePrefix = "adm:art:edit_title:"
	cbAdmArtEditBodyPrefix  = "adm:art:edit_body:"
	cbAdmArtTogglePrefix    = "adm:art:toggle:"
	cbAdmArtUpPrefix        = "adm:art:up:"
	cbAdmArtDownPrefix      = "adm:art:down:"
)

const (
	adminStateAddCategoryName  = "add_category_name"
	adminStateEditCategoryName = "edit_category_name"
	adminStateAddArticleTitle  = "add_article_title"
	adminStateAddArticleBody   = "add_article_body"
	adminStateEditArticleTitle = "edit_article_title"
	adminStateEditArticleBody  = "edit_article_body"
)

type AdminSession struct {
	State      string
	CategoryID int64
	ArticleID  int64
	DraftTitle string
}

func (h *Handler) handleAdminEntry(ctx context.Context, chatID, uid int64) {
	ok, err := h.isAdmin(ctx, uid)
	if err != nil {
		h.replyText(chatID, "Temporary error. Try again later.")
		return
	}
	if !ok {
		h.replyText(chatID, "Немає доступу")
		return
	}
	h.clearAdminSession(uid)
	h.showAdminMenu(chatID)
}

func (h *Handler) handleAdminCancel(chatID, uid int64) bool {
	if !h.clearAdminSession(uid) {
		return false
	}
	h.replyText(chatID, "Скасовано.")
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if isAdmin, err := h.isAdmin(ctx, uid); err == nil && isAdmin {
		h.showAdminMenu(chatID)
	}
	return true
}

func (h *Handler) handleAdminMessageInput(ctx context.Context, chatID, uid int64, text string) bool {
	session, ok := h.getAdminSession(uid)
	if !ok {
		return false
	}
	isAdmin, err := h.isAdmin(ctx, uid)
	if err != nil {
		h.replyText(chatID, "Temporary error. Try again later.")
		return true
	}
	if !isAdmin {
		h.clearAdminSession(uid)
		h.replyText(chatID, "Немає доступу")
		return true
	}

	switch session.State {
	case adminStateAddCategoryName:
		_, err := h.db.CreateCategory(ctx, text)
		if err != nil {
			switch {
			case errors.Is(err, db.ErrCategoryNameExists):
				h.replyText(chatID, "Категорія з такою назвою вже існує")
				h.promptAdminInput(chatID, "Введіть іншу назву категорії:")
			case errors.Is(err, db.ErrEmptyName):
				h.promptAdminInput(chatID, "Назва не може бути порожньою. Введіть назву категорії:")
			default:
				log.Printf("create category failed: %v", err)
				h.replyText(chatID, "Не вдалося додати категорію.")
			}
			return true
		}
		h.clearAdminSession(uid)
		h.replyText(chatID, "Категорію додано.")
		h.showAdminCategories(chatID)
		return true

	case adminStateEditCategoryName:
		err := h.db.RenameCategory(ctx, session.CategoryID, text)
		if err != nil {
			switch {
			case errors.Is(err, db.ErrCategoryNameExists):
				h.replyText(chatID, "Категорія з такою назвою вже існує")
				h.promptAdminInput(chatID, "Введіть іншу назву категорії:")
			case errors.Is(err, db.ErrEmptyName):
				h.promptAdminInput(chatID, "Назва не може бути порожньою. Введіть нову назву:")
			case errors.Is(err, sql.ErrNoRows):
				h.clearAdminSession(uid)
				h.replyText(chatID, "Категорію не знайдено.")
				h.showAdminCategories(chatID)
			default:
				log.Printf("rename category failed: %v", err)
				h.replyText(chatID, "Не вдалося перейменувати категорію.")
			}
			return true
		}
		h.clearAdminSession(uid)
		h.replyText(chatID, "Категорію оновлено.")
		h.showAdminCategories(chatID)
		return true

	case adminStateAddArticleTitle:
		if strings.TrimSpace(text) == "" {
			h.promptAdminInput(chatID, "Заголовок не може бути порожнім. Введіть title:")
			return true
		}
		session.DraftTitle = strings.TrimSpace(text)
		session.State = adminStateAddArticleBody
		h.setAdminSession(uid, session)
		h.promptAdminInput(chatID, "Введіть текст статті (Body):")
		return true

	case adminStateAddArticleBody:
		_, err := h.db.CreateArticle(ctx, session.CategoryID, session.DraftTitle, text)
		if err != nil {
			switch {
			case errors.Is(err, db.ErrArticleTitleExists):
				session.State = adminStateAddArticleTitle
				session.DraftTitle = ""
				h.setAdminSession(uid, session)
				h.replyText(chatID, "У цій категорії вже є стаття з таким заголовком")
				h.promptAdminInput(chatID, "Введіть інший title:")
			case errors.Is(err, db.ErrEmptyBody):
				h.promptAdminInput(chatID, "Текст статті не може бути порожнім. Введіть Body:")
			case errors.Is(err, db.ErrEmptyTitle):
				session.State = adminStateAddArticleTitle
				session.DraftTitle = ""
				h.setAdminSession(uid, session)
				h.promptAdminInput(chatID, "Введіть title:")
			default:
				log.Printf("create article failed: %v", err)
				h.replyText(chatID, "Не вдалося додати статтю.")
			}
			return true
		}
		h.clearAdminSession(uid)
		h.replyText(chatID, "Статтю додано.")
		h.showAdminArticles(chatID, session.CategoryID)
		return true

	case adminStateEditArticleTitle:
		catID, err := h.db.UpdateArticleTitle(ctx, session.ArticleID, text)
		if err != nil {
			switch {
			case errors.Is(err, db.ErrArticleTitleExists):
				h.replyText(chatID, "У цій категорії вже є стаття з таким заголовком")
				h.promptAdminInput(chatID, "Введіть інший title:")
			case errors.Is(err, db.ErrEmptyTitle):
				h.promptAdminInput(chatID, "Заголовок не може бути порожнім. Введіть новий title:")
			case errors.Is(err, sql.ErrNoRows):
				h.clearAdminSession(uid)
				h.replyText(chatID, "Статтю не знайдено.")
				h.showAdminArticleCategories(chatID)
			default:
				log.Printf("update article title failed: %v", err)
				h.replyText(chatID, "Не вдалося оновити заголовок статті.")
			}
			return true
		}
		h.clearAdminSession(uid)
		h.replyText(chatID, "Заголовок статті оновлено.")
		h.showAdminArticles(chatID, catID)
		return true

	case adminStateEditArticleBody:
		catID, err := h.db.UpdateArticleBody(ctx, session.ArticleID, text)
		if err != nil {
			switch {
			case errors.Is(err, db.ErrEmptyBody):
				h.promptAdminInput(chatID, "Текст статті не може бути порожнім. Введіть новий Body:")
			case errors.Is(err, sql.ErrNoRows):
				h.clearAdminSession(uid)
				h.replyText(chatID, "Статтю не знайдено.")
				h.showAdminArticleCategories(chatID)
			default:
				log.Printf("update article body failed: %v", err)
				h.replyText(chatID, "Не вдалося оновити текст статті.")
			}
			return true
		}
		h.clearAdminSession(uid)
		h.replyText(chatID, "Текст статті оновлено.")
		h.showAdminArticles(chatID, catID)
		return true

	default:
		h.clearAdminSession(uid)
		return false
	}
}

func (h *Handler) onAdminCallback(ctx context.Context, q *tgbotapi.CallbackQuery, chatID, uid int64) {
	isAdmin, err := h.isAdmin(ctx, uid)
	if err != nil {
		h.answerCallback(q.ID, "Temporary error")
		return
	}
	if !isAdmin {
		h.clearAdminSession(uid)
		h.answerCallback(q.ID, "Немає доступу")
		return
	}

	data := q.Data
	h.clearAdminSession(uid)
	switch {
	case data == cbAdmMenu:
		h.clearAdminSession(uid)
		h.showAdminMenu(chatID)
	case data == cbAdmClose:
		h.clearAdminSession(uid)
		h.replyText(chatID, "Адмін-панель закрито.")
	case data == cbAdmCancel:
		h.clearAdminSession(uid)
		h.replyText(chatID, "Скасовано.")
		h.showAdminMenu(chatID)
	case data == cbAdmCats:
		h.clearAdminSession(uid)
		h.showAdminCategories(chatID)
	case data == cbAdmArts:
		h.clearAdminSession(uid)
		h.showAdminArticleCategories(chatID)
	case data == cbAdmCatAdd:
		h.setAdminSession(uid, AdminSession{State: adminStateAddCategoryName})
		h.promptAdminInput(chatID, "Введіть назву нової категорії:")
	case strings.HasPrefix(data, cbAdmCatEditPrefix):
		id, err := parseAdminID(data, cbAdmCatEditPrefix)
		if err != nil {
			h.answerCallback(q.ID, "Некоректна категорія")
			return
		}
		h.setAdminSession(uid, AdminSession{State: adminStateEditCategoryName, CategoryID: id})
		h.promptAdminInput(chatID, "Введіть нову назву категорії:")
	case strings.HasPrefix(data, cbAdmCatTogglePrefix):
		id, err := parseAdminID(data, cbAdmCatTogglePrefix)
		if err != nil {
			h.answerCallback(q.ID, "Некоректна категорія")
			return
		}
		if _, err := h.db.ToggleCategoryActive(ctx, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				h.answerCallback(q.ID, "Категорію не знайдено")
				return
			}
			log.Printf("toggle category failed: %v", err)
			h.answerCallback(q.ID, "Помилка")
			return
		}
		h.showAdminCategories(chatID)
	case strings.HasPrefix(data, cbAdmCatUpPrefix):
		id, err := parseAdminID(data, cbAdmCatUpPrefix)
		if err != nil {
			h.answerCallback(q.ID, "Некоректна категорія")
			return
		}
		if err := h.db.MoveCategory(ctx, id, true); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				h.answerCallback(q.ID, "Категорію не знайдено")
				return
			}
			log.Printf("move category up failed: %v", err)
			h.answerCallback(q.ID, "Помилка")
			return
		}
		h.showAdminCategories(chatID)
	case strings.HasPrefix(data, cbAdmCatDownPrefix):
		id, err := parseAdminID(data, cbAdmCatDownPrefix)
		if err != nil {
			h.answerCallback(q.ID, "Некоректна категорія")
			return
		}
		if err := h.db.MoveCategory(ctx, id, false); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				h.answerCallback(q.ID, "Категорію не знайдено")
				return
			}
			log.Printf("move category down failed: %v", err)
			h.answerCallback(q.ID, "Помилка")
			return
		}
		h.showAdminCategories(chatID)
	case strings.HasPrefix(data, cbAdmArtsCatPrefix):
		catID, err := parseAdminID(data, cbAdmArtsCatPrefix)
		if err != nil {
			h.answerCallback(q.ID, "Некоректна категорія")
			return
		}
		h.clearAdminSession(uid)
		h.showAdminArticles(chatID, catID)
	case strings.HasPrefix(data, cbAdmArtAddPrefix):
		catID, err := parseAdminID(data, cbAdmArtAddPrefix)
		if err != nil {
			h.answerCallback(q.ID, "Некоректна категорія")
			return
		}
		h.setAdminSession(uid, AdminSession{State: adminStateAddArticleTitle, CategoryID: catID})
		h.promptAdminInput(chatID, "Введіть заголовок статті (Title):")
	case strings.HasPrefix(data, cbAdmArtEditTitlePrefix):
		artID, err := parseAdminID(data, cbAdmArtEditTitlePrefix)
		if err != nil {
			h.answerCallback(q.ID, "Некоректна стаття")
			return
		}
		h.setAdminSession(uid, AdminSession{State: adminStateEditArticleTitle, ArticleID: artID})
		h.promptAdminInput(chatID, "Введіть новий заголовок статті:")
	case strings.HasPrefix(data, cbAdmArtEditBodyPrefix):
		artID, err := parseAdminID(data, cbAdmArtEditBodyPrefix)
		if err != nil {
			h.answerCallback(q.ID, "Некоректна стаття")
			return
		}
		h.setAdminSession(uid, AdminSession{State: adminStateEditArticleBody, ArticleID: artID})
		h.promptAdminInput(chatID, "Введіть новий текст статті:")
	case strings.HasPrefix(data, cbAdmArtTogglePrefix):
		artID, err := parseAdminID(data, cbAdmArtTogglePrefix)
		if err != nil {
			h.answerCallback(q.ID, "Некоректна стаття")
			return
		}
		catID, _, err := h.db.ToggleArticlePublished(ctx, artID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				h.answerCallback(q.ID, "Статтю не знайдено")
				return
			}
			log.Printf("toggle article failed: %v", err)
			h.answerCallback(q.ID, "Помилка")
			return
		}
		h.showAdminArticles(chatID, catID)
	case strings.HasPrefix(data, cbAdmArtUpPrefix):
		artID, err := parseAdminID(data, cbAdmArtUpPrefix)
		if err != nil {
			h.answerCallback(q.ID, "Некоректна стаття")
			return
		}
		catID, err := h.db.MoveArticle(ctx, artID, true)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				h.answerCallback(q.ID, "Статтю не знайдено")
				return
			}
			log.Printf("move article up failed: %v", err)
			h.answerCallback(q.ID, "Помилка")
			return
		}
		h.showAdminArticles(chatID, catID)
	case strings.HasPrefix(data, cbAdmArtDownPrefix):
		artID, err := parseAdminID(data, cbAdmArtDownPrefix)
		if err != nil {
			h.answerCallback(q.ID, "Некоректна стаття")
			return
		}
		catID, err := h.db.MoveArticle(ctx, artID, false)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				h.answerCallback(q.ID, "Статтю не знайдено")
				return
			}
			log.Printf("move article down failed: %v", err)
			h.answerCallback(q.ID, "Помилка")
			return
		}
		h.showAdminArticles(chatID, catID)
	default:
		h.answerCallback(q.ID, "Unknown admin action")
		return
	}
	h.answerCallback(q.ID, "")
}

func (h *Handler) showAdminMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Адмін-панель")
	msg.ReplyMarkup = AdminMainKeyboard()
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("send admin menu failed: %v", err)
	}
}

func (h *Handler) showAdminCategories(chatID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	cats, err := h.db.ListAllCategories(ctx)
	if err != nil {
		log.Printf("list admin categories failed: %v", err)
		h.replyText(chatID, "Не вдалося завантажити категорії.")
		return
	}

	var b strings.Builder
	b.WriteString("Категорії (натисніть назву для перейменування):\n")
	if len(cats) == 0 {
		b.WriteString("Список порожній.")
	} else {
		for _, c := range cats {
			status := "✅"
			if !c.IsActive {
				status = "🚫"
			}
			fmt.Fprintf(&b, "%s #%d %s (sort:%d)\n", status, c.ID, c.Name, c.SortOrder)
		}
	}
	keyboard := AdminCategoriesKeyboard(cats)
	h.sendChunkedText(chatID, b.String(), &keyboard)
}

func (h *Handler) showAdminArticleCategories(chatID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	cats, err := h.db.ListAllCategories(ctx)
	if err != nil {
		log.Printf("list article categories failed: %v", err)
		h.replyText(chatID, "Не вдалося завантажити категорії.")
		return
	}

	var b strings.Builder
	b.WriteString("Оберіть категорію для керування статтями:\n")
	if len(cats) == 0 {
		b.WriteString("Категорій поки немає.")
	} else {
		for _, c := range cats {
			status := "✅"
			if !c.IsActive {
				status = "🚫"
			}
			fmt.Fprintf(&b, "%s #%d %s\n", status, c.ID, c.Name)
		}
	}

	keyboard := AdminArticleCategoriesKeyboard(cats)
	h.sendChunkedText(chatID, b.String(), &keyboard)
}

func (h *Handler) showAdminArticles(chatID, catID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	cat, err := h.db.GetCategory(ctx, catID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.replyText(chatID, "Категорію не знайдено.")
			return
		}
		log.Printf("get category failed: %v", err)
		h.replyText(chatID, "Не вдалося завантажити категорію.")
		return
	}

	arts, err := h.db.ListAllArticlesByCategory(ctx, catID)
	if err != nil {
		log.Printf("list admin articles failed: %v", err)
		h.replyText(chatID, "Не вдалося завантажити статті.")
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Статті в категорії \"%s\":\n", cat.Name)
	if len(arts) == 0 {
		b.WriteString("Список порожній.")
	} else {
		for _, a := range arts {
			status := "✅"
			if !a.IsPublished {
				status = "🚫"
			}
			fmt.Fprintf(&b, "%s #%d %s (sort:%d)\n", status, a.ID, a.Title, a.SortOrder)
		}
	}
	keyboard := AdminArticlesKeyboard(catID, arts)
	h.sendChunkedText(chatID, b.String(), &keyboard)
}

func (h *Handler) promptAdminInput(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = AdminCancelKeyboard()
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("send admin prompt failed: %v", err)
	}
}

func (h *Handler) sendChunkedText(chatID int64, text string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	chunks := splitByLen(text, 3500)
	for i, ch := range chunks {
		msg := tgbotapi.NewMessage(chatID, ch)
		if keyboard != nil && i == len(chunks)-1 {
			msg.ReplyMarkup = *keyboard
		}
		if _, err := h.bot.Send(msg); err != nil {
			log.Printf("send chunk failed: %v", err)
			return
		}
	}
}

func parseAdminID(data, prefix string) (int64, error) {
	raw := strings.TrimPrefix(data, prefix)
	if raw == data || strings.TrimSpace(raw) == "" {
		return 0, fmt.Errorf("invalid callback id")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (h *Handler) isAdmin(ctx context.Context, uid int64) (bool, error) {
	return h.db.IsAdmin(ctx, uid)
}

func (h *Handler) getAdminSession(uid int64) (AdminSession, bool) {
	h.adminMu.RLock()
	defer h.adminMu.RUnlock()
	s, ok := h.adminSessions[uid]
	return s, ok
}

func (h *Handler) setAdminSession(uid int64, s AdminSession) {
	h.adminMu.Lock()
	defer h.adminMu.Unlock()
	h.adminSessions[uid] = s
}

func (h *Handler) clearAdminSession(uid int64) bool {
	h.adminMu.Lock()
	defer h.adminMu.Unlock()
	if _, ok := h.adminSessions[uid]; !ok {
		return false
	}
	delete(h.adminSessions, uid)
	return true
}
