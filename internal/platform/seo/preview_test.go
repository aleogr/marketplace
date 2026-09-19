package seo_test

import (
	"bytes"
	"image/png"
	"os"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/seo"
)

func TestThePreviewIsAReadableImageOfTheRightShape(t *testing.T) {
	preview, err := seo.NewPreview()
	if err != nil {
		t.Fatalf("NewPreview() = %v", err)
	}

	drawn, err := preview.PNG("Marketplace 1")
	if err != nil {
		t.Fatalf("PNG() = %v", err)
	}

	image, err := png.Decode(bytes.NewReader(drawn))
	if err != nil {
		t.Fatalf("the preview is not a readable PNG: %v", err)
	}
	if bounds := image.Bounds(); bounds.Dx() != 1200 || bounds.Dy() != 630 {
		t.Errorf("the preview is %dx%d, want 1200x630", bounds.Dx(), bounds.Dy())
	}

	// The name is drawn, so the image differs per marketplace: one image for
	// every marketplace would be the same picture under three names.
	other, err := preview.PNG("Marketplace 2")
	if err != nil {
		t.Fatalf("PNG() = %v", err)
	}
	if bytes.Equal(drawn, other) {
		t.Error("two marketplaces got the same preview image")
	}

	// Kept when a test run asks for it, so a person can look at what is being
	// shipped rather than trusting a byte count.
	if directory := os.Getenv("PREVIEW_OUT"); directory != "" {
		if err := os.WriteFile(directory+"/preview.png", drawn, 0o600); err != nil {
			t.Fatalf("cannot keep the preview: %v", err)
		}
	}
}
