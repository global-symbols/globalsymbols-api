package db

import (
	"database/sql"
	"fmt"
	"strings"

	"gs-api/internal/models"
)

// logoURL resolves CarrierWave-stored logo filenames (or legacy paths) to a full URL.
// DB column holds the filename only; public path is /uploads/{env}/symbolset/logo/{id}/{filename}.
func logoURL(imageBaseURL, appEnv string, symbolsetID int64, stored sql.NullString) *string {
	if !stored.Valid || strings.TrimSpace(stored.String) == "" {
		return nil
	}
	s := strings.TrimSpace(stored.String)
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return &s
	}
	if imageBaseURL == "" {
		return nil
	}
	base := strings.TrimSuffix(imageBaseURL, "/")

	// Full site-relative path already (e.g. uploads/production/symbolset/logo/96/file.png).
	rel := strings.TrimPrefix(s, "/")
	if strings.HasPrefix(rel, "uploads/") {
		u := base + "/" + rel
		return &u
	}

	// Filename only (CarrierWave :filename in DB).
	if !strings.Contains(s, "/") {
		if appEnv == "" {
			return nil
		}
		relPath := fmt.Sprintf("uploads/%s/symbolset/logo/%d/%s", appEnv, symbolsetID, s)
		u := base + "/" + relPath
		return &u
	}

	// Fallback: join like picto imagefile paths.
	u := base + "/" + rel
	return &u
}

// ListPublished returns published symbolsets ordered by featured_level (nulls last) then name.
func ListPublished(conn *sql.DB, imageBaseURL, appEnv string) ([]models.Symbolset, error) {
	rows, err := conn.Query(`
		SELECT s.id, s.slug, s.name, s.publisher, s.publisher_url, s.status, s.featured_level, s.logo,
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
		var pubURL, licURL, licVer, licProps, logo sql.NullString
		var feat sql.NullInt64
		var statusInt int
		if err := rows.Scan(
			&s.ID, &s.Slug, &s.Name, &s.Publisher, &pubURL, &statusInt, &feat, &logo,
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
		s.LogoURL = logoURL(imageBaseURL, appEnv, s.ID, logo)
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
