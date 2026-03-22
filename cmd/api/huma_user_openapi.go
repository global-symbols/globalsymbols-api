package main

import (
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"gs-api/internal/config"
)

// registerUserOpenAPI adds the proxied /user route to the OpenAPI spec (runtime
// handler remains the Chi UserProxy).
func registerUserOpenAPI(api huma.API, cfg *config.Config) {
	o := api.OpenAPI()
	if o.Paths == nil {
		o.Paths = map[string]*huma.PathItem{}
	}

	desc := "Proxies to the Rails Global Symbols API at `/api/v1/user`. Send an OAuth2 access token (e.g. Doorkeeper) in the `Authorization` header. Required scope: `profile`. Response shape and status codes come from Rails."
	if u, err := url.Parse(cfg.RailsBaseURL); err == nil && u.Scheme != "" && u.Host != "" {
		base := strings.TrimRight(cfg.RailsBaseURL, "/")
		desc += " Typical Doorkeeper-style endpoints are under `" + base + "` (for example `/oauth/token`)."
	}

	o.Paths["/user"] = &huma.PathItem{
		Get: &huma.Operation{
			OperationID: "get-user",
			Summary:     "Get current user",
			Description: desc,
			Tags:        []string{"User"},
			Security: []map[string][]string{
				{"OAuth2User": {}},
			},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "User profile JSON from Rails.",
					Content: map[string]*huma.MediaType{
						"application/json": {
							Schema: &huma.Schema{Type: "object"},
						},
					},
				},
				"401": {Description: "No access token or invalid token."},
				"403": {Description: "Token is valid but lacks the `profile` scope."},
			},
		},
	}
}
