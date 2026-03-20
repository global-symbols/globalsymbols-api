package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
)

type endpointConfig struct {
	Name string
	Path string
	Kind string
}

type conceptRow struct {
	ID      int64
	Subject string
}

type labelRow struct {
	ID          int64
	Text        string
	Language    string
	PictoID     int64
	SymbolsetID int64
}

func main() {
	railsBase := mustEnv("TEST_RAILS_BASE_URL")
	goBase := mustEnv("TEST_GO_BASE_URL")
	apiKey := mustEnv("TEST_API_KEY")

	endpoints := []endpointConfig{
		{
			Name: "ConceptsSuggest",
			Path: "/api/v1/concepts/suggest?query=computer&limit=5",
			Kind: "concepts",
		},
		{
			Name: "LabelsSearch",
			Path: "/api/v1/labels/search?query=hello&limit=5",
			Kind: "labels",
		},
	}

	for i, ep := range endpoints {
		if i > 0 {
			fmt.Println()
		}
		inspectEndpoint(railsBase, goBase, apiKey, ep)
	}
}

func mustEnv(name string) string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		fmt.Fprintf(os.Stderr, "missing required env var %s\n", name)
		os.Exit(1)
	}
	return v
}

func fetch(baseURL, path, apiKey string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+path, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}

func inspectEndpoint(railsBase, goBase, apiKey string, ep endpointConfig) {
	fmt.Printf("=== %s ===\n", ep.Name)
	fmt.Printf("Path: %s\n", ep.Path)

	railsCode, railsBody, err := fetch(railsBase, ep.Path, apiKey)
	if err != nil {
		fmt.Printf("Rails request error: %v\n", err)
		return
	}
	goCode, goBody, err := fetch(goBase, ep.Path, apiKey)
	if err != nil {
		fmt.Printf("Go request error: %v\n", err)
		return
	}

	fmt.Printf("Rails status: %d\n", railsCode)
	fmt.Printf("Go status:    %d\n", goCode)

	if railsCode != http.StatusOK || goCode != http.StatusOK {
		fmt.Printf("\nRails body:\n%s\n", string(railsBody))
		fmt.Printf("\nGo body:\n%s\n", string(goBody))
		return
	}

	switch ep.Kind {
	case "concepts":
		inspectConcepts(railsBody, goBody)
	case "labels":
		inspectLabels(railsBody, goBody)
	default:
		fmt.Printf("unknown endpoint kind %q\n", ep.Kind)
	}
}

func inspectConcepts(railsBody, goBody []byte) {
	railsRows, err := parseConcepts(railsBody)
	if err != nil {
		fmt.Printf("Rails JSON parse error: %v\nRails body:\n%s\n", err, string(railsBody))
		return
	}
	goRows, err := parseConcepts(goBody)
	if err != nil {
		fmt.Printf("Go JSON parse error: %v\nGo body:\n%s\n", err, string(goBody))
		return
	}

	fmt.Println()
	fmt.Println("Rails ordered rows:")
	for i, row := range railsRows {
		fmt.Printf("  [%d] id=%d subject=%q\n", i, row.ID, row.Subject)
	}

	fmt.Println()
	fmt.Println("Go ordered rows:")
	for i, row := range goRows {
		fmt.Printf("  [%d] id=%d subject=%q\n", i, row.ID, row.Subject)
	}

	railsIDs := conceptIDs(railsRows)
	goIDs := conceptIDs(goRows)
	printIDComparison(railsIDs, goIDs)
}

func inspectLabels(railsBody, goBody []byte) {
	railsRows, err := parseLabels(railsBody)
	if err != nil {
		fmt.Printf("Rails JSON parse error: %v\nRails body:\n%s\n", err, string(railsBody))
		return
	}
	goRows, err := parseLabels(goBody)
	if err != nil {
		fmt.Printf("Go JSON parse error: %v\nGo body:\n%s\n", err, string(goBody))
		return
	}

	fmt.Println()
	fmt.Println("Rails ordered rows:")
	for i, row := range railsRows {
		fmt.Printf(
			"  [%d] id=%d text=%q language=%q picto.id=%d picto.symbolset_id=%d\n",
			i, row.ID, row.Text, row.Language, row.PictoID, row.SymbolsetID,
		)
	}

	fmt.Println()
	fmt.Println("Go ordered rows:")
	for i, row := range goRows {
		fmt.Printf(
			"  [%d] id=%d text=%q language=%q picto.id=%d picto.symbolset_id=%d\n",
			i, row.ID, row.Text, row.Language, row.PictoID, row.SymbolsetID,
		)
	}

	railsIDs := labelIDs(railsRows)
	goIDs := labelIDs(goRows)
	printIDComparison(railsIDs, goIDs)
}

func parseConcepts(body []byte) ([]conceptRow, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil, err
	}

	rows := make([]conceptRow, 0, len(arr))
	for _, item := range arr {
		rows = append(rows, conceptRow{
			ID:      getInt64(item, "id"),
			Subject: getString(item, "subject"),
		})
	}
	return rows, nil
}

func parseLabels(body []byte) ([]labelRow, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil, err
	}

	rows := make([]labelRow, 0, len(arr))
	for _, item := range arr {
		picto, _ := item["picto"].(map[string]any)
		rows = append(rows, labelRow{
			ID:          getInt64(item, "id"),
			Text:        getString(item, "text"),
			Language:    getString(item, "language"),
			PictoID:     getInt64(picto, "id"),
			SymbolsetID: getInt64(picto, "symbolset_id"),
		})
	}
	return rows, nil
}

func getInt64(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}

func getString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func conceptIDs(rows []conceptRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func labelIDs(rows []labelRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func printIDComparison(railsIDs, goIDs []int64) {
	fmt.Println()
	fmt.Printf("Rails ordered IDs: %v\n", railsIDs)
	fmt.Printf("Go ordered IDs:    %v\n", goIDs)

	railsSorted := sortedCopy(railsIDs)
	goSorted := sortedCopy(goIDs)
	fmt.Printf("Rails sorted IDs:  %v\n", railsSorted)
	fmt.Printf("Go sorted IDs:     %v\n", goSorted)

	fmt.Printf("Only in Rails:     %v\n", difference(railsSorted, goSorted))
	fmt.Printf("Only in Go:        %v\n", difference(goSorted, railsSorted))
}

func sortedCopy(in []int64) []int64 {
	out := append([]int64(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func difference(a, b []int64) []int64 {
	counts := map[int64]int{}
	for _, id := range b {
		counts[id]++
	}

	var out []int64
	for _, id := range a {
		if counts[id] > 0 {
			counts[id]--
			continue
		}
		out = append(out, id)
	}
	return out
}

