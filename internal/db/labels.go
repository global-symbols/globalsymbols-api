package db

import (
	"database/sql"
	"fmt"
	"strings"

	"gs-api/internal/models"
)

func imageURL(imageBaseURL, appEnv string, imageID int64, stored sql.NullString) string {
	if !stored.Valid || strings.TrimSpace(stored.String) == "" || imageBaseURL == "" {
		return ""
	}
	s := strings.TrimSpace(stored.String)
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return s
	}
	base := strings.TrimSuffix(imageBaseURL, "/")
	rel := strings.TrimPrefix(s, "/")
	if strings.HasPrefix(rel, "uploads/") {
		return base + "/" + rel
	}
	if !strings.Contains(s, "/") && imageID > 0 && appEnv != "" {
		return fmt.Sprintf("%s/uploads/%s/image/imagefile/%d/%s", base, appEnv, imageID, s)
	}
	return base + "/" + rel
}

// LabelsSearch returns authoritative labels matching the query (non-archived pictos, published symbolset, visibility everybody).
// If symbolsetID > 0, restrict to that symbolset.
// Order: exact, underscore-boundary variants, prefix, then by text.
//
// Note: Rails filters by languages.iso639_* rather than languages.id, and that can impact which rows
// win when multiple labels have identical ORDER BY expressions under LIMIT.
func LabelsSearch(conn *sql.DB, query string, symbolsetID int64, languageCode string, languageISOFormat string, limit int, imageBaseURL, appEnv string) ([]models.Label, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return []models.Label{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	norm := strings.ToLower(q)
	norm = strings.ReplaceAll(norm, " ", "_")
	langColumn := isoColumnForFormat(languageISOFormat)
	if langColumn == "" {
		langColumn = "iso639_3"
	}

	whereParts := []string{
		"LOWER(l.text) LIKE ?",
		"p.visibility = 0",
		fmt.Sprintf("lang.%s = ?", langColumn),
	}
	args := []any{"%" + norm + "%", languageCode}
	if symbolsetID > 0 {
		whereParts = append(whereParts, "p.symbolset_id = ?")
		args = append(args, symbolsetID)
	}

	sqlQuery := `
		SELECT l.id, l.text, l.text_diacritised, l.description, lang.iso639_3,
		       p.id, p.symbolset_id, p.part_of_speech, i.id, i.imagefile, i.adaptable
		FROM labels l
		JOIN sources s ON s.id = l.source_id AND s.authoritative = 1
		JOIN languages lang ON lang.id = l.language_id
		JOIN pictos p ON p.id = l.picto_id AND p.archived = 0
		JOIN symbolsets ss ON ss.id = p.symbolset_id AND ss.status = 0
		LEFT JOIN images i ON i.picto_id = p.id
		WHERE ` + strings.Join(whereParts, " AND ") + `
		ORDER BY
		  CASE WHEN LOWER(l.text) = ? THEN 0
		       WHEN LOWER(l.text) LIKE ? THEN 10
		       WHEN LOWER(l.text) LIKE ? THEN 20
		       WHEN LOWER(l.text) LIKE ? THEN 30
		       WHEN LOWER(l.text) LIKE ? THEN 40
		       WHEN LOWER(l.text) LIKE ? THEN 50
		    ELSE 60
		  END,
		  l.text
		LIMIT ?`

	args = append(args,
		norm,
		norm+"\\_%",
		"%\\_"+norm,
		"%\\_"+norm+"\\_%",
		norm+"%",
		"%"+norm,
		limit,
	)

	rows, err := conn.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Label
	for rows.Next() {
		var lab models.Label
		var textDiac, desc sql.NullString
		var pos int
		var imageID sql.NullInt64
		var imgFile sql.NullString
		var adapt sql.NullBool
		if err := rows.Scan(
			&lab.ID,
			&lab.Text,
			&textDiac,
			&desc,
			&lab.Language,
			&lab.Picto.ID,
			&lab.Picto.SymbolsetID,
			&pos,
			&imageID,
			&imgFile,
			&adapt,
		); err != nil {
			return nil, err
		}

		if textDiac.Valid {
			lab.TextDiacritised = &textDiac.String
		}
		if desc.Valid {
			lab.Description = &desc.String
		}
		lab.Picto.PartOfSpeech = PartOfSpeechFromInt(pos)
		if imageID.Valid {
			lab.Picto.ImageURL = imageURL(imageBaseURL, appEnv, imageID.Int64, imgFile)
		}
		lab.Picto.NativeFormat = "png"
		if adapt.Valid {
			lab.Picto.Adaptable = &adapt.Bool
		}

		list = append(list, lab)
	}
	return list, rows.Err()
}

func isoColumnForFormat(languageISOFormat string) string {
	switch strings.TrimSpace(languageISOFormat) {
	case "639-1":
		return "iso639_1"
	case "639-2b":
		return "iso639_2b"
	case "639-2t":
		return "iso639_2t"
	case "639-3", "":
		return "iso639_3"
	default:
		return ""
	}
}

// LabelByID returns a single label by ID (authoritative, non-archived picto, published symbolset).
func LabelByID(conn *sql.DB, id int64, imageBaseURL string) (*models.Label, error) {
	var lab models.Label
	var textDiac, desc sql.NullString
	var pos int
	var imgFile sql.NullString
	var adapt sql.NullBool
	err := conn.QueryRow(`
		SELECT l.id, l.text, l.text_diacritised, l.description, lang.iso639_3,
		       p.id, p.symbolset_id, p.part_of_speech, i.imagefile, i.adaptable
		FROM labels l
		JOIN sources s ON s.id = l.source_id AND s.authoritative = 1
		JOIN languages lang ON lang.id = l.language_id
		JOIN pictos p ON p.id = l.picto_id AND p.archived = 0
		JOIN symbolsets ss ON ss.id = p.symbolset_id AND ss.status = 0
		LEFT JOIN images i ON i.picto_id = p.id
		WHERE l.id = ? AND p.visibility = 0`, id,
	).Scan(
		&lab.ID, &lab.Text, &textDiac, &desc, &lab.Language,
		&lab.Picto.ID, &lab.Picto.SymbolsetID, &pos, &imgFile, &adapt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if textDiac.Valid {
		lab.TextDiacritised = &textDiac.String
	}
	if desc.Valid {
		lab.Description = &desc.String
	}
	lab.Picto.PartOfSpeech = PartOfSpeechFromInt(pos)
	if imgFile.Valid && imgFile.String != "" {
		lab.Picto.ImageURL = strings.TrimSuffix(imageBaseURL, "/") + "/" + strings.TrimPrefix(imgFile.String, "/")
	}
	lab.Picto.NativeFormat = "png"
	if adapt.Valid {
		lab.Picto.Adaptable = &adapt.Bool
	}
	return &lab, nil
}
