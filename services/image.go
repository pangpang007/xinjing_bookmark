package services

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const (
	shareWidth  = 1080
	shareHeight = 1350
)

var styleColors = map[string]color.RGBA{
	"warm":       {R: 255, G: 140, B: 66, A: 255},
	"melancholy": {R: 44, G: 95, B: 93, A: 255},
	"nostalgic":  {R: 139, G: 115, B: 85, A: 255},
	"hopeful":    {R: 106, G: 168, B: 79, A: 255},
}

type ImageService struct {
	font       *opentype.Font
	httpClient *http.Client
	location   *time.Location
}

type ShareImageInput struct {
	LiteratureText string
	BookName       string
	Author         string
	Style          string
	Nickname       string
	AvatarURL      string
	QRCodePNG      []byte
}

func NewImageService(fontPath string, loc *time.Location) (*ImageService, error) {
	raw, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("read font: %w", err)
	}
	f, err := opentype.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return &ImageService{
		font:       f,
		httpClient: &http.Client{Timeout: 8 * time.Second},
		location:   loc,
	}, nil
}

func (s *ImageService) Generate(in ShareImageInput) ([]byte, error) {
	bg, ok := styleColors[in.Style]
	if !ok {
		bg = styleColors["nostalgic"]
	}

	dc := gg.NewContext(shareWidth, shareHeight)
	dc.SetColor(bg)
	dc.Clear()
	drawVignette(dc)

	bodyFace, err := s.face(42)
	if err != nil {
		return nil, err
	}
	metaFace, err := s.face(28)
	if err != nil {
		return nil, err
	}
	smallFace, err := s.face(24)
	if err != nil {
		return nil, err
	}

	const (
		padX       = 90.0
		textTop    = 160.0
		textBottom = 980.0
	)
	maxWidth := float64(shareWidth) - padX*2
	maxHeight := textBottom - textTop

	dc.SetFontFace(bodyFace)
	dc.SetRGB(1, 1, 1)
	lines, lineH := fitLines(dc, s, in.LiteratureText, maxWidth, maxHeight)
	totalH := float64(len(lines)) * lineH
	startY := textTop + (maxHeight-totalH)/2 + lineH*0.35
	for i, line := range lines {
		dc.DrawStringAnchored(line, shareWidth/2, startY+float64(i)*lineH, 0.5, 0.5)
	}

	dc.SetFontFace(metaFace)
	dc.SetRGBA(1, 1, 1, 0.88)
	citation := fmt.Sprintf("《%s》  %s", in.BookName, in.Author)
	dc.DrawStringAnchored(citation, shareWidth/2, 1048, 0.5, 0.5)

	dc.SetRGBA(1, 1, 1, 0.35)
	dc.SetLineWidth(1)
	dc.DrawLine(shareWidth/2-160, 1008, shareWidth/2+160, 1008)
	dc.Stroke()

	drawFooter(dc, s, smallFace, in)
	return encodeJPEG(dc.Image(), 90)
}

func (s *ImageService) face(size float64) (font.Face, error) {
	return opentype.NewFace(s.font, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

func fitLines(dc *gg.Context, s *ImageService, text string, maxWidth, maxHeight float64) ([]string, float64) {
	text = strings.TrimSpace(text)
	for size := 44.0; size >= 28; size -= 2 {
		face, err := s.face(size)
		if err != nil {
			continue
		}
		dc.SetFontFace(face)
		lines := wrapText(dc, text, maxWidth)
		lineH := size * 1.7
		if float64(len(lines))*lineH <= maxHeight {
			return lines, lineH
		}
	}
	face, err := s.face(28)
	if err != nil {
		return []string{text}, 48
	}
	dc.SetFontFace(face)
	lines := wrapText(dc, text, maxWidth)
	const maxLines = 12
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		last := lines[maxLines-1]
		if utf8.RuneCountInString(last) > 2 {
			runes := []rune(last)
			lines[maxLines-1] = string(runes[:len(runes)-1]) + "…"
		}
	}
	return lines, 28 * 1.7
}

func wrapText(dc *gg.Context, text string, maxWidth float64) []string {
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		var cur string
		for _, r := range para {
			trial := cur + string(r)
			w, _ := dc.MeasureString(trial)
			if w > maxWidth && cur != "" {
				lines = append(lines, cur)
				cur = string(r)
				continue
			}
			cur = trial
		}
		if cur != "" {
			lines = append(lines, cur)
		}
	}
	if len(lines) == 0 {
		return []string{text}
	}
	return lines
}

func drawVignette(dc *gg.Context) {
	grad := gg.NewLinearGradient(0, 1100, 0, shareHeight)
	grad.AddColorStop(0, color.RGBA{0, 0, 0, 0})
	grad.AddColorStop(1, color.RGBA{0, 0, 0, 90})
	dc.SetFillStyle(grad)
	dc.DrawRectangle(0, 1100, shareWidth, shareHeight-1100)
	dc.Fill()
}

func drawFooter(dc *gg.Context, s *ImageService, smallFace font.Face, in ShareImageInput) {
	const (
		bottomPad = 72.0
		avatarR   = 36.0
	)
	avatarCX := 90.0 + avatarR
	avatarCY := float64(shareHeight) - bottomPad - 88
	avatar := s.loadAvatar(in.AvatarURL, int(avatarR*2))
	drawCircleImage(dc, avatar, avatarCX, avatarCY, avatarR)

	dc.SetFontFace(smallFace)
	dc.SetRGB(1, 1, 1)
	nickname := strings.TrimSpace(in.Nickname)
	if nickname == "" {
		nickname = "心境用户"
	}
	nickname = ellipsis(dc, nickname, 280)
	dc.DrawStringAnchored(nickname, avatarCX+avatarR+18, avatarCY, 0, 0.5)

	date := time.Now().In(s.location).Format("2006年1月2日")
	qrSize := 168.0
	qrX := float64(shareWidth) - 90 - qrSize
	qrY := float64(shareHeight) - bottomPad - qrSize
	dc.DrawStringAnchored(date, qrX+qrSize, qrY-18, 1, 1)

	if qr := decodeQR(in.QRCodePNG, int(qrSize)); qr != nil {
		dc.DrawImage(qr, int(qrX), int(qrY))
	} else {
		dc.SetRGBA(1, 1, 1, 0.18)
		dc.DrawRoundedRectangle(qrX, qrY, qrSize, qrSize, 16)
		dc.Fill()
		dc.SetRGBA(1, 1, 1, 0.7)
		dc.DrawStringAnchored("心境书签", qrX+qrSize/2, qrY+qrSize/2, 0.5, 0.5)
	}
}

func (s *ImageService) loadAvatar(url string, size int) image.Image {
	placeholder := avatarPlaceholder(size)
	if url == "" {
		return placeholder
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return placeholder
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("[WARN] download avatar: %v", err)
		return placeholder
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return placeholder
	}
	img, err := imaging.Decode(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return placeholder
	}
	return imaging.Fill(img, size, size, imaging.Center, imaging.Lanczos)
}

func avatarPlaceholder(size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{255, 255, 255, 40}}, image.Point{}, draw.Src)
	return img
}

func drawCircleImage(dc *gg.Context, img image.Image, cx, cy, r float64) {
	dc.Push()
	dc.DrawCircle(cx, cy, r)
	dc.Clip()
	dc.DrawImageAnchored(img, int(cx), int(cy), 0.5, 0.5)
	dc.Pop()
	dc.SetRGB(1, 1, 1)
	dc.SetLineWidth(3)
	dc.DrawCircle(cx, cy, r)
	dc.Stroke()
}

func decodeQR(pngBytes []byte, size int) image.Image {
	if len(pngBytes) == 0 {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		img, err = imaging.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			return nil
		}
	}
	return imaging.Fit(img, size, size, imaging.Lanczos)
}

func ellipsis(dc *gg.Context, s string, maxWidth float64) string {
	if w, _ := dc.MeasureString(s); w <= maxWidth {
		return s
	}
	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		trial := string(runes[:i]) + "…"
		if w, _ := dc.MeasureString(trial); w <= maxWidth {
			return trial
		}
	}
	return "…"
}

func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
