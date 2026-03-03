package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"mvs-kb-bot/internal/db"
)

func CategoriesKeyboard(cats []db.Category) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(cats))
	for _, c := range cats {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(c.Name, cbCatPrefix+fmt.Sprint(c.ID))))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func ArticlesKeyboard(catID int64, offset, pageSize int, arts []db.Article) tgbotapi.InlineKeyboardMarkup {
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
