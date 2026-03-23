package previews

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gs-api/internal/models"
)

func TestPNGDataURL_Returns64x64PNG(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		src := image.NewRGBA(image.Rect(0, 0, 128, 64))
		for y := 0; y < 64; y++ {
			for x := 0; x < 128; x++ {
				src.Set(x, y, color.RGBA{R: 0x33, G: 0x66, B: 0x99, A: 0xff})
			}
		}
		if err := png.Encode(w, src); err != nil {
			t.Fatalf("encode source png: %v", err)
		}
	}))
	defer server.Close()

	dataURL, err := PNGDataURL(context.Background(), server.URL, 64)
	if err != nil {
		t.Fatalf("PNGDataURL error: %v", err)
	}
	if !strings.HasPrefix(dataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected prefix: %s", dataURL[:min(len(dataURL), 32)])
	}

	encoded := strings.TrimPrefix(dataURL, "data:image/png;base64,")
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode data url: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode preview png: %v", err)
	}
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("expected 64x64 preview, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestPopulateLabelSearchPreviewDataURLs_SoftFails(t *testing.T) {
	items := []models.LabelSearchResult{
		{
			ID:       1,
			Text:     "dog",
			Language: "eng",
			Picto: models.LabelSearchPicto{
				ID:       1,
				ImageURL: "http://127.0.0.1:1/unreachable.png",
			},
		},
	}

	PopulateLabelSearchPreviewDataURLs(context.Background(), items, 64, 1)

	if items[0].Picto.PreviewDataURL != "" {
		t.Fatalf("expected preview_data_url to remain empty on fetch failure")
	}
}
