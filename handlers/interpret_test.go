package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func TestInterpretEmptyMoodDoesNotCallSecCheck(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(14)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		cfg:        &config.Config{InterpretDailyLimit: 3, Timezone: time.UTC},
		deepseek:   &services.DeepSeek{},
		quota:      services.NewMemoryInterpretQuota(time.UTC, 3),
		testOpenID: "oTEST",
		wechat: services.NewWeChatForTest("tok", roundTripFunc(func(r *http.Request) (*http.Response, error) {
			t.Fatal("empty mood should not call content check")
			return nil, nil
		})),
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
}

func TestInterpretContentBlockedDoesNotConsumeQuota(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(15)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		cfg:        &config.Config{InterpretDailyLimit: 3, Timezone: time.UTC},
		deepseek:   &services.DeepSeek{},
		quota:      services.NewMemoryInterpretQuota(time.UTC, 3),
		testOpenID: "oTEST",
		wechat: services.NewWeChatForTest("tok", roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "加微信") {
				return wxJSON(`{"errcode":0,"errmsg":"ok","result":{"suggest":"risky","label":10001}}`), nil
			}
			return wxJSON(`{"errcode":0,"errmsg":"ok","result":{"suggest":"pass","label":100}}`), nil
		})),
	}
	r := testInterpretRouter(h, jwt)

	req := httptest.NewRequest(http.MethodPost, "/interpret", strings.NewReader(`{"mood":"加微信买课"}`))
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
	if body.Code != utils.ErrCodeContentBlocked {
		t.Fatalf("code=%d body=%s", body.Code, w.Body.String())
	}
	if body.Data != nil {
		t.Fatalf("data=%v", body.Data)
	}
	if body.Msg != "这段内容像推广信息，请只写下你的心情" {
		t.Fatalf("msg=%q", body.Msg)
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
		t.Fatalf("used=%d, blocked mood should not consume quota", payload.Quota.Used)
	}
}

func TestInterpretContentCheckFailOpen(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(16)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		cfg:        &config.Config{InterpretDailyLimit: 3, Timezone: time.UTC},
		deepseek:   &services.DeepSeek{},
		quota:      services.NewMemoryInterpretQuota(time.UTC, 3),
		testOpenID: "oTEST",
		wechat: services.NewWeChatForTest("tok", roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		})),
	}
	r := testInterpretRouter(h, jwt)

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
		t.Fatalf("code=%d body=%s, wechat timeout should fail-open", body.Code, w.Body.String())
	}
}

func TestInterpretContentCheckTokenFailOpen(t *testing.T) {
	jwt := middleware.NewJWTAuth("secret", time.Hour)
	token, err := jwt.Generate(17)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		cfg:        &config.Config{InterpretDailyLimit: 3, Timezone: time.UTC},
		deepseek:   &services.DeepSeek{},
		quota:      services.NewMemoryInterpretQuota(time.UTC, 3),
		testOpenID: "oTEST",
		wechat: services.NewWeChatForTest("", roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if strings.Contains(r.URL.Path, "token") {
				return wxJSON(`{"errcode":40001,"errmsg":"invalid credential"}`), nil
			}
			t.Fatal("should not call msg_sec_check after token fail")
			return nil, nil
		})),
	}
	r := testInterpretRouter(h, jwt)

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
		t.Fatalf("code=%d body=%s, token failure should fail-open", body.Code, w.Body.String())
	}
}

func TestBlockUnsafeMood(t *testing.T) {
	h := &Handler{wechat: &services.WeChat{}}
	blocked, msg := h.blockUnsafeMood(context.Background(), 1, "想爸爸了")
	if blocked {
		t.Fatalf("no openid should fail-open, msg=%q", msg)
	}

	h.testOpenID = "oTEST"
	h.wechat = services.NewWeChatForTest("tok", roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return wxJSON(`{"errcode":0,"errmsg":"ok","result":{"suggest":"risky","label":20003}}`), nil
	}))
	blocked, msg = h.blockUnsafeMood(context.Background(), 1, "骂人")
	if !blocked || msg != "请换一种更平和的说法" {
		t.Fatalf("blocked=%v msg=%q", blocked, msg)
	}
}

func TestInterpretMoodDecision(t *testing.T) {
	blocked, msg := interpretMoodDecision("pass", 100)
	if blocked || msg != "" {
		t.Fatalf("pass blocked=%v msg=%q", blocked, msg)
	}
	blocked, msg = interpretMoodDecision("risky", 10001)
	if !blocked || msg != "这段内容像推广信息，请只写下你的心情" {
		t.Fatalf("risky blocked=%v msg=%q", blocked, msg)
	}
	blocked, msg = interpretMoodDecision("review", 20003)
	if !blocked || msg != "请换一种更平和的说法" {
		t.Fatalf("review blocked=%v msg=%q", blocked, msg)
	}
	blocked, msg = interpretMoodDecision("risky", 21000)
	if !blocked || strings.Contains(msg, "21000") || strings.Contains(strings.ToLower(msg), "risky") {
		t.Fatalf("should not leak wechat label/errmsg: %q", msg)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func wxJSON(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
