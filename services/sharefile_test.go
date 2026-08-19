package services

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestPrepareShareJPEG(t *testing.T) {
	var jpegBuf bytes.Buffer
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := jpeg.Encode(&jpegBuf, src, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	got, err := PrepareShareJPEG(jpegBuf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if sniffImage(got) != "jpeg" {
		t.Fatal("jpeg passthrough")
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, src); err != nil {
		t.Fatal(err)
	}
	got, err = PrepareShareJPEG(pngBuf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if sniffImage(got) != "jpeg" {
		t.Fatal("png should become jpeg")
	}

	if _, err := PrepareShareJPEG([]byte("hello")); err != ErrNotShareImage {
		t.Fatalf("err=%v", err)
	}
}

func TestReadAtMost(t *testing.T) {
	_, err := ReadAtMost(bytes.NewReader(bytes.Repeat([]byte{'a'}, 8)), 4)
	if !IsTooLarge(err) {
		t.Fatalf("err=%v", err)
	}
	got, err := ReadAtMost(bytes.NewReader([]byte("ab")), 4)
	if err != nil || string(got) != "ab" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestWxaCodeObjectKey(t *testing.T) {
	if WxaCodeObjectKey("") != "wxacode/share-release.png" {
		t.Fatal(WxaCodeObjectKey(""))
	}
	if WxaCodeObjectKey("develop") != "wxacode/share-develop.png" {
		t.Fatal(WxaCodeObjectKey("develop"))
	}
}
