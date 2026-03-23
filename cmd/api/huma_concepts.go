package main

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"gs-api/internal/config"
	"gs-api/internal/db"
	"gs-api/internal/models"
	"gs-api/internal/previews"
)

// registerHumaOperations registers Huma-backed operations that will be included
// in the generated OpenAPI spec and Scalar docs.
func registerHumaOperations(api huma.API, sqlDB *sql.DB, cfg *config.Config) {
	registerConceptByID(api, sqlDB, cfg)
	registerConceptsSuggest(api, sqlDB, cfg)
	registerLabelsSearch(api, sqlDB, cfg)
	registerLabelByID(api, sqlDB, cfg)
	registerLanguagesActive(api, sqlDB)
	registerSymbolsets(api, sqlDB, cfg)
	registerPictos(api, sqlDB, cfg)
}

func withAPIKeyAuth(op huma.Operation) huma.Operation {
	op.Security = []map[string][]string{
		{"ApiKeyAuth": {}},
	}
	return op
}

// conceptByIDInput models the path parameters for the concept-by-ID operation.
type conceptByIDInput struct {
	ID int64 `path:"id" doc:"Concept ID"`
}

// conceptByIDOutput wraps the response body so Huma can describe it in OpenAPI.
type conceptByIDOutput struct {
	Body models.Concept
}

func registerConceptByID(api huma.API, sqlDB *sql.DB, cfg *config.Config) {
	huma.Register(api, withAPIKeyAuth(huma.Operation{
		OperationID: "get-concept-by-id",
		Method:      http.MethodGet,
		Path:        "/concepts/{id}",
		Summary:     "Get concept by ID",
		Responses: apiKeyProtectedResponses(map[int]string{
			http.StatusBadRequest: "Invalid concept ID.",
			http.StatusNotFound:   "Concept not found.",
		}),
	}), func(ctx context.Context, input *conceptByIDInput) (*conceptByIDOutput, error) {
		concept, err := db.ConceptByID(sqlDB, input.ID, cfg.ImageBaseURL)
		if err != nil {
			return nil, huma.Error500InternalServerError("Internal server error", err)
		}
		if concept == nil {
			return nil, huma.Error404NotFound("Couldn't find Concept with id " + strconv.FormatInt(input.ID, 10))
		}

		return &conceptByIDOutput{Body: *concept}, nil
	})
}

// conceptsSuggestInput models the query parameters for concepts suggestion.
type conceptsSuggestInput struct {
	Query             string `query:"query" doc:"Search query (required)"`
	SymbolsetSlug     string `query:"symbolset" doc:"Optional symbolset slug to filter concepts"`
	Language          string `query:"language" doc:"Optional language code"`
	LanguageISOFormat string `query:"language_iso_format" doc:"Language ISO format: 639-1, 639-2b, 639-2t, or 639-3 (default: 639-3)"`
	Limit             int    `query:"limit" doc:"Maximum number of results to return (1-100, default 10)"`
}

type conceptsSuggestOutput struct {
	Body []models.Concept
}

func registerConceptsSuggest(api huma.API, sqlDB *sql.DB, cfg *config.Config) {
	huma.Register(api, withAPIKeyAuth(huma.Operation{
		OperationID: "get-concepts-suggest",
		Method:      http.MethodGet,
		Path:        "/concepts/suggest",
		Summary:     "Suggest concepts by query",
		Description: "Limit handling note: Out-of-range limit values are normalized to 10 (not rejected). The request then follows the normal success path.",
		Responses: apiKeyProtectedResponses(map[int]string{
			http.StatusBadRequest: "Missing or invalid query parameters.",
		}),
	}), func(ctx context.Context, input *conceptsSuggestInput) (*conceptsSuggestOutput, error) {
		if input.Query == "" {
			return nil, newAPIErrorNoCode(http.StatusBadRequest, "query is missing")
		}

		limit := input.Limit
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		language := input.Language
		if language == "" {
			language = "eng"
		}
		languageISOFormat := input.LanguageISOFormat
		if languageISOFormat == "" {
			languageISOFormat = "639-3"
		}

		var symbolsetID int64
		if input.SymbolsetSlug != "" {
			id, status, err := db.SymbolsetBySlug(sqlDB, input.SymbolsetSlug)
			if err != nil || !db.IsSymbolsetPublished(status) {
				return nil, huma.Error400BadRequest("symbolset is not published or not found", err)
			}
			symbolsetID = id
		}

		languageID, err := db.LanguageIDByCode(sqlDB, language, languageISOFormat)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid language or language_iso_format", err)
		}

		list, err := db.ConceptsSuggest(sqlDB, input.Query, symbolsetID, languageID, limit, cfg.ImageBaseURL)
		if err != nil {
			return nil, huma.Error500InternalServerError("Internal server error", err)
		}
		return &conceptsSuggestOutput{Body: list}, nil
	})
}

// labelsSearchInput models the query parameters for label search.
type labelsSearchInput struct {
	Query             string `query:"query" doc:"Search query (required)"`
	SymbolsetSlug     string `query:"symbolset" doc:"Optional symbolset slug to filter labels"`
	Language          string `query:"language" doc:"Optional language code to filter labels (default: eng)"`
	LanguageISOFormat string `query:"language_iso_format" doc:"Language ISO format (default: 639-3)"`
	Limit             int    `query:"limit" doc:"Maximum number of results to return (1-100, default 10)"`
	IncludePreview    bool   `query:"include_preview" doc:"When true, include an inline 64x64 PNG preview_data_url for each picto"`
}

type labelsSearchOutput struct {
	Body []models.LabelSearchResult
}

func registerLabelsSearch(api huma.API, sqlDB *sql.DB, cfg *config.Config) {
	huma.Register(api, withAPIKeyAuth(huma.Operation{
		OperationID: "get-labels-search",
		Method:      http.MethodGet,
		Path:        "/labels/search",
		Summary:     "Search labels by query",
		Description: "Limit handling note: Out-of-range limit values are normalized to 10 (not rejected). The request then follows the normal success path.",
		Responses: apiKeyProtectedResponses(map[int]string{
			http.StatusBadRequest: "Missing or invalid query parameters.",
		}),
	}), func(ctx context.Context, input *labelsSearchInput) (*labelsSearchOutput, error) {
		if input.Query == "" {
			return nil, newAPIErrorNoCode(http.StatusBadRequest, "query is missing")
		}

		limit := input.Limit
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		var symbolsetID int64
		if input.SymbolsetSlug != "" {
			id, status, err := db.SymbolsetBySlug(sqlDB, input.SymbolsetSlug)
			if err != nil || !db.IsSymbolsetPublished(status) {
				return nil, huma.Error400BadRequest("symbolset is not published or not found", err)
			}
			symbolsetID = id
		}

		language := input.Language
		if language == "" {
			language = "eng"
		}
		languageISOFormat := input.LanguageISOFormat
		if languageISOFormat == "" {
			languageISOFormat = "639-3"
		}
		if _, err := db.LanguageIDByCode(sqlDB, language, languageISOFormat); err != nil {
			return nil, huma.Error400BadRequest("invalid language or language_iso_format", err)
		}

		list, err := db.LabelsSearch(sqlDB, input.Query, symbolsetID, language, languageISOFormat, limit, cfg.ImageBaseURL, cfg.AppEnv)
		if err != nil {
			return nil, huma.Error500InternalServerError("Internal server error", err)
		}
		body := labelSearchResults(list)
		if input.IncludePreview {
			previewCtx, cancel := context.WithTimeout(ctx, previews.DefaultPictoPreviewTimeout)
			defer cancel()
			previews.PopulateLabelSearchPreviewDataURLs(previewCtx, body, previews.DefaultPictoPreviewSize, previews.DefaultPictoPreviewWorkers)
		}
		return &labelsSearchOutput{Body: body}, nil
	})
}

func labelSearchResults(items []models.Label) []models.LabelSearchResult {
	out := make([]models.LabelSearchResult, 0, len(items))
	for _, item := range items {
		out = append(out, models.LabelSearchResult{
			ID:              item.ID,
			Text:            item.Text,
			TextDiacritised: item.TextDiacritised,
			Description:     item.Description,
			Language:        item.Language,
			Picto: models.LabelSearchPicto{
				ID:           item.Picto.ID,
				SymbolsetID:  item.Picto.SymbolsetID,
				PartOfSpeech: item.Picto.PartOfSpeech,
				ImageURL:     item.Picto.ImageURL,
				NativeFormat: item.Picto.NativeFormat,
				Adaptable:    item.Picto.Adaptable,
				Symbolset:    item.Picto.Symbolset,
			},
		})
	}
	return out
}

// labelByIDInput models the path parameter for label lookup.
type labelByIDInput struct {
	ID int64 `path:"id" doc:"Label ID"`
}

type labelByIDOutput struct {
	Body models.Label
}

func registerLabelByID(api huma.API, sqlDB *sql.DB, cfg *config.Config) {
	huma.Register(api, withAPIKeyAuth(huma.Operation{
		OperationID: "get-label-by-id",
		Method:      http.MethodGet,
		Path:        "/labels/{id}",
		Summary:     "Get label by ID",
		Responses: apiKeyProtectedResponses(map[int]string{
			http.StatusBadRequest: "Invalid label ID.",
			http.StatusNotFound:   "Label not found.",
		}),
	}), func(ctx context.Context, input *labelByIDInput) (*labelByIDOutput, error) {
		lab, err := db.LabelByID(sqlDB, input.ID, cfg.ImageBaseURL)
		if err != nil {
			return nil, huma.Error500InternalServerError("Internal server error", err)
		}
		if lab == nil {
			return nil, huma.Error404NotFound("Couldn't find Label with id " + strconv.FormatInt(input.ID, 10))
		}
		return &labelByIDOutput{Body: *lab}, nil
	})
}

type languagesActiveOutput struct {
	Body []models.Language
}

func registerLanguagesActive(api huma.API, sqlDB *sql.DB) {
	huma.Register(api, withAPIKeyAuth(huma.Operation{
		OperationID: "get-languages-active",
		Method:      http.MethodGet,
		Path:        "/languages/active",
		Summary:     "List active languages",
		Responses:   apiKeyProtectedResponses(nil),
	}), func(ctx context.Context, _ *struct{}) (*languagesActiveOutput, error) {
		rows, err := sqlDB.Query(`
			SELECT id, name, scope, category, iso639_1, iso639_2b, iso639_2t, iso639_3
			FROM languages
			WHERE active = 1
			ORDER BY name`,
		)
		if err != nil {
			return nil, huma.Error500InternalServerError("Internal server error", err)
		}
		defer rows.Close()

		var result []models.Language
		for rows.Next() {
			var (
				l        models.Language
				iso6391  sql.NullString
				iso6392b sql.NullString
				iso6392t sql.NullString
			)
			if err := rows.Scan(
				&l.ID, &l.Name, &l.Scope, &l.Category,
				&iso6391, &iso6392b, &iso6392t, &l.ISO6393,
			); err != nil {
				return nil, huma.Error500InternalServerError("Internal server error", err)
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
			return nil, huma.Error500InternalServerError("Internal server error", err)
		}
		return &languagesActiveOutput{Body: result}, nil
	})
}

type symbolsetsOutput struct {
	Body []models.Symbolset
}

func registerSymbolsets(api huma.API, sqlDB *sql.DB, cfg *config.Config) {
	huma.Register(api, withAPIKeyAuth(huma.Operation{
		OperationID: "get-symbolsets",
		Method:      http.MethodGet,
		Path:        "/symbolsets",
		Summary:     "List published symbolsets",
		Responses:   apiKeyProtectedResponses(nil),
	}), func(ctx context.Context, _ *struct{}) (*symbolsetsOutput, error) {
		list, err := db.ListPublished(sqlDB, cfg.ImageBaseURL, cfg.AppEnv)
		if err != nil {
			return nil, huma.Error500InternalServerError("Internal server error", err)
		}
		return &symbolsetsOutput{Body: list}, nil
	})
}

// pictosInput models query parameters for the pictos listing endpoint.
type pictosInput struct {
	SymbolsetSlug string `query:"symbolset" doc:"Symbolset slug (required)"`
	Page          int    `query:"page" doc:"Page number (>=1, default 1)"`
	PerPage       int    `query:"per_page" doc:"Items per page (1-100, default 100)"`
	Since         string `query:"since" doc:"Optional ISO 8601 timestamp to request delta updates"`
}

type pictosOutput struct {
	Body models.PagedPictosResponse
}

func registerPictos(api huma.API, sqlDB *sql.DB, cfg *config.Config) {
	huma.Register(api, withAPIKeyAuth(huma.Operation{
		OperationID: "get-pictos",
		Method:      http.MethodGet,
		Path:        "/pictos",
		Summary:     "List pictos for a symbolset",
		Responses: apiKeyProtectedResponses(map[int]string{
			http.StatusBadRequest: "Missing or invalid query parameters.",
			http.StatusNotFound:   "Symbolset not found or not published.",
		}),
	}), func(ctx context.Context, input *pictosInput) (*pictosOutput, error) {
		if input.SymbolsetSlug == "" {
			return nil, huma.Error400BadRequest("symbolset is required")
		}
		symbolsetID, status, err := db.SymbolsetBySlug(sqlDB, input.SymbolsetSlug)
		if err != nil || !db.IsSymbolsetPublished(status) {
			return nil, huma.Error404NotFound("Couldn't find Symbolset or not published")
		}

		page := input.Page
		if page < 1 {
			page = 1
		}
		perPage := input.PerPage
		if perPage < 1 || perPage > 100 {
			perPage = 100
		}

		if input.Since == "" {
			items, total, err := db.PictosList(sqlDB, symbolsetID, page, perPage, cfg.ImageBaseURL, cfg.AppEnv)
			if err != nil {
				return nil, huma.Error500InternalServerError("Internal server error", err)
			}
			return &pictosOutput{
				Body: models.PagedPictosResponse{
					Items: items,
					Total: total,
				},
			}, nil
		}

		since, err := time.Parse(time.RFC3339, input.Since)
		if err != nil {
			// Try RFC3339Nano
			since, err = time.Parse(time.RFC3339Nano, input.Since)
		}
		if err != nil {
			return nil, huma.Error400BadRequest("Invalid 'since' timestamp. Use ISO 8601 format (e.g., 2026-01-01T00:00:00Z).", err)
		}

		items, total, deletions, lastUpdated, err := db.PictosDelta(sqlDB, symbolsetID, since, page, perPage, cfg.ImageBaseURL, cfg.AppEnv)
		if err != nil {
			return nil, huma.Error500InternalServerError("Internal server error", err)
		}

		resp := models.PagedPictosResponse{
			Items:     items,
			Total:     total,
			Deletions: deletions,
		}
		if lastUpdated != nil {
			s := lastUpdated.UTC().Format(time.RFC3339)
			resp.LastUpdated = &s
		}

		return &pictosOutput{Body: resp}, nil
	})
}
