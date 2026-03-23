package db

import (
	"database/sql"
	"testing"
)

func TestLogoURL(t *testing.T) {
	base := "https://globalsymbols.com"
	env := "production"
	id := int64(96)
	filename := "_13b23b05-f2fa-4be2-a8d4-697a24cdfd76.png"

	t.Run("filename only", func(t *testing.T) {
		got := logoURL(base, env, id, sql.NullString{String: filename, Valid: true})
		if got == nil {
			t.Fatal("expected non-nil")
		}
		want := "https://globalsymbols.com/uploads/production/symbolset/logo/96/_13b23b05-f2fa-4be2-a8d4-697a24cdfd76.png"
		if *got != want {
			t.Fatalf("got %q want %q", *got, want)
		}
	})

	t.Run("absolute URL unchanged", func(t *testing.T) {
		abs := "https://cdn.example.com/logo.png"
		got := logoURL(base, env, id, sql.NullString{String: abs, Valid: true})
		if got == nil || *got != abs {
			t.Fatalf("got %v want %q", got, abs)
		}
	})

	t.Run("uploads-relative path", func(t *testing.T) {
		rel := "uploads/production/symbolset/logo/96/file.png"
		got := logoURL(base, env, id, sql.NullString{String: rel, Valid: true})
		if got == nil {
			t.Fatal("expected non-nil")
		}
		want := "https://globalsymbols.com/uploads/production/symbolset/logo/96/file.png"
		if *got != want {
			t.Fatalf("got %q want %q", *got, want)
		}
	})

	t.Run("empty base", func(t *testing.T) {
		got := logoURL("", env, id, sql.NullString{String: filename, Valid: true})
		if got != nil {
			t.Fatalf("expected nil, got %q", *got)
		}
	})

	t.Run("empty APP_ENV with filename only", func(t *testing.T) {
		got := logoURL(base, "", id, sql.NullString{String: filename, Valid: true})
		if got != nil {
			t.Fatalf("expected nil, got %q", *got)
		}
	})
}
