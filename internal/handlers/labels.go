package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gs-api/internal/db"
	"gs-api/internal/httpx"
)

func LabelsSearch(sqlDB *sql.DB, imageBaseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == "" {
			httpx.Error(w, http.StatusBadRequest, "query is required")
			return
		}
		symbolsetSlug := r.URL.Query().Get("symbolset")
		language := r.URL.Query().Get("language")
		languageISOFormat := r.URL.Query().Get("language_iso_format")
		if languageISOFormat == "" {
			languageISOFormat = "639-3"
		}
		limit := 10
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n >= 1 && n <= 100 {
				limit = n
			}
		}

		var symbolsetID int64
		if symbolsetSlug != "" {
			id, status, err := db.SymbolsetBySlug(sqlDB, symbolsetSlug)
			if err != nil || !db.IsSymbolsetPublished(status) {
				httpx.Error(w, http.StatusBadRequest, "symbolset is not published or not found")
				return
			}
			symbolsetID = id
		}

		// Validate language parameters (Rails filters by iso639_* rather than language_id).
		if language != "" {
			if _, err := db.LanguageIDByCode(sqlDB, language, languageISOFormat); err != nil {
				httpx.Error(w, http.StatusBadRequest, "invalid language or language_iso_format")
				return
			}
		}

		list, err := db.LabelsSearch(sqlDB, query, symbolsetID, language, languageISOFormat, limit, imageBaseURL)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		httpx.JSON(w, http.StatusOK, list)
	}
}

func LabelByID(sqlDB *sql.DB, imageBaseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			httpx.Error(w, http.StatusNotFound, "Couldn't find Label with id "+idStr)
			return
		}
		lab, err := db.LabelByID(sqlDB, id, imageBaseURL)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		if lab == nil {
			httpx.Error(w, http.StatusNotFound, "Couldn't find Label with id "+idStr)
			return
		}
		httpx.JSON(w, http.StatusOK, lab)
	}
}
