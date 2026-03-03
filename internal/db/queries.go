package db

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
)

type Category struct {
	ID   int64
	Name string
}

type Article struct {
	ID         int64
	CategoryID int64
	Title      string
	Body       string
}

func (d *DB) EnsureAdmin(ctx context.Context, telegramUserID int64) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO users (telegram_user_id, role, is_active, last_seen_at)
		VALUES ($1, 'admin', true, now())
		ON CONFLICT (telegram_user_id)
		DO UPDATE SET role = 'admin', is_active = true, last_seen_at = now()
	`, telegramUserID)
	return err
}

func (d *DB) TouchSeen(ctx context.Context, telegramUserID int64) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO users (telegram_user_id, last_seen_at)
		VALUES ($1, now())
		ON CONFLICT (telegram_user_id)
		DO UPDATE SET last_seen_at = now()
	`, telegramUserID)
	return err
}

func (d *DB) ActivateByCode(ctx context.Context, telegramUserID int64, accessCode, submittedCode string) (bool, error) {
	a := strings.TrimSpace(accessCode)
	b := strings.TrimSpace(submittedCode)
	if a == "" || b == "" || len(a) != len(b) || subtle.ConstantTimeCompare([]byte(a), []byte(b)) != 1 {
		return false, nil
	}
	_, err := d.ExecContext(ctx, `
		INSERT INTO users (telegram_user_id, role, is_active, last_seen_at)
		VALUES ($1, 'user', true, now())
		ON CONFLICT (telegram_user_id)
		DO UPDATE SET is_active = true, last_seen_at = now()
	`, telegramUserID)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *DB) IsActive(ctx context.Context, telegramUserID int64) (bool, string, error) {
	var active bool
	var role string
	err := d.QueryRowContext(ctx, `
		SELECT is_active, role
		FROM users
		WHERE telegram_user_id = $1
	`, telegramUserID).Scan(&active, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return active, role, nil
}

func (d *DB) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, name
		FROM categories
		WHERE is_active = true
		ORDER BY sort_order, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) ListArticlesByCategory(ctx context.Context, catID int64, limit, offset int) ([]Article, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := d.QueryContext(ctx, `
		SELECT id, category_id, title, body
		FROM articles
		WHERE category_id = $1
			AND is_published = true
		ORDER BY sort_order, id
		LIMIT $2 OFFSET $3
	`, catID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.CategoryID, &a.Title, &a.Body); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) GetArticle(ctx context.Context, articleID int64) (Article, error) {
	var a Article
	err := d.QueryRowContext(ctx, `
		SELECT id, category_id, title, body
		FROM articles
		WHERE id = $1
			AND is_published = true
	`, articleID).Scan(&a.ID, &a.CategoryID, &a.Title, &a.Body)
	return a, err
}
