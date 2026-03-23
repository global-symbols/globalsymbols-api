package previews

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"gs-api/internal/models"
)

const (
	DefaultPictoPreviewSize      = 64
	DefaultPictoPreviewWorkers   = 4
	DefaultPictoPreviewTimeout   = 8 * time.Second
	maxPictoPreviewSourceBytes   = 10 << 20
)

var pictoPreviewHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
}

// PopulateLabelSearchPreviewDataURLs fills preview_data_url for any rows whose
// source image can be fetched and resized in memory. Failures are ignored so
// search results still return even when an individual preview cannot be built.
func PopulateLabelSearchPreviewDataURLs(ctx context.Context, items []models.LabelSearchResult, size, workers int) {
	if size <= 0 {
		size = DefaultPictoPreviewSize
	}
	if workers <= 0 {
		workers = DefaultPictoPreviewWorkers
	}

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for i := range items {
		if strings.TrimSpace(items[i].Picto.ImageURL) == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			previewDataURL, err := PNGDataURL(ctx, items[idx].Picto.ImageURL, size)
			if err == nil {
				items[idx].Picto.PreviewDataURL = previewDataURL
			}
		}(i)
	}

	wg.Wait()
}

func PNGDataURL(ctx context.Context, sourceURL string, size int) (string, error) {
	img, err := decodeRemoteImage(ctx, sourceURL)
	if err != nil {
		return "", err
	}
	preview := resizeToSquarePNG(img, size)

	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&buf, preview); err != nil {
		return "", fmt.Errorf("encode preview png: %w", err)
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func decodeRemoteImage(ctx context.Context, sourceURL string) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build image request: %w", err)
	}
	req.Header.Set("Accept", "image/*")

	resp, err := pictoPreviewHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch image: unexpected status %d", resp.StatusCode)
	}

	limited := &io.LimitedReader{R: resp.Body, N: maxPictoPreviewSourceBytes + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read image bytes: %w", err)
	}
	if int64(len(data)) > maxPictoPreviewSourceBytes {
		return nil, fmt.Errorf("image exceeds %d byte limit", maxPictoPreviewSourceBytes)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	return img, nil
}

func resizeToSquarePNG(src image.Image, size int) *image.RGBA {
	if size <= 0 {
		size = DefaultPictoPreviewSize
	}

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	bounds := src.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	if srcWidth <= 0 || srcHeight <= 0 {
		return dst
	}

	scale := math.Min(float64(size)/float64(srcWidth), float64(size)/float64(srcHeight))
	targetWidth := max(1, int(math.Round(float64(srcWidth)*scale)))
	targetHeight := max(1, int(math.Round(float64(srcHeight)*scale)))
	offsetX := (size - targetWidth) / 2
	offsetY := (size - targetHeight) / 2

	for y := 0; y < targetHeight; y++ {
		srcY := bounds.Min.Y + int(float64(y)*float64(srcHeight)/float64(targetHeight))
		if srcY >= bounds.Max.Y {
			srcY = bounds.Max.Y - 1
		}
		for x := 0; x < targetWidth; x++ {
			srcX := bounds.Min.X + int(float64(x)*float64(srcWidth)/float64(targetWidth))
			if srcX >= bounds.Max.X {
				srcX = bounds.Max.X - 1
			}
			dst.Set(offsetX+x, offsetY+y, src.At(srcX, srcY))
		}
	}

	return dst
}
