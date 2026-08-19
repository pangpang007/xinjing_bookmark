package handlers

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/soupcircle/bookjie-api/config"
	"github.com/soupcircle/bookjie-api/middleware"
	"github.com/soupcircle/bookjie-api/utils"
)

func tinyJPEG() []byte {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func shareRouter(h *Handler, jwt *middleware.JWTAuth) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/share-image", jwt.Required(), h.ShareImage)
	return r
}

func TestShareImageRequiresLogin(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	r := shareRouter(&Handler{cfg: &config.Config{PublicBaseURL: "https://api.soupcircle.xyz/bookmark"}}, jwt)
	req := httptest.NewRequest(http.MethodPost, "/share-image", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body utils.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != utils.ErrCodeJWTInvalid {
		t.Fatalf("code=%d", body.Code)
	}
}

func TestShareImageLegacyJSON(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(1)
	if err != nil {
		t.Fatal(err)
	}
	r := shareRouter(&Handler{cfg: &config.Config{}}, jwt)
	req := httptest.NewRequest(http.MethodPost, "/share-image", strings.NewReader(`{"literature_text":"a"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body utils.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != utils.ErrCodeParamInvalid || body.Msg != "请升级后重试" {
		t.Fatalf("code=%d msg=%q", body.Code, body.Msg)
	}
}

func TestShareImageMissingFile(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(1)
	if err != nil {
		t.Fatal(err)
	}
	r := shareRouter(&Handler{cfg: &config.Config{}}, jwt)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("mood", "今天有些犹豫")
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/share-image", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body utils.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != utils.ErrCodeParamInvalid || body.Msg != "请上传分享图" {
		t.Fatalf("code=%d msg=%q", body.Code, body.Msg)
	}
}

func TestShareImageRejectsNonImage(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(1)
	if err != nil {
		t.Fatal(err)
	}
	r := shareRouter(&Handler{cfg: &config.Config{}}, jwt)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("image", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("not-an-image"))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/share-image", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body utils.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != utils.ErrCodeParamInvalid {
		t.Fatalf("code=%d msg=%q", body.Code, body.Msg)
	}
}

func TestShareImageTooLarge(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(1)
	if err != nil {
		t.Fatal(err)
	}
	r := shareRouter(&Handler{cfg: &config.Config{}}, jwt)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("image", "big.jpg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(tinyJPEG())
	_, _ = part.Write(bytes.Repeat([]byte{0xff}, 2<<20))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/share-image", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body utils.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != utils.ErrCodeImageTooLarge {
		t.Fatalf("code=%d msg=%q body=%s", body.Code, body.Msg, w.Body.String())
	}
}

func TestPublicWxaCodeURL(t *testing.T) {
	h := &Handler{cfg: &config.Config{PublicBaseURL: "https://api.soupcircle.xyz/bookmark"}}
	if got := h.publicWxaCodeURL(); got != "https://api.soupcircle.xyz/bookmark/wxacode/share.png" {
		t.Fatalf("got %q", got)
	}
}
