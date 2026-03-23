package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"gs-api/internal/db"
	"gs-api/internal/httpx"
	"gs-api/internal/models"
)

func Pictos(sqlDB *sql.DB, imageBaseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		symbolsetSlug := r.URL.Query().Get("symbolset")
		if symbolsetSlug == "" {
			httpx.Error(w, http.StatusBadRequest, "symbolset is required")
			return
		}
		symbolsetID, status, err := db.SymbolsetBySlug(sqlDB, symbolsetSlug)
		if err != nil || !db.IsSymbolsetPublished(status) {
			httpx.Error(w, http.StatusNotFound, "Couldn't find Symbolset or not published")
			return
		}

		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			if n, err := strconv.Atoi(p); err == nil && n >= 1 {
				page = n
			}
		}
		perPage := 100
		if pp := r.URL.Query().Get("per_page"); pp != "" {
			if n, err := strconv.Atoi(pp); err == nil && n >= 1 && n <= 100 {
				perPage = n
			}
		}

		sinceStr := r.URL.Query().Get("since")
		if sinceStr == "" {
			items, total, err := db.PictosList(sqlDB, symbolsetID, page, perPage, imageBaseURL, "")
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, "Internal server error")
				return
			}
			httpx.JSON(w, http.StatusOK, models.PagedPictosResponse{Items: items, Total: total})
			return
		}

		since, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			// Try RFC3339Nano
			since, err = time.Parse(time.RFC3339Nano, sinceStr)
		}
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "Invalid 'since' timestamp. Use ISO 8601 format (e.g., 2026-01-01T00:00:00Z).")
			return
		}

		items, total, deletions, lastUpdated, err := db.PictosDelta(sqlDB, symbolsetID, since, page, perPage, imageBaseURL, "")
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		resp := models.PagedPictosResponse{
			Items: items, Total: total, Deletions: deletions,
		}
		if lastUpdated != nil {
			s := lastUpdated.UTC().Format(time.RFC3339)
			resp.LastUpdated = &s
		}
		httpx.JSON(w, http.StatusOK, resp)
	}
}
