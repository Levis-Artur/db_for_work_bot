package db

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrCategoryNameExists = errors.New("category with this name already exists")
	ErrArticleTitleExists = errors.New("article with this title already exists in this category")
	ErrEmptyName          = errors.New("name is required")
	ErrEmptyTitle         = errors.New("title is required")
	ErrEmptyBody          = errors.New("body is required")
)

type Category struct {
	ID   int64
	Name string
}

type AdminCategory struct {
	ID        int64
	Name      string
	SortOrder int
	IsActive  bool
}

type ArticlePreview struct {
	ID    int64
	Title string
}

type AdminArticlePreview struct {
	ID          int64
	CategoryID  int64
	Title       string
	SortOrder   int
	IsPublished bool
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

func (d *DB) IsAdmin(ctx context.Context, telegramUserID int64) (bool, error) {
	active, role, err := d.IsActive(ctx, telegramUserID)
	if err != nil {
		return false, err
	}
	if !active {
		return false, nil
	}
	return role == "admin" || role == "owner", nil
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

func (d *DB) ListAllCategories(ctx context.Context) ([]AdminCategory, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, name, sort_order, is_active
		FROM categories
		ORDER BY sort_order, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminCategory
	for rows.Next() {
		var c AdminCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.SortOrder, &c.IsActive); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) GetCategory(ctx context.Context, categoryID int64) (AdminCategory, error) {
	var c AdminCategory
	err := d.QueryRowContext(ctx, `
		SELECT id, name, sort_order, is_active
		FROM categories
		WHERE id = $1
	`, categoryID).Scan(&c.ID, &c.Name, &c.SortOrder, &c.IsActive)
	return c, err
}

func (d *DB) CreateCategory(ctx context.Context, name string) (Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, ErrEmptyName
	}
	var c Category
	err := d.QueryRowContext(ctx, `
		INSERT INTO categories(name, sort_order, is_active)
		VALUES ($1, COALESCE((SELECT MAX(sort_order) + 10 FROM categories), 10), true)
		RETURNING id, name
	`, name).Scan(&c.ID, &c.Name)
	if err != nil {
		return Category{}, mapDBError(err)
	}
	return c, nil
}

func (d *DB) RenameCategory(ctx context.Context, categoryID int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrEmptyName
	}
	res, err := d.ExecContext(ctx, `
		UPDATE categories
		SET name = $2
		WHERE id = $1
	`, categoryID, name)
	if err != nil {
		return mapDBError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (d *DB) ToggleCategoryActive(ctx context.Context, categoryID int64) (bool, error) {
	var isActive bool
	err := d.QueryRowContext(ctx, `
		UPDATE categories
		SET is_active = NOT is_active
		WHERE id = $1
		RETURNING is_active
	`, categoryID).Scan(&isActive)
	return isActive, err
}

func (d *DB) MoveCategory(ctx context.Context, categoryID int64, moveUp bool) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentID int64
	var currentSort int
	if err := tx.QueryRowContext(ctx, `
		SELECT id, sort_order
		FROM categories
		WHERE id = $1
		FOR UPDATE
	`, categoryID).Scan(&currentID, &currentSort); err != nil {
		return err
	}

	var neighborID int64
	var neighborSort int
	if moveUp {
		err = tx.QueryRowContext(ctx, `
			SELECT id, sort_order
			FROM categories
			WHERE (sort_order < $1) OR (sort_order = $1 AND id < $2)
			ORDER BY sort_order DESC, id DESC
			LIMIT 1
			FOR UPDATE
		`, currentSort, currentID).Scan(&neighborID, &neighborSort)
	} else {
		err = tx.QueryRowContext(ctx, `
			SELECT id, sort_order
			FROM categories
			WHERE (sort_order > $1) OR (sort_order = $1 AND id > $2)
			ORDER BY sort_order ASC, id ASC
			LIMIT 1
			FOR UPDATE
		`, currentSort, currentID).Scan(&neighborID, &neighborSort)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE categories
		SET sort_order = $2
		WHERE id = $1
	`, currentID, neighborSort); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE categories
		SET sort_order = $2
		WHERE id = $1
	`, neighborID, currentSort); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *DB) ListArticlesByCategory(ctx context.Context, catID int64, limit, offset int) ([]ArticlePreview, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := d.QueryContext(ctx, `
		SELECT id, title
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
	var out []ArticlePreview
	for rows.Next() {
		var a ArticlePreview
		if err := rows.Scan(&a.ID, &a.Title); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) ListAllArticlesByCategory(ctx context.Context, catID int64) ([]AdminArticlePreview, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, category_id, title, sort_order, is_published
		FROM articles
		WHERE category_id = $1
		ORDER BY sort_order, id
	`, catID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminArticlePreview
	for rows.Next() {
		var a AdminArticlePreview
		if err := rows.Scan(&a.ID, &a.CategoryID, &a.Title, &a.SortOrder, &a.IsPublished); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) CreateArticle(ctx context.Context, catID int64, title, body string) (ArticlePreview, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return ArticlePreview{}, ErrEmptyTitle
	}
	if strings.TrimSpace(body) == "" {
		return ArticlePreview{}, ErrEmptyBody
	}
	var a ArticlePreview
	err := d.QueryRowContext(ctx, `
		INSERT INTO articles(category_id, title, body, sort_order, is_published)
		VALUES (
			$1,
			$2,
			$3,
			COALESCE((SELECT MAX(sort_order) + 10 FROM articles WHERE category_id = $1), 10),
			true
		)
		RETURNING id, title
	`, catID, title, body).Scan(&a.ID, &a.Title)
	if err != nil {
		return ArticlePreview{}, mapDBError(err)
	}
	return a, nil
}

func (d *DB) UpdateArticleTitle(ctx context.Context, articleID int64, title string) (int64, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return 0, ErrEmptyTitle
	}
	var categoryID int64
	err := d.QueryRowContext(ctx, `
		UPDATE articles
		SET title = $2, updated_at = now()
		WHERE id = $1
		RETURNING category_id
	`, articleID, title).Scan(&categoryID)
	if err != nil {
		return 0, mapDBError(err)
	}
	return categoryID, nil
}

func (d *DB) UpdateArticleBody(ctx context.Context, articleID int64, body string) (int64, error) {
	if strings.TrimSpace(body) == "" {
		return 0, ErrEmptyBody
	}
	var categoryID int64
	err := d.QueryRowContext(ctx, `
		UPDATE articles
		SET body = $2, updated_at = now()
		WHERE id = $1
		RETURNING category_id
	`, articleID, body).Scan(&categoryID)
	return categoryID, err
}

func (d *DB) ToggleArticlePublished(ctx context.Context, articleID int64) (int64, bool, error) {
	var categoryID int64
	var isPublished bool
	err := d.QueryRowContext(ctx, `
		UPDATE articles
		SET is_published = NOT is_published, updated_at = now()
		WHERE id = $1
		RETURNING category_id, is_published
	`, articleID).Scan(&categoryID, &isPublished)
	return categoryID, isPublished, err
}

func (d *DB) MoveArticle(ctx context.Context, articleID int64, moveUp bool) (int64, error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var currentID int64
	var categoryID int64
	var currentSort int
	if err := tx.QueryRowContext(ctx, `
		SELECT id, category_id, sort_order
		FROM articles
		WHERE id = $1
		FOR UPDATE
	`, articleID).Scan(&currentID, &categoryID, &currentSort); err != nil {
		return 0, err
	}

	var neighborID int64
	var neighborSort int
	if moveUp {
		err = tx.QueryRowContext(ctx, `
			SELECT id, sort_order
			FROM articles
			WHERE category_id = $1
				AND ((sort_order < $2) OR (sort_order = $2 AND id < $3))
			ORDER BY sort_order DESC, id DESC
			LIMIT 1
			FOR UPDATE
		`, categoryID, currentSort, currentID).Scan(&neighborID, &neighborSort)
	} else {
		err = tx.QueryRowContext(ctx, `
			SELECT id, sort_order
			FROM articles
			WHERE category_id = $1
				AND ((sort_order > $2) OR (sort_order = $2 AND id > $3))
			ORDER BY sort_order ASC, id ASC
			LIMIT 1
			FOR UPDATE
		`, categoryID, currentSort, currentID).Scan(&neighborID, &neighborSort)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return categoryID, tx.Commit()
	}
	if err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE articles
		SET sort_order = $2
		WHERE id = $1
	`, currentID, neighborSort); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE articles
		SET sort_order = $2
		WHERE id = $1
	`, neighborID, currentSort); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return categoryID, nil
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

func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "uq_categories_name":
			return ErrCategoryNameExists
		case "uq_articles_category_title":
			return ErrArticleTitleExists
		}
	}
	return err
}
