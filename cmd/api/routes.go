package main

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"gs-api/internal/auth"
	"gs-api/internal/config"
	"gs-api/internal/handlers"
)

// registerRoutes wires Huma as the runtime owner and exposes public docs/spec.
func registerRoutes(r *chi.Mux, sqlDB *sql.DB, cfg *config.Config) {
	r.Route("/api/v1", func(r chi.Router) {
		// /api/v1/user stays proxied to Rails and is not part of the OpenAPI docs.
		r.Handle("/user", handlers.UserProxy(cfg.RailsBaseURL))

		// Huma API with Scalar docs, mounted directly under /api/v1 (no Chi auth).
		// Override Huma's default RFC 9457 problem details so Go matches the
		// Rails/spec error envelope across all endpoints.
		configureHumaErrors()
		humaCfg := huma.DefaultConfig("Global Symbols Go API", "1.0.0")
		humaCfg.DocsPath = "/docs" // GET /api/v1/docs
		humaCfg.DocsRenderer = huma.DocsRendererScalar
		// Ensure generated OpenAPI/Scalar requests use the correct base path.
		humaCfg.Servers = []*huma.Server{{URL: "/api/v1"}}
		securitySchemes := humaCfg.Components.SecuritySchemes
		if securitySchemes == nil {
			securitySchemes = map[string]*huma.SecurityScheme{}
		}
		// Match the actual middleware: accept API keys via X-Api-Key.
		securitySchemes["ApiKeyAuth"] = &huma.SecurityScheme{
			Type: "apiKey",
			In:   "header",
			Name: "X-Api-Key",
		}
		humaCfg.Components.SecuritySchemes = securitySchemes
		api := humachi.New(r, humaCfg)

		// Huma middleware to enforce API key on data endpoints only.
		api.UseMiddleware(func(hctx huma.Context, next func(huma.Context)) {
			req, res := humachi.Unwrap(hctx)
			path := req.URL.Path

			// Allow docs + OpenAPI/spec endpoints without API key.
			if strings.HasPrefix(path, "/api/v1/docs") ||
				strings.HasPrefix(path, "/api/v1/openapi") ||
				strings.HasPrefix(path, "/api/v1/schemas") {
				next(hctx)
				return
			}

			// Reuse existing Chi API key middleware for everything else.
			mw := auth.APIKeyMiddleware(sqlDB)
			mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if metadata, ok := auth.MetadataFromContext(r.Context()); ok {
					next(huma.WithValue(hctx, auth.MetadataContextKey(), metadata))
					return
				}

				next(hctx)
			})).ServeHTTP(res, req)
		})

		// Huma operations now own the runtime for all documented endpoints.
		registerHumaOperations(api, sqlDB, cfg)
	})
}
