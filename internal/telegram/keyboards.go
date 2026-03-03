package telegram

import (
	"fmt"

	"db_for_work_bot/internal/db"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func CategoriesKeyboard(cats []db.Category) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(cats))
	for _, c := range cats {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(c.Name, cbCatPrefix+fmt.Sprint(c.ID))))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func ArticlesKeyboard(catID int64, offset, pageSize int, arts []db.ArticlePreview) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(arts)+2)
	for _, a := range arts {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Doc: "+a.Title, cbArtPrefix+fmt.Sprint(a.ID))))
	}
	var nav []tgbotapi.InlineKeyboardButton
	if offset > 0 {
		prev := offset - pageSize
		if prev < 0 {
			prev = 0
		}
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("< Prev", cbCatPagePref+fmt.Sprintf("%d:%d", catID, prev)))
	}
	if len(arts) == pageSize {
		nav = append(nav, tgbotapi.NewInlineKeyboardButtonData("Next >", cbCatPagePref+fmt.Sprintf("%d:%d", catID, offset+pageSize)))
	}
	if len(nav) > 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(nav...))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Back to categories", cbHome)))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func BackToCategoriesKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Back to categories", cbHome)))
}

func AdminMainKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📁 Категорії", cbAdmCats)),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📝 Статті", cbAdmArts)),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("❌ Закрити", cbAdmClose)),
	)
}

func AdminCancelKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Скасувати", cbAdmCancel)),
	)
}

func AdminCategoriesKeyboard(cats []db.AdminCategory) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(cats)+5)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ Додати категорію", cbAdmCatAdd),
	))
	for _, c := range cats {
		status := "✅"
		toggleLabel := "🚫 Вимк"
		if !c.IsActive {
			status = "🚫"
			toggleLabel = "✅ Увімк"
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(status+" "+shortButtonText(c.Name, 20), cbAdmCatEditPrefix+fmt.Sprint(c.ID)),
			tgbotapi.NewInlineKeyboardButtonData(toggleLabel, cbAdmCatTogglePrefix+fmt.Sprint(c.ID)),
			tgbotapi.NewInlineKeyboardButtonData("▲", cbAdmCatUpPrefix+fmt.Sprint(c.ID)),
			tgbotapi.NewInlineKeyboardButtonData("▼", cbAdmCatDownPrefix+fmt.Sprint(c.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📝 Статті", cbAdmArts),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Меню", cbAdmMenu),
		tgbotapi.NewInlineKeyboardButtonData("❌ Закрити", cbAdmClose),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Скасувати", cbAdmCancel),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func AdminArticleCategoriesKeyboard(cats []db.AdminCategory) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(cats)+3)
	for _, c := range cats {
		status := "✅"
		if !c.IsActive {
			status = "🚫"
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(status+" "+shortButtonText(c.Name, 28), cbAdmArtsCatPrefix+fmt.Sprint(c.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📁 Категорії", cbAdmCats),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Меню", cbAdmMenu),
		tgbotapi.NewInlineKeyboardButtonData("❌ Закрити", cbAdmClose),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Скасувати", cbAdmCancel),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func AdminArticlesKeyboard(catID int64, arts []db.AdminArticlePreview) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(arts)*2+5)
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ Додати статтю", cbAdmArtAddPrefix+fmt.Sprint(catID)),
	))
	for _, a := range arts {
		status := "✅"
		toggleLabel := "🚫 Приховати"
		if !a.IsPublished {
			status = "🚫"
			toggleLabel = "✅ Опублікувати"
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(status+" "+shortButtonText(a.Title, 28), cbAdmArtEditTitlePrefix+fmt.Sprint(a.ID)),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Текст", cbAdmArtEditBodyPrefix+fmt.Sprint(a.ID)),
			tgbotapi.NewInlineKeyboardButtonData(toggleLabel, cbAdmArtTogglePrefix+fmt.Sprint(a.ID)),
			tgbotapi.NewInlineKeyboardButtonData("▲", cbAdmArtUpPrefix+fmt.Sprint(a.ID)),
			tgbotapi.NewInlineKeyboardButtonData("▼", cbAdmArtDownPrefix+fmt.Sprint(a.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Категорії статей", cbAdmArts),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Меню", cbAdmMenu),
		tgbotapi.NewInlineKeyboardButtonData("❌ Закрити", cbAdmClose),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Скасувати", cbAdmCancel),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func shortButtonText(s string, maxRunes int) string {
	r := []rune(s)
	if maxRunes <= 0 || len(r) <= maxRunes {
		return s
	}
	if maxRunes <= 1 {
		return string(r[:maxRunes])
	}
	return string(r[:maxRunes-1]) + "…"
}
