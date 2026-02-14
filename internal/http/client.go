package http

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"
	"resty.dev/v3"
)

type Client struct {
	client    *resty.Client
	logger    zerolog.Logger
	validator *validator.Validate
	mu        sync.RWMutex
	closed    bool
}

type Config struct {
	BaseURL      string            `validate:"required,url"`
	Timeout      time.Duration     `validate:"min=1ms"`
	MaxRetries   int               `validate:"min=0"`
	RetryWaitMin time.Duration     `validate:"min=0"`
	RetryWaitMax time.Duration     `validate:"min=0"`
	Headers      map[string]string `validate:"omitempty"`
}

type RequestOption func(*resty.Request)

func NewClient(config *Config) (*Client, error) {
	if err := validator.New().Struct(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client := resty.New()
	client.SetBaseURL(config.BaseURL)
	client.SetTimeout(config.Timeout)
	client.SetRetryCount(config.MaxRetries)
	client.SetRetryWaitTime(config.RetryWaitMin)
	client.SetRetryMaxWaitTime(config.RetryWaitMax)
	client.AddContentTypeEncoder("application/json", func(w io.Writer, v any) error {
		data, err := sonic.Marshal(v)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	client.AddContentTypeDecoder("application/json", func(r io.Reader, v any) error {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		return sonic.Unmarshal(data, v)
	})

	for k, v := range config.Headers {
		client.SetHeader(k, v)
	}

	logger := zerolog.Nop()

	c := &Client{
		client:    client,
		logger:    logger,
		validator: validator.New(),
	}

	client.AddRequestMiddleware(func(_ *resty.Client, req *resty.Request) error {
		logger.Debug().
			Str("method", req.Method).
			Str("url", req.URL).
			Msg("http request")
		return nil
	})

	client.AddResponseMiddleware(func(_ *resty.Client, resp *resty.Response) error {
		logger.Debug().
			Str("method", resp.Request.Method).
			Str("url", resp.Request.URL).
			Int("status", resp.StatusCode()).
			Int("size", len(resp.Bytes())).
			Msg("http response")
		return nil
	})

	return c, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.client.Close()
}

func (c *Client) Request() *resty.Request {
	return c.client.R()
}

func (c *Client) Get(ctx context.Context, url string, opts ...RequestOption) (*resty.Response, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	req := c.client.R().SetContext(ctx)
	for _, opt := range opts {
		opt(req)
	}
	return req.Get(url)
}

func (c *Client) Post(ctx context.Context, url string, body any, opts ...RequestOption) (*resty.Response, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	req := c.client.R().SetContext(ctx).SetBody(body)
	for _, opt := range opts {
		opt(req)
	}
	return req.Post(url)
}

func (c *Client) Delete(ctx context.Context, url string, opts ...RequestOption) (*resty.Response, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	req := c.client.R().SetContext(ctx)
	for _, opt := range opts {
		opt(req)
	}
	return req.Delete(url)
}

func WithHeader(key, value string) RequestOption {
	return func(r *resty.Request) {
		r.SetHeader(key, value)
	}
}

func WithHeaders(headers map[string]string) RequestOption {
	return func(r *resty.Request) {
		r.SetHeaders(headers)
	}
}

func WithQueryParam(key, value string) RequestOption {
	return func(r *resty.Request) {
		r.SetQueryParam(key, value)
	}
}

func WithQueryParams(params map[string]string) RequestOption {
	return func(r *resty.Request) {
		r.SetQueryParams(params)
	}
}

func WithResult(res any) RequestOption {
	return func(r *resty.Request) {
		r.SetResult(res)
	}
}

func WithError(err any) RequestOption {
	return func(r *resty.Request) {
		r.SetError(err)
	}
}
