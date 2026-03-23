package handlers

import (
	"database/sql"
	"net/http"

	"gs-api/internal/db"
	"gs-api/internal/httpx"
)

func Symbolsets(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := db.ListPublished(conn, "", "")
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		httpx.JSON(w, http.StatusOK, list)
	}
}
