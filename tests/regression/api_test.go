// Package regression runs contract/behavior tests against the Go API only.
// Set GO_API_BASE_URL (e.g. http://localhost:8080) and TEST_API_KEY to run against a live server.
// If unset, tests are skipped.
package regression

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"gs-api/internal/models"
)

var (
	baseURL = os.Getenv("GO_API_BASE_URL")
	apiKey  = os.Getenv("TEST_API_KEY")
)

func skipIfNoEnv(t *testing.T) {
	if baseURL == "" || apiKey == "" {
		t.Skip("Set GO_API_BASE_URL and TEST_API_KEY to run regression tests")
	}
}

func get(t *testing.T, path string, withKey bool) (int, []byte) {
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if withKey {
		req.Header.Set("X-Api-Key", apiKey)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func getJSONCode(t *testing.T, body []byte) float64 {
	t.Helper()

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("JSON body: %v", err)
	}
	raw, ok := m["code"]
	if !ok {
		t.Fatalf("missing 'code' in body: %s", string(body))
	}

	code, ok := raw.(float64)
	if !ok {
		t.Fatalf("expected 'code' as number, got %T (value=%v)", raw, raw)
	}
	return code
}

func getJSONError(t *testing.T, body []byte) string {
	t.Helper()

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("JSON body: %v", err)
	}
	raw, ok := m["error"]
	if !ok {
		t.Fatalf("missing 'error' in body: %s", string(body))
	}

	msg, ok := raw.(string)
	if !ok {
		t.Fatalf("expected 'error' as string, got %T (value=%v)", raw, raw)
	}
	return msg
}

func decodeJSON[T any](t *testing.T, body []byte) T {
	t.Helper()

	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("JSON unmarshal: %v\nbody=%s", err, string(body))
	}
	return out
}

func TestAuth_NoKey_401(t *testing.T) {
	skipIfNoEnv(t)
	code, body := get(t, "/api/v1/languages/active", false)
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401 without key, got %d", code)
	}
	if got := getJSONCode(t, body); got != float64(401) {
		t.Errorf("expected code 401 in body, got %v", got)
	}
}

func TestAuth_InvalidKey_401(t *testing.T) {
	skipIfNoEnv(t)
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/languages/active", nil)
	req.Header.Set("X-Api-Key", "invalid-key")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 with invalid key, got %d body=%s", resp.StatusCode, string(body))
	}

	if got := getJSONCode(t, body); got != float64(401) {
		t.Errorf("expected code 401 in body, got %v", got)
	}
}

func TestAuth_ValidKey_200(t *testing.T) {
	skipIfNoEnv(t)
	code, body := get(t, "/api/v1/languages/active", true)
	if code != http.StatusOK {
		t.Errorf("expected 200 with valid key, got %d body %s", code, string(body))
	}

	// Ensure the successful response is a JSON array of languages with required fields.
	arr := decodeJSON[[]models.Language](t, body)
	if len(arr) == 0 {
		t.Fatalf("expected non-empty languages array")
	}
	for i, lang := range arr {
		if lang.ID == 0 {
			t.Errorf("item %d: expected id > 0", i)
		}
		if lang.Name == "" {
			t.Errorf("item %d: expected name non-empty", i)
		}
		if lang.ISO6393 == "" {
			t.Errorf("item %d: expected iso639_3 non-empty", i)
		}
	}
}

func TestValidation_PictosInvalidSince_400(t *testing.T) {
	skipIfNoEnv(t)
	code, body := get(t, "/api/v1/pictos?symbolset=arasaac&since=not-a-date", true)
	if code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid since, got %d", code)
	}
	if got := getJSONCode(t, body); got != float64(400) {
		t.Errorf("expected code 400, got %v", got)
	}
}

func TestValidation_ConceptsMissingQuery_400(t *testing.T) {
	skipIfNoEnv(t)
	code, body := get(t, "/api/v1/concepts/suggest", true)
	if code != http.StatusBadRequest {
		t.Errorf("expected 400 when query missing, got %d", code)
	}
	if got := getJSONError(t, body); got != "query is missing" {
		t.Errorf("expected error %q in body, got %q", "query is missing", got)
	}
}

func TestValidation_LabelsMissingQuery_400(t *testing.T) {
	skipIfNoEnv(t)
	code, body := get(t, "/api/v1/labels/search", true)
	if code != http.StatusBadRequest {
		t.Errorf("expected 400 when query missing, got %d", code)
	}
	if got := getJSONError(t, body); got != "query is missing" {
		t.Errorf("expected error %q in body, got %q", "query is missing", got)
	}
}

func TestHappy_LanguagesActive(t *testing.T) {
	skipIfNoEnv(t)
	code, body := get(t, "/api/v1/languages/active", true)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	arr := decodeJSON[[]models.Language](t, body)
	if len(arr) == 0 {
		t.Fatalf("expected non-empty languages array")
	}

	for i, lang := range arr {
		if lang.ID == 0 {
			t.Errorf("item %d: expected id > 0", i)
		}
		if lang.Name == "" {
			t.Errorf("item %d: expected name non-empty", i)
		}
		if lang.ISO6393 == "" {
			t.Errorf("item %d: expected iso639_3 non-empty", i)
		}
	}
}

func TestHappy_Symbolsets(t *testing.T) {
	skipIfNoEnv(t)
	code, body := get(t, "/api/v1/symbolsets", true)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}

	arr := decodeJSON[[]models.Symbolset](t, body)
	if len(arr) == 0 {
		t.Fatalf("expected non-empty symbolsets array")
	}

	for i, s := range arr {
		if s.ID == 0 {
			t.Errorf("item %d: expected id > 0", i)
		}
		if s.Slug == "" {
			t.Errorf("item %d: expected slug non-empty", i)
		}
		if s.Status == "" {
			t.Errorf("item %d: expected status non-empty", i)
		}
	}
}

func TestHappy_Pictos(t *testing.T) {
	skipIfNoEnv(t)
	code, body := get(t, "/api/v1/pictos?symbolset=arasaac&per_page=2", true)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", code, string(body))
	}

	resp := decodeJSON[models.PagedPictosResponse](t, body)
	if resp.Total < 0 {
		t.Fatalf("expected total >= 0, got %d", resp.Total)
	}
	if len(resp.Items) == 0 {
		t.Fatalf("expected non-empty pictos.items")
	}
	if resp.Items == nil {
		t.Fatalf("expected items slice not nil")
	}
	if len(resp.Items) > 2 {
		t.Fatalf("expected <= 2 items for per_page=2, got %d", len(resp.Items))
	}

	for i, p := range resp.Items {
		if p.ID == 0 {
			t.Errorf("item %d: expected id > 0", i)
		}
		if p.PartOfSpeech == "" {
			t.Errorf("item %d: expected part_of_speech non-empty", i)
		}
		if p.ImageURL == "" {
			t.Errorf("item %d: expected image_url non-empty", i)
		}
		if p.NativeFormat == "" {
			t.Errorf("item %d: expected native_format non-empty", i)
		}
		// If labels are present, ensure each has required fields.
		for j, lab := range p.Labels {
			if lab.Language == "" {
				t.Errorf("item %d: labels[%d]: expected language non-empty", i, j)
			}
			if lab.Text == "" {
				t.Errorf("item %d: labels[%d]: expected text non-empty", i, j)
			}
		}
	}
}

func TestHappy_ConceptsSuggest(t *testing.T) {
	skipIfNoEnv(t)
	code, body := get(t, "/api/v1/concepts/suggest?query=computer&limit=5", true)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", code, string(body))
	}

	arr := decodeJSON[[]models.Concept](t, body)
	if len(arr) == 0 {
		t.Fatalf("expected non-empty concepts array")
	}
	for i, c := range arr {
		if c.ID == 0 {
			t.Errorf("item %d: expected id > 0", i)
		}
		if c.Subject == "" {
			t.Errorf("item %d: expected subject non-empty", i)
		}
		if c.Language.ISO6393 == "" {
			t.Errorf("item %d: expected language.iso639_3 non-empty", i)
		}
		if c.CodingFramework.ID == 0 || c.CodingFramework.Name == "" {
			t.Errorf("item %d: expected coding_framework populated", i)
		}
	}
}

func TestHappy_LabelsSearch(t *testing.T) {
	skipIfNoEnv(t)
	code, body := get(t, "/api/v1/labels/search?query=dog&language=eng&language_iso_format=639-3&limit=5", true)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", code, string(body))
	}

	arr := decodeJSON[[]models.Label](t, body)
	if len(arr) == 0 {
		t.Fatalf("expected non-empty labels array")
	}
	for i, l := range arr {
		if l.ID == 0 {
			t.Errorf("item %d: expected id > 0", i)
		}
		if l.Text == "" {
			t.Errorf("item %d: expected text non-empty", i)
		}
		if l.Language == "" {
			t.Errorf("item %d: expected language non-empty", i)
		}
		if l.Picto.ID == 0 {
			t.Errorf("item %d: expected picto.id > 0", i)
		}
		if l.Picto.ImageURL == "" {
			t.Errorf("item %d: expected picto.image_url non-empty", i)
		}
		if l.Picto.NativeFormat == "" {
			t.Errorf("item %d: expected picto.native_format non-empty", i)
		}
	}
}
