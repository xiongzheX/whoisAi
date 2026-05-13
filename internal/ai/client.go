package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"whoisai/internal/game"
)

const defaultTimeout = 8 * time.Second

var errAIUnavailable = errors.New("ai client not configured")

type Config struct {
	APIKey    string
	BaseURL   string
	Model     string
	Timeout   time.Duration
	UserAgent string
}

func LoadConfigFromEnv() Config {
	timeout := defaultTimeout
	if raw := strings.TrimSpace(os.Getenv("AI_TIMEOUT_MS")); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}

	return Config{
		APIKey:    strings.TrimSpace(os.Getenv("AI_API_KEY")),
		BaseURL:   strings.TrimSpace(os.Getenv("AI_BASE_URL")),
		Model:     strings.TrimSpace(os.Getenv("AI_MODEL")),
		Timeout:   timeout,
		UserAgent: "whoisai-go-server/1.0",
	}
}

func (c Config) Enabled() bool {
	return c.APIKey != "" && c.BaseURL != "" && c.Model != ""
}

type Client struct {
	config     Config
	httpClient *http.Client
	logger     *log.Logger
}

func NewClient(config Config, logger *log.Logger) *Client {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	config.Timeout = timeout
	if config.UserAgent == "" {
		config.UserAgent = "whoisai-go-server/1.0"
	}
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

func (c *Client) Rewrite(message string, style game.Style) (string, error) {
	if c == nil || !c.config.Enabled() {
		return "", errAIUnavailable
	}

	payload := map[string]any{
		"model": c.config.Model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": systemPrompt(style),
			},
			{
				"role":    "user",
				"content": strings.TrimSpace(message),
			},
		},
		"temperature": 0.7,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	endpoint := strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logf("AI rewrite request failed, using fallback: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.logf("AI rewrite response decode failed, using fallback: %v", err)
		return "", err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if result.Error != nil && result.Error.Message != "" {
			err = fmt.Errorf("ai rewrite request failed: %s", result.Error.Message)
		} else {
			err = fmt.Errorf("ai rewrite request failed: status %d", resp.StatusCode)
		}
		c.logf("%v, using fallback", err)
		return "", err
	}
	if len(result.Choices) == 0 {
		err = errors.New("ai rewrite response missing choices")
		c.logf("%v, using fallback", err)
		return "", err
	}

	rewritten := strings.TrimSpace(result.Choices[0].Message.Content)
	if rewritten == "" {
		err = errors.New("ai rewrite response empty")
		c.logf("%v, using fallback", err)
		return "", err
	}
	return rewritten, nil
}

func (c *Client) logf(format string, args ...any) {
	if c != nil && c.logger != nil {
		c.logger.Printf(format, args...)
	}
}

func systemPrompt(style game.Style) string {
	base := "你在一个阿瓦隆式社交推理游戏中，负责把玩家原话轻微改写成看起来像 AI 干扰后的版本。只输出改写后的单句，不要解释，不要加引号，不要改变核心立场，不要扩写超过 30 个字。"
	switch style {
	case game.StylePolite:
		return base + "风格要求：更委婉、更像试探。"
	case game.StyleVerbose:
		return base + "风格要求：略微冗长，带一点理由感。"
	case game.StyleNeutral:
		return base + "风格要求：语气更中性，把过于绝对的话说得保守一点。"
	case game.StyleAwkward:
		return base + "风格要求：有点不自然，像系统腔，但仍然像人类在说话。"
	default:
		return base
	}
}
