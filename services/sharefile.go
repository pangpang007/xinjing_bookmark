package services

import (
	"bytes"
	"errors"
	"image/jpeg"
	"image/png"
	"io"
)

const MaxShareImageBytes = 2 << 20

var (
	ErrNotShareImage = errors.New("not a jpeg or png image")
	errTooLarge      = errors.New("image too large")
)

func PrepareShareJPEG(data []byte) ([]byte, error) {
	switch sniffImage(data) {
	case "jpeg":
		return data, nil
	case "png":
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, ErrNotShareImage
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	default:
		return nil, ErrNotShareImage
	}
}

func sniffImage(data []byte) string {
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "jpeg"
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) {
		return "png"
	}
	return ""
}

func ReadAtMost(r io.Reader, n int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(n)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > n {
		return nil, errTooLarge
	}
	return data, nil
}

func IsTooLarge(err error) bool {
	return errors.Is(err, errTooLarge)
}
