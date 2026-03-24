package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gs-api/internal/models"
)

func pictoImageURL(imageBaseURL, appEnv string, imageID int64, stored sql.NullString) string {
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

// PictosList lists non-archived, visible pictos for a symbolset with pagination.
// Returns total count and page of PictoSummary with authoritative labels.
func PictosList(conn *sql.DB, symbolsetID int64, page, perPage int, imageBaseURL, appEnv string) (items []models.PictoSummary, total int64, err error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 100
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * perPage

	if err := conn.QueryRow(`
		SELECT COUNT(*) FROM pictos p
		WHERE p.symbolset_id = ? AND p.archived = 0 AND p.visibility = 0`,
		symbolsetID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := conn.Query(`
		SELECT p.id, p.part_of_speech, i.id, i.imagefile
		FROM pictos p
		LEFT JOIN images i ON i.picto_id = p.id
		WHERE p.symbolset_id = ? AND p.archived = 0 AND p.visibility = 0
		ORDER BY p.id
		LIMIT ? OFFSET ?`,
		symbolsetID, perPage, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var p models.PictoSummary
		var pos int
		var imageID sql.NullInt64
		var imgFile sql.NullString
		if err := rows.Scan(&p.ID, &pos, &imageID, &imgFile); err != nil {
			return nil, 0, err
		}
		p.PartOfSpeech = PartOfSpeechFromInt(pos)
		if imageID.Valid {
			p.ImageURL = pictoImageURL(imageBaseURL, appEnv, imageID.Int64, imgFile)
		}
		p.NativeFormat = inferNativeFormatFromImageFile(imgFile)
		labels, err := pictoLabels(conn, p.ID)
		if err != nil {
			return nil, 0, err
		}
		p.Labels = labels
		items = append(items, p)
	}
	return items, total, rows.Err()
}

func pictoLabels(conn *sql.DB, pictoID int64) ([]models.LabelSummary, error) {
	rows, err := conn.Query(`
		SELECT l.text, l.text_diacritised, lang.iso639_3
		FROM labels l
		JOIN sources s ON s.id = l.source_id AND s.authoritative = 1
		JOIN languages lang ON lang.id = l.language_id
		WHERE l.picto_id = ?`,
		pictoID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []models.LabelSummary
	for rows.Next() {
		var lab models.LabelSummary
		var diac sql.NullString
		if err := rows.Scan(&lab.Text, &diac, &lab.Language); err != nil {
			return nil, err
		}
		if diac.Valid {
			lab.TextDiacritised = &diac.String
		}
		list = append(list, lab)
	}
	return list, rows.Err()
}

// PictosDelta returns pictos updated after since, deletions (archived since), and last_updated.
func PictosDelta(conn *sql.DB, symbolsetID int64, since time.Time, page, perPage int, imageBaseURL, appEnv string) (items []models.PictoSummary, total int64, deletions []int64, lastUpdated *time.Time, err error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 100
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * perPage

	// Count: pictos that were updated after since (including archived)
	if err := conn.QueryRow(`
		SELECT COUNT(*) FROM pictos WHERE symbolset_id = ? AND updated_at > ?`,
		symbolsetID, since,
	).Scan(&total); err != nil {
		return nil, 0, nil, nil, err
	}

	// Deletions: ids of pictos archived after since
	delRows, err := conn.Query(`
		SELECT id FROM pictos WHERE symbolset_id = ? AND archived = 1 AND updated_at > ?`,
		symbolsetID, since,
	)
	if err != nil {
		return nil, 0, nil, nil, err
	}
	defer delRows.Close()
	for delRows.Next() {
		var id int64
		if err := delRows.Scan(&id); err != nil {
			return nil, 0, nil, nil, err
		}
		deletions = append(deletions, id)
	}
	if err := delRows.Err(); err != nil {
		return nil, 0, nil, nil, err
	}

	// Last updated
	var lu sql.NullTime
	if err := conn.QueryRow(`
		SELECT MAX(updated_at) FROM pictos WHERE symbolset_id = ? AND updated_at > ?`,
		symbolsetID, since,
	).Scan(&lu); err != nil {
		return nil, 0, nil, nil, err
	}
	if lu.Valid {
		lastUpdated = &lu.Time
	}

	// Items: non-archived, visible pictos updated after since
	rows, err := conn.Query(`
		SELECT p.id, p.part_of_speech, i.id, i.imagefile
		FROM pictos p
		LEFT JOIN images i ON i.picto_id = p.id
		WHERE p.symbolset_id = ? AND p.archived = 0 AND p.visibility = 0 AND p.updated_at > ?
		ORDER BY p.id
		LIMIT ? OFFSET ?`,
		symbolsetID, since, perPage, offset,
	)
	if err != nil {
		return nil, 0, nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p models.PictoSummary
		var pos int
		var imageID sql.NullInt64
		var imgFile sql.NullString
		if err := rows.Scan(&p.ID, &pos, &imageID, &imgFile); err != nil {
			return nil, 0, nil, nil, err
		}
		p.PartOfSpeech = PartOfSpeechFromInt(pos)
		if imageID.Valid {
			p.ImageURL = pictoImageURL(imageBaseURL, appEnv, imageID.Int64, imgFile)
		}
		p.NativeFormat = inferNativeFormatFromImageFile(imgFile)
		labels, err := pictoLabels(conn, p.ID)
		if err != nil {
			return nil, 0, nil, nil, err
		}
		p.Labels = labels
		items = append(items, p)
	}
	return items, total, deletions, lastUpdated, rows.Err()
}
