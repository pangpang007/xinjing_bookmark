package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/soupcircle/bookjie-api/config"
	"github.com/soupcircle/bookjie-api/middleware"
	"github.com/soupcircle/bookjie-api/services"
	"github.com/soupcircle/bookjie-api/utils"
)

func testInterpretRouter(h *Handler, jwt *middleware.JWTAuth) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/interpret", jwt.Required(), h.Interpret)
	return r
}

func TestInterpretRequiresLogin(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	h := &Handler{
		cfg:      &config.Config{InterpretDailyLimit: 3, Timezone: time.UTC},
		deepseek: &services.DeepSeek{},
	}
	r := testInterpretRouter(h, jwt)

	req := httptest.NewRequest(http.MethodPost, "/interpret", strings.NewReader(`{"mood":"想爸爸了"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("http %d", w.Code)
	}
	var body utils.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != utils.ErrCodeJWTInvalid {
		t.Fatalf("code=%d", body.Code)
	}
}

func TestInterpretParamErrorDoesNotConsumeQuota(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(9)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		cfg:      &config.Config{InterpretDailyLimit: 3, Timezone: time.UTC},
		deepseek: &services.DeepSeek{},
		quota:    services.NewMemoryInterpretQuota(time.UTC, 3),
	}
	r := testInterpretRouter(h, jwt)

	req := httptest.NewRequest(http.MethodPost, "/interpret", strings.NewReader(`{"mood":""}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body utils.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != utils.ErrCodeParamInvalid {
		t.Fatalf("code=%d", body.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/interpret", strings.NewReader(`{"mood":"想爸爸了"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 0 {
		t.Fatalf("code=%d body=%s", body.Code, w.Body.String())
	}
	data, _ := json.Marshal(body.Data)
	var payload struct {
		Quota struct {
			Used int `json:"used"`
		} `json:"quota"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Quota.Used != 1 {
		t.Fatalf("used=%d, param error should not consume quota", payload.Quota.Used)
	}
}

func TestInterpretSuccessAttachesQuota(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(11)
	if err != nil {
		t.Fatal(err)
	}

	h := &Handler{
		cfg:      &config.Config{InterpretDailyLimit: 3, Timezone: time.UTC},
		deepseek: &services.DeepSeek{},
		quota:    services.NewMemoryInterpretQuota(time.UTC, 3),
	}
	r := testInterpretRouter(h, jwt)

	req := httptest.NewRequest(http.MethodPost, "/interpret", bytes.NewReader([]byte(`{"mood":"想爸爸了"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body struct {
		Code int `json:"code"`
		Data struct {
			LiteratureText string `json:"literature_text"`
			Quota          struct {
				Used      int `json:"used"`
				Limit     int `json:"limit"`
				Remaining int `json:"remaining"`
			} `json:"quota"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 0 {
		t.Fatalf("code=%d body=%s", body.Code, w.Body.String())
	}
	if body.Data.LiteratureText == "" {
		t.Fatal("missing literature")
	}
	if body.Data.Quota.Used != 1 || body.Data.Quota.Limit != 3 || body.Data.Quota.Remaining != 2 {
		t.Fatalf("quota=%+v", body.Data.Quota)
	}
}

func TestInterpretQuotaExceeded(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(12)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		cfg:      &config.Config{InterpretDailyLimit: 3, Timezone: time.UTC},
		deepseek: &services.DeepSeek{},
		quota:    services.NewMemoryInterpretQuota(time.UTC, 3),
	}
	r := testInterpretRouter(h, jwt)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/interpret", strings.NewReader(`{"mood":"想爸爸了"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var body utils.Body
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Code != 0 {
			t.Fatalf("i=%d code=%d", i, body.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/interpret", strings.NewReader(`{"mood":"再来一段"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body utils.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("http %d", w.Code)
	}
	if body.Code != utils.ErrCodeQuotaExceeded {
		t.Fatalf("code=%d body=%s", body.Code, w.Body.String())
	}
	if body.Data != nil {
		t.Fatalf("data=%v", body.Data)
	}
}

func TestInterpretUnlimitedSkipsQuota(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(13)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		cfg:      &config.Config{InterpretDailyLimit: 0, Timezone: time.UTC},
		deepseek: &services.DeepSeek{},
	}
	r := testInterpretRouter(h, jwt)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/interpret", strings.NewReader(`{"mood":"想爸爸了"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var body utils.Body
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Code != 0 {
			t.Fatalf("i=%d code=%d", i, body.Code)
		}
	}
}
