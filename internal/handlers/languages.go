package handlers

import (
	"database/sql"
	"net/http"

	"gs-api/internal/httpx"
	"gs-api/internal/models"
)

func LanguagesActive(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(`
			SELECT id, name, scope, category, iso639_1, iso639_2b, iso639_2t, iso639_3
			FROM languages
			WHERE active = 1
			ORDER BY name`,
		)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		defer rows.Close()

		var result []models.Language
		for rows.Next() {
			var (
				l          models.Language
				iso6391    sql.NullString
				iso6392b   sql.NullString
				iso6392t   sql.NullString
			)
			if err := rows.Scan(
				&l.ID, &l.Name, &l.Scope, &l.Category,
				&iso6391, &iso6392b, &iso6392t, &l.ISO6393,
			); err != nil {
				httpx.Error(w, http.StatusInternalServerError, "Internal server error")
				return
			}
			if iso6391.Valid {
				l.ISO6391 = &iso6391.String
			}
			if iso6392b.Valid {
				l.ISO6392b = &iso6392b.String
			}
			if iso6392t.Valid {
				l.ISO6392t = &iso6392t.String
			}
			result = append(result, l)
		}
		if err := rows.Err(); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		httpx.JSON(w, http.StatusOK, result)
	}
}

