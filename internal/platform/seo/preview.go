package seo

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The size every social network expects of a link preview: 1.91:1, large
// enough that nobody upscales it.
const (
	previewWidth  = 1200
	previewHeight = 630
)

// Preview draws the image a link to this marketplace shows in a message or a
// post.
//
// It is drawn rather than stored, because the text on it is the marketplace's
// name and every marketplace has a different one: an image per marketplace,
// committed to the repository, would be a file to remember on the day somebody
// creates the fourth marketplace. Drawing also keeps the repository free of
// binary blobs nobody can review.
//
// Link previews are served in both indexing modes, which is exactly why
// crawling stays allowed while indexing is refused (docs/requirements.md,
// section 7.1).
type Preview struct {
	mu     sync.Mutex
	drawn  map[string][]byte
	shaper *opentype.Font
}

// NewPreview returns a preview drawer, or an error when the font it draws with
// cannot be read — which would be a build that shipped a broken dependency
// rather than anything a deployment can fix.
func NewPreview() (*Preview, error) {
	parsed, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}
	return &Preview{drawn: map[string][]byte{}, shaper: parsed}, nil
}

// PNG returns the preview image for a name, drawing it the first time.
func (p *Preview) PNG(name string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if image, ok := p.drawn[name]; ok {
		return image, nil
	}

	image, err := p.draw(name)
	if err != nil {
		return nil, err
	}
	p.drawn[name] = image
	return image, nil
}

func (p *Preview) draw(name string) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, previewWidth, previewHeight))
	background := color.RGBA{R: 0x11, G: 0x18, B: 0x27, A: 0xff}
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: background}, image.Point{}, draw.Src)

	face, err := opentype.NewFace(p.shaper, &opentype.FaceOptions{
		Size: 72, DPI: 72, Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = face.Close() }()

	drawer := &font.Drawer{
		Dst:  canvas,
		Src:  image.NewUniform(color.RGBA{R: 0xf8, G: 0xfa, B: 0xfc, A: 0xff}),
		Face: face,
	}

	// Centred, both ways. The text is the only thing on the image, so it being
	// off-centre is the only way it can look wrong.
	width := drawer.MeasureString(name)
	drawer.Dot = fixed.Point26_6{
		X: fixed.I(previewWidth)/2 - width/2,
		Y: fixed.I(previewHeight)/2 + fixed.I(24),
	}
	drawer.DrawString(name)

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
