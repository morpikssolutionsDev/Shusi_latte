package media_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"sushilatte/api/internal/media"
)

// mp4Header is the smallest "ftyp" box Go's content sniffer recognises as video/mp4.
var mp4Header = append([]byte{0, 0, 0, 0x18}, []byte("ftypmp42\x00\x00\x00\x00mp42isom")...)

func TestSaveVideo(t *testing.T) {
	dir := t.TempDir()
	s := media.NewStore(dir)
	key, err := s.SaveVideo(bytes.NewReader(mp4Header))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(key) != ".mp4" || key[:4] != "vid/" {
		t.Fatalf("key %q", key)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(key))); err != nil {
		t.Fatal(err)
	}
	again, err := s.SaveVideo(bytes.NewReader(mp4Header))
	if err != nil || again != key {
		t.Fatalf("identical bytes must reuse the key: %q %v", again, err)
	}
	if _, err := s.SaveVideo(bytes.NewReader([]byte("not a video"))); !errors.Is(err, media.ErrUnsupportedVideo) {
		t.Fatalf("want ErrUnsupportedVideo, got %v", err)
	}
}
