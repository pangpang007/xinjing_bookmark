package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/sashabaranov/go-openai"
	"github.com/soupcircle/bookjie-api/models"
	"github.com/soupcircle/bookjie-api/utils"
)

const (
	recommendMaxAvoid = 8
	previewRunes      = 40
)

type AvoidWork struct {
	BookName       string
	Author         string
	LiteratureText string
}

type DeepSeek struct {
	client *openai.Client
}

func NewDeepSeek(apiKey, baseURL string) *DeepSeek {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &DeepSeek{client: openai.NewClientWithConfig(cfg)}
}

func (s *DeepSeek) Recommend(ctx context.Context, mood string, avoid []AvoidWork) *models.LiteratureResponse {
	if len(avoid) > recommendMaxAvoid {
		avoid = avoid[:recommendMaxAvoid]
	}
	if s == nil || s.client == nil {
		return pickFallback(avoid)
	}

	temp := float32(0.7)
	if len(avoid) > 0 {
		temp = 0.95
	}

	result, err := s.recommendOnce(ctx, mood, avoid, temp)
	if err != nil {
		log.Printf("[WARN] deepseek first attempt failed: %v", err)
		result, err = s.recommendOnce(ctx, mood, avoid, temp)
	}
	if err != nil {
		log.Printf("[WARN] deepseek retry failed, using default: %v", err)
		return pickFallback(avoid)
	}
	if !isDuplicate(result, avoid) {
		return result
	}

	log.Printf("[WARN] deepseek repeated a previous work %q, retrying", result.BookName)
	retry, retryErr := s.recommendOnce(ctx, mood, avoid, 1.0)
	if retryErr == nil && !isDuplicate(retry, avoid) {
		return retry
	}
	if retryErr != nil {
		log.Printf("[WARN] deepseek variety retry failed: %v", retryErr)
	}
	return pickFallback(avoid)
}

func (s *DeepSeek) recommendOnce(ctx context.Context, mood string, avoid []AvoidWork, temperature float32) (*models.LiteratureResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       "deepseek-chat",
		Temperature: temperature,
		MaxTokens:   1024,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: buildRecommendPrompt(mood, avoid),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices")
	}

	raw := utils.ExtractJSON(resp.Choices[0].Message.Content)
	var parsed models.LiteratureResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	parsed.LiteratureText = strings.TrimSpace(parsed.LiteratureText)
	parsed.BookName = strings.TrimSpace(parsed.BookName)
	parsed.Author = strings.TrimSpace(parsed.Author)
	parsed.Style = models.NormalizeStyle(strings.ToLower(strings.TrimSpace(parsed.Style)))

	if parsed.LiteratureText == "" || parsed.BookName == "" || parsed.Author == "" {
		return nil, fmt.Errorf("incomplete literature payload")
	}
	return &parsed, nil
}

func buildRecommendPrompt(mood string, avoid []AvoidWork) string {
	var b strings.Builder
	b.WriteString(`你是一个文学推荐助手。用户会告诉你他当下的心情，你需要从中国文学作品中推荐一段最契合的原文。

要求：
1. 只推荐真实存在的中国文学作品（现代文学、古典文学、散文、诗歌均可）
2. 推荐的片段要能慰藉、共鸣或启发用户
3. 返回JSON格式，包含以下字段：
   - literature_text: 文学原文（100-300字）
   - book_name: 书名或篇名
   - author: 作者
   - style: 风格建议（warm/melancholy/nostalgic/hopeful之一）

用户心情：`)
	b.WriteString(mood)

	if len(avoid) > 0 {
		b.WriteString("\n\n用户已经看过下面这些推荐。必须换一篇完全不同的作品：书名/篇名不能相同，原文也不能相同或高度相似。不要只改几个字，不要继续用同一篇。\n")
		for i, w := range avoid {
			title := strings.TrimSpace(w.BookName)
			author := strings.TrimSpace(w.Author)
			fmt.Fprintf(&b, "%d. 《%s》 %s", i+1, title, author)
			if preview := runePreview(w.LiteratureText, previewRunes); preview != "" {
				fmt.Fprintf(&b, " — %s", preview)
			}
			b.WriteByte('\n')
		}
	}

	b.WriteString("\n返回JSON：")
	return b.String()
}

func isDuplicate(got *models.LiteratureResponse, avoid []AvoidWork) bool {
	if got == nil {
		return false
	}
	gotTitle := normTitle(got.BookName)
	gotText := compactText(got.LiteratureText)
	for _, a := range avoid {
		if gotTitle != "" && gotTitle == normTitle(a.BookName) {
			return true
		}
		if gotText != "" && gotText == compactText(a.LiteratureText) {
			return true
		}
	}
	return false
}

func pickFallback(avoid []AvoidWork) *models.LiteratureResponse {
	for i := range fallbackLiteratures {
		item := fallbackLiteratures[i]
		if !isDuplicate(&item, avoid) {
			cp := item
			return &cp
		}
	}
	cp := fallbackLiteratures[len(fallbackLiteratures)-1]
	return &cp
}

func normTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "《")
	s = strings.TrimSuffix(s, "》")
	return strings.TrimSpace(s)
}

func compactText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func runePreview(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) == 0 {
		return ""
	}
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

var fallbackLiteratures = []models.LiteratureResponse{
	{
		LiteratureText: models.DefaultLiterature.LiteratureText,
		BookName:       models.DefaultLiterature.BookName,
		Author:         models.DefaultLiterature.Author,
		Style:          models.DefaultLiterature.Style,
	},
	{
		LiteratureText: "我与父亲不相见已二年余了，我最不能忘记的是他的背影。那年冬天，祖母死了，父亲的差使也交卸了，正是祸不单行的日子。我从北京到徐州，打算跟着父亲奔丧回家。到徐州见着父亲，看见满院狼藉的东西，又想起祖母，不禁簌簌地流下眼泪。",
		BookName:       "背影",
		Author:         "朱自清",
		Style:          "warm",
	},
	{
		LiteratureText: "轻轻的我走了，正如我轻轻的来；我轻轻的招手，作别西天的云彩。那河畔的金柳，是夕阳中的新娘；波光里的艳影，在我的心头荡漾。软泥上的青荇，油油的在水底招摇；在康河的柔波里，我甘心做一条水草！",
		BookName:       "再别康桥",
		Author:         "徐志摩",
		Style:          "melancholy",
	},
}
