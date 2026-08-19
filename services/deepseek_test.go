package services

import (
	"strings"
	"testing"

	"github.com/soupcircle/bookjie-api/models"
)

func TestBuildRecommendPromptIncludesAvoidList(t *testing.T) {
	t.Parallel()
	prompt := buildRecommendPrompt("想爸爸了", []AvoidWork{
		{BookName: "匆匆", Author: "朱自清", LiteratureText: "燕子去了，有再来的时候"},
	})
	if !strings.Contains(prompt, "想爸爸了") {
		t.Fatal("missing mood")
	}
	if !strings.Contains(prompt, "《匆匆》") || !strings.Contains(prompt, "朱自清") {
		t.Fatalf("missing avoid work: %s", prompt)
	}
	if !strings.Contains(prompt, "必须换一篇") {
		t.Fatal("missing variety instruction")
	}
	if strings.Contains(prompt, "%s") {
		t.Fatal("unsubstituted placeholder")
	}
}

func TestBuildRecommendPromptWithoutAvoid(t *testing.T) {
	t.Parallel()
	prompt := buildRecommendPrompt("有点累", nil)
	if strings.Contains(prompt, "已经看过") {
		t.Fatalf("should not mention history: %s", prompt)
	}
}

func TestIsDuplicate(t *testing.T) {
	t.Parallel()
	avoid := []AvoidWork{
		{BookName: "《匆匆》", Author: "朱自清", LiteratureText: "燕子去了，有再来的时候"},
	}
	if !isDuplicate(&models.LiteratureResponse{BookName: "匆匆", Author: "朱自清", LiteratureText: "别的原文"}, avoid) {
		t.Fatal("same title should be duplicate")
	}
	if !isDuplicate(&models.LiteratureResponse{
		BookName:       "别的",
		Author:         "别人",
		LiteratureText: "燕子去了，有再来的时候",
	}, avoid) {
		t.Fatal("same text should be duplicate")
	}
	if isDuplicate(&models.LiteratureResponse{BookName: "背影", Author: "朱自清", LiteratureText: "我与父亲不相见已二年余了"}, avoid) {
		t.Fatal("different work should not be duplicate")
	}
}

func TestPickFallbackSkipsAvoided(t *testing.T) {
	t.Parallel()
	got := pickFallback([]AvoidWork{{BookName: "匆匆", Author: "朱自清"}})
	if got.BookName == "匆匆" {
		t.Fatalf("should skip 匆匆, got %+v", got)
	}
	got = pickFallback([]AvoidWork{
		{BookName: "匆匆"},
		{BookName: "背影"},
	})
	if got.BookName != "再别康桥" {
		t.Fatalf("got %q", got.BookName)
	}
}

func TestRecommendNilClientUsesFallback(t *testing.T) {
	t.Parallel()
	d := &DeepSeek{}
	first := d.Recommend(nil, "想爸爸了", nil)
	if first.BookName != "匆匆" {
		t.Fatalf("first fallback=%q", first.BookName)
	}
	second := d.Recommend(nil, "想爸爸了", []AvoidWork{{
		BookName:       first.BookName,
		Author:         first.Author,
		LiteratureText: first.LiteratureText,
	}})
	if second.BookName == first.BookName {
		t.Fatalf("换一段 should not reuse %q", first.BookName)
	}
}

func TestNormTitle(t *testing.T) {
	t.Parallel()
	if normTitle("《匆匆》") != "匆匆" {
		t.Fatal(normTitle("《匆匆》"))
	}
}
