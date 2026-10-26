// Package media stores uploaded images and generates their served variants (TR-DM-09).
//
// Keys are content-addressed ("img/<sha256 prefix>"), so a file never changes in place and can
// be cached for a long time. Square variants (sq320, sq640, sq1024) are centre crops; the 16:9
// Product Display crop depends on the Product's focal point and is named after it
// (wide-<x>-<y>.webp), so moving the focal point produces a new URL.
package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"
)

const (
	MaxUpload  = 10 << 20
	quality    = 82
	wideWidth  = 1920
	wideHeight = 1080
)

// SquareSizes are the 1:1 variants used by cards, detail sheets and avatars.
var SquareSizes = []int{320, 640, 1024}

var ErrUnsupported = errors.New("unsupported image type (use JPEG, PNG or WebP)")

type Store struct{ dir string }

func NewStore(imagesDir string) *Store { return &Store{dir: imagesDir} }

// Save stores an image and its square variants, returning its key. Saving identical bytes
// again returns the same key without reprocessing.
func (s *Store) Save(r io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxUpload+1))
	if err != nil {
		return "", err
	}
	if len(data) > MaxUpload {
		return "", fmt.Errorf("image larger than %d MB", MaxUpload>>20)
	}
	ext, ok := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[http.DetectContentType(data)]
	if !ok {
		return "", ErrUnsupported
	}
	sum := sha256.Sum256(data)
	key := "img/" + hex.EncodeToString(sum[:12])
	dir := filepath.Join(s.dir, filepath.FromSlash(key))
	if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("sq%d.webp", SquareSizes[len(SquareSizes)-1]))); err == nil {
		return key, nil
	}
	img, err := decode(data)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "original"+ext), data, 0o644); err != nil {
		return "", err
	}
	for _, size := range SquareSizes {
		if err := writeWebP(filepath.Join(dir, fmt.Sprintf("sq%d.webp", size)), crop(img, size, size, 50, 50)); err != nil {
			return "", err
		}
	}
	return key, nil
}

// EnsureWide generates the 16:9 Product Display crop for a focal point, if missing.
func (s *Store) EnsureWide(key string, focusX, focusY int64) error {
	dir := filepath.Join(s.dir, filepath.FromSlash(key))
	target := filepath.Join(dir, WideName(focusX, focusY))
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "original.*"))
	if len(matches) == 0 {
		return fmt.Errorf("image %s not found", key)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return err
	}
	img, err := decode(data)
	if err != nil {
		return err
	}
	return writeWebP(target, crop(img, wideWidth, wideHeight, int(focusX), int(focusY)))
}

func WideName(focusX, focusY int64) string { return fmt.Sprintf("wide-%d-%d.webp", focusX, focusY) }

func decode(data []byte) (image.Image, error) {
	if http.DetectContentType(data) == "image/webp" {
		return webp.Decode(bytes.NewReader(data))
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrUnsupported
	}
	return img, nil
}

// crop scales img to cover w×h, then cuts the window centred on the focal point (percent).
func crop(img image.Image, w, h, focusX, focusY int) image.Image {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	// Source window with the target aspect ratio, as large as possible.
	cw, ch := sw, sw*h/w
	if ch > sh {
		ch, cw = sh, sh*w/h
	}
	cx := clamp(sw*focusX/100-cw/2, 0, sw-cw)
	cy := clamp(sh*focusY/100-ch/2, 0, sh-ch)
	src := image.Rect(b.Min.X+cx, b.Min.Y+cy, b.Min.X+cx+cw, b.Min.Y+cy+ch)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, src, draw.Src, nil)
	return dst
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

func writeWebP(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, webp.Options{Quality: quality}); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
