package quotes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL    = "http://api.forismatic.com/api/1.0/"
	defaultLanguage   = "ru"
	defaultRetryCount = 2
	defaultRetryDelay = 500 * time.Millisecond
	fallbackAuthor    = "Неизвестный автор"
)

// Quote описывает цитату, полученную из внешнего источника.
type Quote struct {
	Text   string
	Author string
}

// HTTPClient определяет поведение http.Client, необходимое для клиента Forismatic.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Config содержит параметры инициализации клиента Forismatic.
type Config struct {
	BaseURL    string
	Language   string
	RetryCount int
	RetryDelay time.Duration
}

// Client инкапсулирует доступ к API Forismatic.
type Client struct {
	httpClient HTTPClient
	baseURL    *url.URL
	language   string
	retryCount int
	retryDelay time.Duration
}

// NewClient создает новый клиент Forismatic с заданной конфигурацией.
func NewClient(httpClient HTTPClient, cfg Config) (*Client, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}

	language := cfg.Language
	if language == "" {
		language = defaultLanguage
	}

	retryCount := cfg.RetryCount
	if retryCount < 0 {
		retryCount = defaultRetryCount
	}

	retryDelay := cfg.RetryDelay
	if retryDelay <= 0 {
		retryDelay = defaultRetryDelay
	}

	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    parsedURL,
		language:   language,
		retryCount: retryCount,
		retryDelay: retryDelay,
	}, nil
}

// GetQuote возвращает случайную цитату из сервиса Forismatic.
func (c *Client) GetQuote(ctx context.Context) (Quote, error) {
	var lastErr error
	attempts := c.retryCount + 1

	for attempt := 0; attempt < attempts; attempt++ {
		quote, err := c.fetchQuote(ctx)
		if err == nil {
			return quote, nil
		}

		// Если контекст отменен, прекращаем попытки немедленно.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Quote{}, err
		}

		lastErr = err

		// Ждем перед следующей попыткой, если она еще возможна.
		if attempt < attempts-1 {
			delay := c.retryDelay * time.Duration(1<<attempt)
			if err := c.waitWithContext(ctx, delay); err != nil {
				return Quote{}, err
			}
		}
	}

	if lastErr == nil {
		lastErr = errors.New("не удалось получить цитату")
	}

	return Quote{}, lastErr
}

func (c *Client) waitWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) fetchQuote(ctx context.Context) (Quote, error) {
	u := *c.baseURL
	query := u.Query()
	query.Set("method", "getQuote")
	query.Set("format", "json")
	query.Set("lang", c.language)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Quote{}, fmt.Errorf("создание запроса: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Quote{}, fmt.Errorf("вызов forismatic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Quote{}, fmt.Errorf("forismatic вернул статус %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Text   string `json:"quoteText"`
		Author string `json:"quoteAuthor"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Quote{}, fmt.Errorf("чтение ответа forismatic: %w", err)
	}

	payload.Text = strings.TrimSpace(payload.Text)
	payload.Author = strings.TrimSpace(payload.Author)

	if payload.Text == "" {
		return Quote{}, errors.New("пустой текст цитаты")
	}

	if payload.Author == "" {
		payload.Author = fallbackAuthor
	}

	return Quote{
		Text:   payload.Text,
		Author: payload.Author,
	}, nil
}
