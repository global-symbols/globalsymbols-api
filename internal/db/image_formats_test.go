package db

import (
	"database/sql"
	"testing"
)

func TestInferNativeFormatFromImageFile(t *testing.T) {
	t.Run("jpg normalizes to jpeg", func(t *testing.T) {
		got := inferNativeFormatFromImageFile(sql.NullString{String: "foo/bar/icon.JPG", Valid: true})
		if got != "jpeg" {
			t.Fatalf("got %q want %q", got, "jpeg")
		}
	})

	t.Run("known formats are preserved", func(t *testing.T) {
		cases := []struct {
			file string
			want string
		}{
			{file: "icon.jpeg", want: "jpeg"},
			{file: "icon.png", want: "png"},
			{file: "icon.svg", want: "svg"},
			{file: "icon.gif", want: "gif"},
			{file: "icon.webp", want: "webp"},
		}

		for _, tc := range cases {
			got := inferNativeFormatFromImageFile(sql.NullString{String: tc.file, Valid: true})
			if got != tc.want {
				t.Fatalf("file %q: got %q want %q", tc.file, got, tc.want)
			}
		}
	})

	t.Run("unknown extension falls back to png", func(t *testing.T) {
		got := inferNativeFormatFromImageFile(sql.NullString{String: "icon.bmp", Valid: true})
		if got != "png" {
			t.Fatalf("got %q want %q", got, "png")
		}
	})

	t.Run("missing extension falls back to png", func(t *testing.T) {
		got := inferNativeFormatFromImageFile(sql.NullString{String: "icon", Valid: true})
		if got != "png" {
			t.Fatalf("got %q want %q", got, "png")
		}
	})

	t.Run("invalid value falls back to png", func(t *testing.T) {
		got := inferNativeFormatFromImageFile(sql.NullString{Valid: false})
		if got != "png" {
			t.Fatalf("got %q want %q", got, "png")
		}
	})
}
