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

// Client wraps a resty.Client with additional functionality for HTTP requests.
type Client struct {
	client    *resty.Client
	logger    zerolog.Logger
	validator *validator.Validate
	mu        sync.RWMutex
	closed    bool
}

// Config holds configuration options for an HTTP client.
type Config struct {
	// BaseURL is the root URL for all requests made by this client.
	BaseURL string `validate:"required,url"`
	// Timeout is the maximum duration for a complete request including retries.
	Timeout time.Duration `validate:"min=1ms"`
	// MaxRetries is the maximum number of retry attempts for failed requests.
	MaxRetries int `validate:"min=0"`
	// RetryWaitMin is the minimum wait time between retry attempts.
	RetryWaitMin time.Duration `validate:"min=0"`
	// RetryWaitMax is the maximum wait time between retry attempts.
	RetryWaitMax time.Duration `validate:"min=0"`
	// Headers contains default headers to include in all requests.
	Headers map[string]string `validate:"omitempty"`
}

// RequestOption is a function that modifies a resty.Request.
type RequestOption func(*resty.Request)

// NewClient creates a new HTTP client with the given configuration.
// It returns an error if the configuration validation fails.
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

// Close releases resources used by the client. It must be called when the client is no longer needed.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.client.Close()
}

// Request creates a new request object for custom request building.
func (c *Client) Request() *resty.Request {
	return c.client.R()
}

// Get performs an HTTP GET request to the specified URL with optional request options.
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

// Post performs an HTTP POST request to the specified URL with the given body and optional request options.
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

// Delete performs an HTTP DELETE request to the specified URL with optional request options.
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

// WithHeader returns a RequestOption that sets a single header on the request.
func WithHeader(key, value string) RequestOption {
	return func(r *resty.Request) {
		r.SetHeader(key, value)
	}
}

// WithHeaders returns a RequestOption that sets multiple headers on the request.
func WithHeaders(headers map[string]string) RequestOption {
	return func(r *resty.Request) {
		r.SetHeaders(headers)
	}
}

// WithQueryParam returns a RequestOption that sets a single query parameter on the request.
func WithQueryParam(key, value string) RequestOption {
	return func(r *resty.Request) {
		r.SetQueryParam(key, value)
	}
}

// WithQueryParams returns a RequestOption that sets multiple query parameters on the request.
func WithQueryParams(params map[string]string) RequestOption {
	return func(r *resty.Request) {
		r.SetQueryParams(params)
	}
}

// WithResult returns a RequestOption that sets the result object for automatic unmarshaling.
func WithResult(res any) RequestOption {
	return func(r *resty.Request) {
		r.SetResult(res)
	}
}

// WithError returns a RequestOption that sets the error object for automatic unmarshaling of error responses.
func WithError(err any) RequestOption {
	return func(r *resty.Request) {
		r.SetError(err)
	}
}
