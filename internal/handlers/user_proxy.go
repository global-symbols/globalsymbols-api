package handlers

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

// UserProxy forwards /api/v2/user requests to the Rails API at /api/v1/user, which handles OAuth2.
func UserProxy(railsBaseURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target, err := url.Parse(railsBaseURL)
		if err != nil {
			http.Error(w, "Invalid Rails base URL", http.StatusInternalServerError)
			return
		}

		target.Path = strings.TrimRight(target.Path, "/") + "/api/v1/user"
		target.RawQuery = r.URL.RawQuery

		var body io.Reader
		if r.Body != nil {
			defer r.Body.Close()
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}
			body = io.NopCloser(strings.NewReader(string(data)))
		}

		req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), body)
		if err != nil {
			http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
			return
		}

		// Copy selected headers; Authorization is critical for OAuth2.
		for _, h := range []string{"Authorization", "Content-Type", "Accept"} {
			if v := r.Header.Get(h); v != "" {
				req.Header.Set(h, v)
			}
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "Upstream Rails API unavailable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})
}

