// Package catalog implements the read API for the localized category taxonomy
// (GET /api/v1/categories). Translations are resolved in the service layer using
// the ADR 0001 fallback chain: requested lang → en → any.
package catalog

import (
	"context"
	"database/sql"
)

// Category is a taxonomy entry with its name resolved to the caller's language.
type Category struct {
	Slug       string  `json:"slug"`
	Name       string  `json:"name"`
	ParentSlug *string `json:"parent_slug"`
	SortOrder  int     `json:"sort_order"`
}

// Store queries the category taxonomy from Postgres.
type Store struct {
	db *sql.DB
}

// NewStore wraps db in a Store.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// listCategoriesSQL returns every category with its name resolved to lang via the
// ADR 0001 fallback chain. The CTE picks the best available translation per
// category by ranking requested-lang first (priority 1), then en (2), then any
// other lang (3), and keeps only rank-1 rows per category.
const listCategoriesSQL = `
WITH resolved AS (
    SELECT
        ct.category_id,
        ct.name,
        ROW_NUMBER() OVER (
            PARTITION BY ct.category_id
            ORDER BY CASE ct.lang
                WHEN $1  THEN 1
                WHEN 'en' THEN 2
                ELSE 3
            END
        ) AS rn
    FROM category_translations ct
)
SELECT
    c.slug,
    r.name,
    p.slug   AS parent_slug,
    c.sort_order
FROM categories c
JOIN  resolved r ON r.category_id = c.id AND r.rn = 1
LEFT JOIN categories p ON p.id = c.parent_id
ORDER BY c.sort_order, r.name`

// ListCategories returns all categories localized to lang (ADR 0001 fallback).
// lang must be one of "en", "si", "ta"; validation is the caller's responsibility.
func (s *Store) ListCategories(ctx context.Context, lang string) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, listCategoriesSQL, lang)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.Slug, &c.Name, &c.ParentSlug, &c.SortOrder); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if cats == nil {
		cats = []Category{} // return [] not null
	}
	return cats, nil
}
