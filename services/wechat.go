package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	wechatAccessTokenKey = "bookjie:wechat:access_token"
	wechatHTTPTimeout    = 12 * time.Second
)

type WeChat struct {
	appID      string
	appSecret  string
	qrPage     string
	envVersion string
	httpClient *http.Client
	rdb        *redis.Client
	memToken   string
	memExpiry  time.Time
}

type jsCode2SessionResp struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

type accessTokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

func NewWeChat(appID, appSecret, qrPage, envVersion string, rdb *redis.Client) *WeChat {
	return &WeChat{
		appID:      appID,
		appSecret:  appSecret,
		qrPage:     qrPage,
		envVersion: envVersion,
		httpClient: &http.Client{Timeout: wechatHTTPTimeout},
		rdb:        rdb,
	}
}

func (w *WeChat) Code2OpenID(ctx context.Context, code string) (string, error) {
	if w.appID == "" || w.appSecret == "" {
		return "", fmt.Errorf("wechat app secret not configured")
	}

	u := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		w.appID, w.appSecret, code,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed jsCode2SessionResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode jscode2session: %w", err)
	}
	if parsed.ErrCode != 0 {
		return "", fmt.Errorf("wechat err %d: %s", parsed.ErrCode, parsed.ErrMsg)
	}
	if parsed.OpenID == "" {
		return "", fmt.Errorf("empty openid")
	}
	return parsed.OpenID, nil
}

func (w *WeChat) MiniProgramCode(ctx context.Context, scene string) ([]byte, error) {
	token, err := w.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"scene":       scene,
		"width":       280,
		"check_path":  false,
		"env_version": w.envVersion,
		"auto_color":  false,
		"line_color":  map[string]int{"r": 0, "g": 0, "b": 0},
		"is_hyaline":  true,
	}
	if w.qrPage != "" {
		payload["page"] = w.qrPage
	}

	raw, _ := json.Marshal(payload)
	u := "https://api.weixin.qq.com/wxa/getwxacodeunlimit?access_token=" + token
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(body) > 0 && body[0] == '{' {
		var wxErr struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		_ = json.Unmarshal(body, &wxErr)
		return nil, fmt.Errorf("wxacode err %d: %s", wxErr.ErrCode, wxErr.ErrMsg)
	}
	if len(body) < 100 {
		return nil, fmt.Errorf("wxacode too small")
	}
	return body, nil
}

func (w *WeChat) accessToken(ctx context.Context) (string, error) {
	if token, ok := w.cachedToken(ctx); ok {
		return token, nil
	}

	u := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		w.appID, w.appSecret,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed accessTokenResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.ErrCode != 0 || parsed.AccessToken == "" {
		return "", fmt.Errorf("access_token err %d: %s", parsed.ErrCode, parsed.ErrMsg)
	}

	ttl := time.Duration(parsed.ExpiresIn)*time.Second - 5*time.Minute
	if ttl < time.Minute {
		ttl = time.Minute
	}
	w.storeToken(ctx, parsed.AccessToken, ttl)
	return parsed.AccessToken, nil
}

func (w *WeChat) cachedToken(ctx context.Context) (string, bool) {
	if w.rdb != nil {
		val, err := w.rdb.Get(ctx, wechatAccessTokenKey).Result()
		if err == nil && val != "" {
			return val, true
		}
	}
	if w.memToken != "" && time.Now().Before(w.memExpiry) {
		return w.memToken, true
	}
	return "", false
}

func (w *WeChat) storeToken(ctx context.Context, token string, ttl time.Duration) {
	w.memToken = token
	w.memExpiry = time.Now().Add(ttl)
	if w.rdb != nil {
		if err := w.rdb.Set(ctx, wechatAccessTokenKey, token, ttl).Err(); err != nil {
			log.Printf("[WARN] redis set access_token: %v", err)
		}
	}
}
