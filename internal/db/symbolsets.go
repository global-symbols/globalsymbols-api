package db

import (
	"database/sql"

	"gs-api/internal/models"
)

// ListPublished returns published symbolsets ordered by featured_level (nulls last) then name.
func ListPublished(conn *sql.DB) ([]models.Symbolset, error) {
	rows, err := conn.Query(`
		SELECT s.id, s.slug, s.name, s.publisher, s.publisher_url, s.status, s.featured_level,
		       l.name, l.url, l.version, l.properties
		FROM symbolsets s
		JOIN licences l ON l.id = s.licence_id
		WHERE s.status = 0
		ORDER BY s.featured_level IS NULL, s.featured_level ASC, s.name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Symbolset
	for rows.Next() {
		var s models.Symbolset
		var pubURL, licURL, licVer, licProps sql.NullString
		var feat sql.NullInt64
		var statusInt int
		if err := rows.Scan(
			&s.ID, &s.Slug, &s.Name, &s.Publisher, &pubURL, &statusInt, &feat,
			&s.Licence.Name, &licURL, &licVer, &licProps,
		); err != nil {
			return nil, err
		}
		s.Status = SymbolsetStatusFromInt(statusInt)
		if pubURL.Valid {
			s.PublisherURL = &pubURL.String
		}
		if feat.Valid {
			s.Featured = &feat.Int64
		}
		if licURL.Valid {
			s.Licence.URL = &licURL.String
		}
		if licVer.Valid {
			s.Licence.Version = &licVer.String
		}
		if licProps.Valid {
			s.Licence.Properties = &licProps.String
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SymbolsetBySlug returns symbolset id and status if the slug exists.
func SymbolsetBySlug(conn *sql.DB, slug string) (id int64, status int, err error) {
	err = conn.QueryRow(
		`SELECT id, status FROM symbolsets WHERE slug = ?`,
		slug,
	).Scan(&id, &status)
	return id, status, err
}
