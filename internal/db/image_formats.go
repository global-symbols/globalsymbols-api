package db

import (
	"database/sql"
	"path"
	"strings"
)

const defaultNativeFormat = "png"

func inferNativeFormatFromImageFile(stored sql.NullString) string {
	if !stored.Valid {
		return defaultNativeFormat
	}

	ext := strings.ToLower(strings.TrimPrefix(path.Ext(strings.TrimSpace(stored.String)), "."))
	switch ext {
	case "jpg":
		return "jpeg"
	case "jpeg", "png", "svg", "gif", "webp":
		return ext
	default:
		return defaultNativeFormat
	}
}
