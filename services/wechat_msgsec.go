package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	msgSecCheckTimeout = 3 * time.Second
	msgSecCheckScene   = 4 // 社交日志
	msgSecCheckVersion = 2
)

type msgSecCheckReq struct {
	Content string `json:"content"`
	Version int    `json:"version"`
	Scene   int    `json:"scene"`
	OpenID  string `json:"openid"`
}

type msgSecCheckResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Result  struct {
		Suggest string `json:"suggest"`
		Label   int    `json:"label"`
	} `json:"result"`
}

func MsgSecBlocked(suggest string) bool {
	switch strings.ToLower(strings.TrimSpace(suggest)) {
	case "risky", "review":
		return true
	default:
		return false
	}
}

func MsgSecCheckUserMsg(label int) string {
	switch label {
	case 10001:
		return "这段内容像推广信息，请只写下你的心情"
	case 20003:
		return "请换一种更平和的说法"
	case 20001, 20002, 20006, 20008, 20012, 20013:
		return "这段内容不便使用，请换一种说法"
	default:
		return "这段内容无法使用，请换一种心情"
	}
}

func (w *WeChat) MsgSecCheck(ctx context.Context, openid, content string) (suggest string, label int, err error) {
	if w == nil || w.httpClient == nil {
		return "", 0, fmt.Errorf("wechat not configured")
	}

	token, err := w.accessToken(ctx)
	if err != nil {
		return "", 0, err
	}

	raw, err := json.Marshal(msgSecCheckReq{
		Content: content,
		Version: msgSecCheckVersion,
		Scene:   msgSecCheckScene,
		OpenID:  openid,
	})
	if err != nil {
		return "", 0, err
	}

	ctx, cancel := context.WithTimeout(ctx, msgSecCheckTimeout)
	defer cancel()

	u := "https://api.weixin.qq.com/wxa/msg_sec_check?access_token=" + token
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}
	return parseMsgSecCheck(body)
}

func parseMsgSecCheck(body []byte) (suggest string, label int, err error) {
	var parsed msgSecCheckResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", 0, fmt.Errorf("decode msg_sec_check: %w", err)
	}
	if parsed.ErrCode != 0 {
		return "", 0, fmt.Errorf("msg_sec_check err %d: %s", parsed.ErrCode, parsed.ErrMsg)
	}
	return parsed.Result.Suggest, parsed.Result.Label, nil
}

// NewWeChatForTest is a WeChat client with a canned token and HTTP transport.
func NewWeChatForTest(token string, rt http.RoundTripper) *WeChat {
	w := &WeChat{
		httpClient: &http.Client{Transport: rt, Timeout: time.Second},
	}
	if token != "" {
		w.memToken = token
		w.memExpiry = time.Now().Add(time.Hour)
	}
	return w
}
