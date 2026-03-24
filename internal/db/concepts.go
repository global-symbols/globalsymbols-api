package db

import (
	"database/sql"
	"strings"

	"gs-api/internal/models"
)

// LanguageIDByCode returns the language id for the given ISO code (column chosen by format: 639-1, 639-2b, 639-2t, 639-3).
func LanguageIDByCode(conn *sql.DB, code string, format string) (int64, error) {
	col := "iso639_3"
	switch format {
	case "639-1":
		col = "iso639_1"
	case "639-2b":
		col = "iso639_2b"
	case "639-2t":
		col = "iso639_2t"
	}
	var id int64
	err := conn.QueryRow(
		`SELECT id FROM languages WHERE `+col+` = ?`,
		code,
	).Scan(&id)
	return id, err
}

// ConceptsSuggest returns concepts matching the normalized query, ordered per spec.
// If symbolsetID > 0, only concepts with at least one picto in that symbolset, and only those pictos.
func ConceptsSuggest(conn *sql.DB, query string, symbolsetID int64, languageID int64, limit int, imageBaseURL string) ([]models.Concept, error) {
	norm := strings.ToLower(strings.TrimSpace(query))
	norm = strings.ReplaceAll(norm, " ", "_")
	if norm == "" {
		return []models.Concept{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	// Build WHERE: subject contains norm; optionally restrict to concepts with pictos in symbolset
	where := `c.subject LIKE ?`
	args := []any{"%" + norm + "%"}
	if languageID > 0 {
		where += ` AND c.language_id = ?`
		args = append(args, languageID)
	}
	if symbolsetID > 0 {
		where += ` AND EXISTS (
			SELECT 1 FROM picto_concepts pc
			JOIN pictos p ON p.id = pc.picto_id AND p.archived = 0
			WHERE pc.concept_id = c.id AND p.symbolset_id = ?
		)`
		args = append(args, symbolsetID)
	}
	args = append(args, norm, norm+"_%", "%_"+norm, "%_"+norm+"_%", norm+"%", "%"+norm+"%", limit)

	q := `
		SELECT c.id, c.subject, c.coding_framework_id, c.language_id,
		       cf.name AS cf_name, cf.structure AS cf_structure, cf.api_uri_base, cf.www_uri_base,
		       lang.name AS lang_name, lang.scope, lang.category, lang.iso639_1, lang.iso639_2b, lang.iso639_2t, lang.iso639_3
		FROM concepts c
		JOIN coding_frameworks cf ON cf.id = c.coding_framework_id
		LEFT JOIN languages lang ON lang.id = c.language_id
		WHERE ` + where + `
		ORDER BY
		  CASE WHEN c.subject = ? THEN 0
		       WHEN c.subject LIKE ? THEN 1
		       WHEN c.subject LIKE ? THEN 2
		       WHEN c.subject LIKE ? THEN 3
		       WHEN c.subject LIKE ? THEN 4
		       WHEN c.subject LIKE ? THEN 5
		       ELSE 6 END,
		  c.subject
		LIMIT ?`

	rows, err := conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Concept
	for rows.Next() {
		var c models.Concept
		var cfID, langID sql.NullInt64
		var cfStruct int
		var apiBase, wwwBase sql.NullString
		var lName, lScope, lCat, iso1, iso2b, iso2t, iso3 sql.NullString
		if err := rows.Scan(
			&c.ID, &c.Subject, &cfID, &langID,
			&c.CodingFramework.Name, &cfStruct, &apiBase, &wwwBase,
			&lName, &lScope, &lCat, &iso1, &iso2b, &iso2t, &iso3,
		); err != nil {
			return nil, err
		}
		c.CodingFramework.ID = cfID.Int64
		c.CodingFramework.Structure = CodingFrameworkStructureFromInt(cfStruct)
		if apiBase.Valid {
			c.APIURI = apiBase.String + c.Subject
		}
		if wwwBase.Valid {
			c.WWWURI = wwwBase.String + c.Subject
		}
		if lName.Valid {
			c.Language.Name = lName.String
		}
		if langID.Valid {
			c.Language.ID = langID.Int64
		}
		if lScope.Valid {
			c.Language.Scope = lScope.String
		}
		if lCat.Valid {
			c.Language.Category = lCat.String
		}
		if iso1.Valid {
			c.Language.ISO6391 = &iso1.String
		}
		if iso2b.Valid {
			c.Language.ISO6392b = &iso2b.String
		}
		if iso2t.Valid {
			c.Language.ISO6392t = &iso2t.String
		}
		if iso3.Valid {
			c.Language.ISO6393 = iso3.String
		}

		// Load pictos for this concept (filter by symbolset if set)
		pictos, cnt, err := conceptPictos(conn, c.ID, symbolsetID, imageBaseURL)
		if err != nil {
			return nil, err
		}
		c.PictosCount = cnt
		c.Pictos = pictos
		list = append(list, c)
	}
	return list, rows.Err()
}

func conceptPictos(conn *sql.DB, conceptID int64, symbolsetID int64, imageBaseURL string) ([]models.Picto, int64, error) {
	q := `
		SELECT p.id, p.symbolset_id, p.part_of_speech, i.imagefile, i.adaptable
		FROM picto_concepts pc
		JOIN pictos p ON p.id = pc.picto_id AND p.archived = 0
		LEFT JOIN images i ON i.picto_id = p.id
		WHERE pc.concept_id = ?`
	args := []any{conceptID}
	if symbolsetID > 0 {
		q += ` AND p.symbolset_id = ?`
		args = append(args, symbolsetID)
	}
	q += ` ORDER BY p.id`

	rows, err := conn.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []models.Picto
	for rows.Next() {
		var p models.Picto
		var pos int
		var imgFile sql.NullString
		var adapt sql.NullBool
		if err := rows.Scan(&p.ID, &p.SymbolsetID, &pos, &imgFile, &adapt); err != nil {
			return nil, 0, err
		}
		p.PartOfSpeech = PartOfSpeechFromInt(pos)
		if imgFile.Valid && imgFile.String != "" {
			p.ImageURL = strings.TrimSuffix(imageBaseURL, "/") + "/" + strings.TrimPrefix(imgFile.String, "/")
		} else {
			p.ImageURL = ""
		}
		p.NativeFormat = inferNativeFormatFromImageFile(imgFile)
		if adapt.Valid {
			p.Adaptable = &adapt.Bool
		}
		list = append(list, p)
	}
	return list, int64(len(list)), rows.Err()
}

// ConceptByID returns a single concept by ID with its pictos (all symbolsets).
func ConceptByID(conn *sql.DB, id int64, imageBaseURL string) (*models.Concept, error) {
	var c models.Concept
	var cfID, langID sql.NullInt64
	var cfStruct int
	var apiBase, wwwBase sql.NullString
	var lName, lScope, lCat, iso1, iso2b, iso2t, iso3 sql.NullString
	err := conn.QueryRow(`
		SELECT c.id, c.subject, c.coding_framework_id, c.language_id,
		       cf.name, cf.structure, cf.api_uri_base, cf.www_uri_base,
		       lang.name, lang.scope, lang.category, lang.iso639_1, lang.iso639_2b, lang.iso639_2t, lang.iso639_3
		FROM concepts c
		JOIN coding_frameworks cf ON cf.id = c.coding_framework_id
		LEFT JOIN languages lang ON lang.id = c.language_id
		WHERE c.id = ?`, id,
	).Scan(
		&c.ID, &c.Subject, &cfID, &langID,
		&c.CodingFramework.Name, &cfStruct, &apiBase, &wwwBase,
		&lName, &lScope, &lCat, &iso1, &iso2b, &iso2t, &iso3,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.CodingFramework.ID = cfID.Int64
	c.CodingFramework.Structure = CodingFrameworkStructureFromInt(cfStruct)
	if apiBase.Valid {
		c.APIURI = apiBase.String + c.Subject
	}
	if wwwBase.Valid {
		c.WWWURI = wwwBase.String + c.Subject
	}
	if lName.Valid {
		c.Language.Name = lName.String
	}
	if langID.Valid {
		c.Language.ID = langID.Int64
	}
	if lScope.Valid {
		c.Language.Scope = lScope.String
	}
	if lCat.Valid {
		c.Language.Category = lCat.String
	}
	if iso1.Valid {
		c.Language.ISO6391 = &iso1.String
	}
	if iso2b.Valid {
		c.Language.ISO6392b = &iso2b.String
	}
	if iso2t.Valid {
		c.Language.ISO6392t = &iso2t.String
	}
	if iso3.Valid {
		c.Language.ISO6393 = iso3.String
	}

	pictos, cnt, err := conceptPictos(conn, c.ID, 0, imageBaseURL)
	if err != nil {
		return nil, err
	}
	c.PictosCount = cnt
	c.Pictos = pictos
	return &c, nil
}
