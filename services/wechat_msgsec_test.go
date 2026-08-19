package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func testWeChat(rt http.RoundTripper) *WeChat {
	return NewWeChatForTest("test-token", rt)
}

func TestMsgSecBlocked(t *testing.T) {
	t.Parallel()
	cases := []struct {
		suggest string
		want    bool
	}{
		{"pass", false},
		{"PASS", false},
		{"risky", true},
		{"RISKY", true},
		{"review", true},
		{"", false},
		{"ok", false},
	}
	for _, tc := range cases {
		if got := MsgSecBlocked(tc.suggest); got != tc.want {
			t.Errorf("suggest=%q got=%v want=%v", tc.suggest, got, tc.want)
		}
	}
}

func TestMsgSecCheckUserMsg(t *testing.T) {
	t.Parallel()
	cases := []struct {
		label int
		msg   string
	}{
		{10001, "这段内容像推广信息，请只写下你的心情"},
		{20003, "请换一种更平和的说法"},
		{20001, "这段内容不便使用，请换一种说法"},
		{20002, "这段内容不便使用，请换一种说法"},
		{20006, "这段内容不便使用，请换一种说法"},
		{20008, "这段内容不便使用，请换一种说法"},
		{20012, "这段内容不便使用，请换一种说法"},
		{20013, "这段内容不便使用，请换一种说法"},
		{21000, "这段内容无法使用，请换一种心情"},
		{100, "这段内容无法使用，请换一种心情"},
		{0, "这段内容无法使用，请换一种心情"},
	}
	for _, tc := range cases {
		if got := MsgSecCheckUserMsg(tc.label); got != tc.msg {
			t.Errorf("label=%d got=%q want=%q", tc.label, got, tc.msg)
		}
	}
}

func TestParseMsgSecCheck(t *testing.T) {
	t.Parallel()
	suggest, label, err := parseMsgSecCheck([]byte(`{"errcode":0,"errmsg":"ok","result":{"suggest":"risky","label":10001}}`))
	if err != nil {
		t.Fatal(err)
	}
	if suggest != "risky" || label != 10001 {
		t.Fatalf("suggest=%q label=%d", suggest, label)
	}

	_, _, err = parseMsgSecCheck([]byte(`{"errcode":40001,"errmsg":"invalid credential"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "40001") {
		t.Fatalf("err=%v", err)
	}

	_, _, err = parseMsgSecCheck([]byte(`not-json`))
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestMsgSecCheckPass(t *testing.T) {
	t.Parallel()
	w := testWeChat(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/wxa/msg_sec_check" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.URL.Query().Get("access_token") != "test-token" {
			t.Fatalf("token=%s", r.URL.Query().Get("access_token"))
		}
		var req msgSecCheckReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Content != "想爸爸了" || req.Version != 2 || req.Scene != 4 || req.OpenID != "oTEST" {
			t.Fatalf("req=%+v", req)
		}
		return jsonResponse(`{"errcode":0,"errmsg":"ok","result":{"suggest":"pass","label":100}}`), nil
	}))

	suggest, label, err := w.MsgSecCheck(context.Background(), "oTEST", "想爸爸了")
	if err != nil {
		t.Fatal(err)
	}
	if suggest != "pass" || label != 100 {
		t.Fatalf("suggest=%q label=%d", suggest, label)
	}
}

func TestMsgSecCheckRisky(t *testing.T) {
	t.Parallel()
	w := testWeChat(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`{"errcode":0,"errmsg":"ok","result":{"suggest":"review","label":20003}}`), nil
	}))
	suggest, label, err := w.MsgSecCheck(context.Background(), "oTEST", "骂人")
	if err != nil {
		t.Fatal(err)
	}
	if !MsgSecBlocked(suggest) || label != 20003 {
		t.Fatalf("suggest=%q label=%d", suggest, label)
	}
}

func TestMsgSecCheckTokenFail(t *testing.T) {
	t.Parallel()
	w := &WeChat{
		httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if strings.Contains(r.URL.Path, "token") {
				return jsonResponse(`{"errcode":40001,"errmsg":"invalid appsecret"}`), nil
			}
			t.Fatal("should not call msg_sec_check")
			return nil, nil
		}), Timeout: time.Second},
	}
	_, _, err := w.MsgSecCheck(context.Background(), "oTEST", "想爸爸了")
	if err == nil {
		t.Fatal("expected token error")
	}
}

func TestMsgSecCheckTimeout(t *testing.T) {
	t.Parallel()
	w := testWeChat(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	}))
	_, _, err := w.MsgSecCheck(context.Background(), "oTEST", "买一送一")
	if err == nil {
		t.Fatal("expected timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("err=%v", err)
	}
}
