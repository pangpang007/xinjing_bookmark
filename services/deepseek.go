package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/soupcircle/bookjie-api/models"
	"github.com/soupcircle/bookjie-api/utils"
)

const deepseekPrompt = `你是一个文学推荐助手。用户会告诉你他当下的心情，你需要从中国文学作品中推荐一段最契合的原文。

要求：
1. 只推荐真实存在的中国文学作品（现代文学、古典文学、散文、诗歌均可）
2. 推荐的片段要能慰藉、共鸣或启发用户
3. 返回JSON格式，包含以下字段：
   - literature_text: 文学原文（100-300字）
   - book_name: 书名或篇名
   - author: 作者
   - style: 风格建议（warm/melancholy/nostalgic/hopeful之一）

用户心情：%s

返回JSON：`

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

func (s *DeepSeek) Recommend(ctx context.Context, mood string) *models.LiteratureResponse {
	if s == nil || s.client == nil {
		return cloneDefault()
	}

	result, err := s.recommendOnce(ctx, mood)
	if err != nil {
		log.Printf("[WARN] deepseek first attempt failed: %v", err)
		result, err = s.recommendOnce(ctx, mood)
	}
	if err != nil {
		log.Printf("[WARN] deepseek retry failed, using default: %v", err)
		return cloneDefault()
	}
	return result
}

func (s *DeepSeek) recommendOnce(ctx context.Context, mood string) (*models.LiteratureResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       "deepseek-chat",
		Temperature: 0.7,
		MaxTokens:   1024,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf(deepseekPrompt, mood),
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

func cloneDefault() *models.LiteratureResponse {
	cp := *models.DefaultLiterature
	return &cp
}
