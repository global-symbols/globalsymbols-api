// Package parity runs the same requests against Rails and Go APIs and compares responses.
// Requires Rails at TEST_RAILS_BASE_URL (e.g. http://localhost:3000) and Go at TEST_GO_BASE_URL (e.g. http://localhost:8080),
// and a valid API key in TEST_API_KEY. Skip if env vars are not set or APIs are unavailable.
package parity

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"testing"

	"gs-api/internal/models"
)

var (
	railsBase = os.Getenv("TEST_RAILS_BASE_URL") // e.g. http://localhost:3000
	goBase    = os.Getenv("TEST_GO_BASE_URL")    // e.g. http://localhost:8080
	apiKey    = os.Getenv("TEST_API_KEY")
)

func skipIfNoParityEnv(t *testing.T) {
	if railsBase == "" || goBase == "" || apiKey == "" {
		t.Skip("Set TEST_RAILS_BASE_URL, TEST_GO_BASE_URL, and TEST_API_KEY to run parity tests")
	}
}

type apiErrorResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func request(t *testing.T, baseURL, path, key string) (int, []byte) {
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if key != "" {
		req.Header.Set("X-Api-Key", key)
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

func get(t *testing.T, baseURL, path string) (int, []byte) {
	return request(t, baseURL, path, apiKey)
}

func sortedInt64sCopy(in []int64) []int64 {
	out := append([]int64(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func assertSameIDMultiset(t *testing.T, label string, railsIDs, goIDs []int64) {
	if len(railsIDs) != len(goIDs) {
		t.Errorf("%s count: Rails %d vs Go %d", label, len(railsIDs), len(goIDs))
		return
	}

	r := sortedInt64sCopy(railsIDs)
	g := sortedInt64sCopy(goIDs)
	for i := range r {
		if r[i] != g[i] {
			t.Errorf(
				"%s id multiset mismatch (sorted). First mismatch at index %d: Rails %d vs Go %d",
				label, i, r[i], g[i],
			)
			return
		}
	}
}

func assertSameErrorResponse(t *testing.T, label string, railsBody, goBody []byte) {
	t.Helper()

	var railsErr, goErr apiErrorResponse
	if err := json.Unmarshal(railsBody, &railsErr); err != nil {
		t.Fatalf("%s Rails JSON: %v body=%s", label, err, string(railsBody))
	}
	if err := json.Unmarshal(goBody, &goErr); err != nil {
		t.Fatalf("%s Go JSON: %v body=%s", label, err, string(goBody))
	}

	if railsErr.Code != goErr.Code {
		t.Errorf("%s code: Rails %d vs Go %d", label, railsErr.Code, goErr.Code)
	}
	if railsErr.Error != goErr.Error {
		t.Errorf("%s error: Rails %q vs Go %q", label, railsErr.Error, goErr.Error)
	}
}

func TestParity_LanguagesActive(t *testing.T) {
	skipIfNoParityEnv(t)
	path := "/api/v2/languages/active"
	railsCode, railsBody := get(t, railsBase, path)
	goCode, goBody := get(t, goBase, path)
	if railsCode != goCode {
		t.Errorf("status: Rails %d vs Go %d", railsCode, goCode)
	}
	if railsCode != http.StatusOK {
		t.Fatalf("expected both APIs to return 200, got Rails %d and Go %d", railsCode, goCode)
	}

	var railsArr, goArr []models.Language
	if err := json.Unmarshal(railsBody, &railsArr); err != nil {
		t.Fatalf("Rails JSON: %v", err)
	}
	if err := json.Unmarshal(goBody, &goArr); err != nil {
		t.Fatalf("Go JSON: %v", err)
	}

	railsIDs := make([]int64, 0, len(railsArr))
	for _, x := range railsArr {
		railsIDs = append(railsIDs, x.ID)
	}
	goIDs := make([]int64, 0, len(goArr))
	for _, x := range goArr {
		goIDs = append(goIDs, x.ID)
	}

	assertSameIDMultiset(t, "languages", railsIDs, goIDs)
}

func TestParity_Symbolsets(t *testing.T) {
	skipIfNoParityEnv(t)
	path := "/api/v2/symbolsets"
	railsCode, railsBody := get(t, railsBase, path)
	goCode, goBody := get(t, goBase, path)
	if railsCode != goCode {
		t.Errorf("status: Rails %d vs Go %d", railsCode, goCode)
	}
	if railsCode != http.StatusOK {
		t.Fatalf("expected both APIs to return 200, got Rails %d and Go %d", railsCode, goCode)
	}

	var railsArr, goArr []models.Symbolset
	if err := json.Unmarshal(railsBody, &railsArr); err != nil {
		t.Fatalf("Rails JSON: %v", err)
	}
	if err := json.Unmarshal(goBody, &goArr); err != nil {
		t.Fatalf("Go JSON: %v", err)
	}

	railsIDs := make([]int64, 0, len(railsArr))
	for _, x := range railsArr {
		railsIDs = append(railsIDs, x.ID)
	}
	goIDs := make([]int64, 0, len(goArr))
	for _, x := range goArr {
		goIDs = append(goIDs, x.ID)
	}

	assertSameIDMultiset(t, "symbolsets", railsIDs, goIDs)
}

func TestParity_ConceptsSuggest(t *testing.T) {
	skipIfNoParityEnv(t)
	path := "/api/v2/concepts/suggest?query=computer&limit=5"
	railsCode, railsBody := get(t, railsBase, path)
	goCode, goBody := get(t, goBase, path)
	if railsCode != goCode {
		t.Errorf("status: Rails %d vs Go %d", railsCode, goCode)
	}
	if railsCode != http.StatusOK {
		t.Fatalf("expected both APIs to return 200, got Rails %d and Go %d", railsCode, goCode)
	}

	var railsArr, goArr []models.Concept
	if err := json.Unmarshal(railsBody, &railsArr); err != nil {
		t.Fatalf("Rails JSON: %v", err)
	}
	if err := json.Unmarshal(goBody, &goArr); err != nil {
		t.Fatalf("Go JSON: %v", err)
	}

	railsIDs := make([]int64, 0, len(railsArr))
	for _, x := range railsArr {
		railsIDs = append(railsIDs, x.ID)
	}
	goIDs := make([]int64, 0, len(goArr))
	for _, x := range goArr {
		goIDs = append(goIDs, x.ID)
	}

	assertSameIDMultiset(t, "concepts", railsIDs, goIDs)
}

func TestParity_LabelsSearch(t *testing.T) {
	skipIfNoParityEnv(t)
	// Label ordering diverges slightly between Rails and Go for tied rows,
	// so compare a broader result set by ID and ignore order.
	path := "/api/v2/labels/search?query=hello&limit=50"
	railsCode, railsBody := get(t, railsBase, path)
	goCode, goBody := get(t, goBase, path)
	if railsCode != goCode {
		t.Errorf("status: Rails %d vs Go %d", railsCode, goCode)
	}
	if railsCode != http.StatusOK {
		t.Fatalf("expected both APIs to return 200, got Rails %d and Go %d", railsCode, goCode)
	}

	var railsArr, goArr []models.Label
	if err := json.Unmarshal(railsBody, &railsArr); err != nil {
		t.Fatalf("Rails JSON: %v", err)
	}
	if err := json.Unmarshal(goBody, &goArr); err != nil {
		t.Fatalf("Go JSON: %v", err)
	}

	railsIDs := make([]int64, 0, len(railsArr))
	for _, x := range railsArr {
		railsIDs = append(railsIDs, x.ID)
	}
	goIDs := make([]int64, 0, len(goArr))
	for _, x := range goArr {
		goIDs = append(goIDs, x.ID)
	}

	assertSameIDMultiset(t, "labels", railsIDs, goIDs)
}

func TestParity_Pictos(t *testing.T) {
	skipIfNoParityEnv(t)
	path := "/api/v2/pictos?symbolset=arasaac&per_page=5"
	railsCode, railsBody := get(t, railsBase, path)
	goCode, goBody := get(t, goBase, path)
	if railsCode != goCode {
		t.Errorf("status: Rails %d vs Go %d", railsCode, goCode)
	}
	if railsCode != http.StatusOK {
		t.Fatalf("expected both APIs to return 200, got Rails %d and Go %d", railsCode, goCode)
	}

	var railsResp, goResp models.PagedPictosResponse
	if err := json.Unmarshal(railsBody, &railsResp); err != nil {
		t.Fatalf("Rails JSON: %v", err)
	}
	if err := json.Unmarshal(goBody, &goResp); err != nil {
		t.Fatalf("Go JSON: %v", err)
	}

	if railsResp.Total != goResp.Total {
		t.Errorf("pictos total: Rails %d vs Go %d", railsResp.Total, goResp.Total)
	}

	railsIDs := make([]int64, 0, len(railsResp.Items))
	for _, x := range railsResp.Items {
		railsIDs = append(railsIDs, x.ID)
	}
	goIDs := make([]int64, 0, len(goResp.Items))
	for _, x := range goResp.Items {
		goIDs = append(goIDs, x.ID)
	}

	assertSameIDMultiset(t, "pictos.items", railsIDs, goIDs)
}

func TestParity_AuthNoKey(t *testing.T) {
	skipIfNoParityEnv(t)
	path := "/api/v2/languages/active"
	railsCode, railsBody := request(t, railsBase, path, "")
	goCode, goBody := request(t, goBase, path, "")
	if railsCode != goCode {
		t.Errorf("status: Rails %d vs Go %d", railsCode, goCode)
	}
	assertSameErrorResponse(t, "auth-no-key", railsBody, goBody)
}

func TestParity_AuthInvalidKey(t *testing.T) {
	skipIfNoParityEnv(t)
	path := "/api/v2/languages/active"
	railsCode, railsBody := request(t, railsBase, path, "invalid-key")
	goCode, goBody := request(t, goBase, path, "invalid-key")
	if railsCode != goCode {
		t.Errorf("status: Rails %d vs Go %d", railsCode, goCode)
	}
	assertSameErrorResponse(t, "auth-invalid-key", railsBody, goBody)
}

func TestParity_ConceptsSuggestMissingQuery(t *testing.T) {
	skipIfNoParityEnv(t)
	path := "/api/v2/concepts/suggest"
	railsCode, railsBody := get(t, railsBase, path)
	goCode, goBody := get(t, goBase, path)
	if railsCode != goCode {
		t.Errorf("status: Rails %d vs Go %d", railsCode, goCode)
	}
	assertSameErrorResponse(t, "concepts-missing-query", railsBody, goBody)
}

func TestParity_LabelsSearchMissingQuery(t *testing.T) {
	skipIfNoParityEnv(t)
	path := "/api/v2/labels/search"
	railsCode, railsBody := get(t, railsBase, path)
	goCode, goBody := get(t, goBase, path)
	if railsCode != goCode {
		t.Errorf("status: Rails %d vs Go %d", railsCode, goCode)
	}
	assertSameErrorResponse(t, "labels-missing-query", railsBody, goBody)
}

func TestParity_PictosInvalidSince(t *testing.T) {
	skipIfNoParityEnv(t)
	path := "/api/v2/pictos?symbolset=arasaac&since=not-a-date"
	railsCode, railsBody := get(t, railsBase, path)
	goCode, goBody := get(t, goBase, path)
	if railsCode != goCode {
		t.Errorf("status: Rails %d vs Go %d", railsCode, goCode)
	}
	assertSameErrorResponse(t, "pictos-invalid-since", railsBody, goBody)
}
