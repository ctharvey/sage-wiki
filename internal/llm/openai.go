package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// openaiProvider implements the OpenAI-compatible API format.
type openaiProvider struct {
	apiKey  string
	baseURL string
}

func newOpenAIProvider(apiKey string, baseURL string) *openaiProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &openaiProvider{apiKey: apiKey, baseURL: baseURL}
}

func (p *openaiProvider) Name() string        { return "openai" }
func (p *openaiProvider) SupportsVision() bool { return true }

func (p *openaiProvider) FormatRequest(messages []Message, opts CallOpts) (*http.Request, error) {
	// Convert messages, handling vision content
	var apiMessages []any
	for _, m := range messages {
		if m.ImageBase64 != "" {
			apiMessages = append(apiMessages, map[string]any{
				"role": m.Role,
				"content": []map[string]any{
					{"type": "text", "text": m.Content},
					{"type": "image_url", "image_url": map[string]string{
						"url": "data:" + m.ImageMime + ";base64," + m.ImageBase64,
					}},
				},
			})
		} else {
			apiMessages = append(apiMessages, map[string]string{
				"role": m.Role, "content": m.Content,
			})
		}
	}

	body := map[string]any{
		"model":    opts.Model,
		"messages": apiMessages,
	}
	if opts.MaxTokens > 0 {
		body["max_tokens"] = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		body["temperature"] = opts.Temperature
	}

	req, err := http.NewRequest("POST", p.baseURL+"/chat/completions", jsonBody(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	return req, nil
}

func (p *openaiProvider) ParseResponse(body []byte) (*Response, error) {
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("openai: parse: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices in response")
	}

	return &Response{
		Content:    result.Choices[0].Message.Content,
		Model:      result.Model,
		TokensUsed: result.Usage.TotalTokens,
	}, nil
}
