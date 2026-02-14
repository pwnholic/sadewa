// Package transport provides HTTP and WebSocket transport implementations for exchange communication.
package transport

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"

	"sadewa/pkg/core"
)

// Client wraps a resty HTTP client with logging and configuration.
// It provides methods for making HTTP requests with automatic retries and timeouts.
type Client struct {
	client *resty.Client
	logger zerolog.Logger
	config *core.Config
}

// Response represents an HTTP response with its status code, body, and headers.
type Response struct {
	// StatusCode is the HTTP status code returned by the server.
	StatusCode int

	// Body contains the raw response body bytes.
	Body []byte

	// Headers contains the response headers as key-value pairs.
	Headers map[string]string
}

// NewClient creates a new HTTP client with the specified configuration.
// The client is configured with timeouts, retries, and JSON marshaling using sonic.
func NewClient(config *core.Config, logger zerolog.Logger) *Client {
	client := resty.New()
	client.SetTimeout(config.Timeout)
	client.SetRetryCount(config.MaxRetries)
	client.SetRetryWaitTime(config.RetryWaitMin)
	client.SetRetryMaxWaitTime(config.RetryWaitMax)
	client.SetJSONMarshaler(sonic.Marshal)
	client.SetJSONUnmarshaler(sonic.Unmarshal)

	client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		logger.Debug().
			Str("method", req.Method).
			Str("url", req.URL).
			Msg("http request")
		return nil
	})

	return &Client{
		client: client,
		logger: logger,
		config: config,
	}
}

func paramsToStringMap(params core.Params) map[string]string {
	result := make(map[string]string)
	for k, v := range params {
		switch val := v.(type) {
		case string:
			result[k] = val
		case int:
			result[k] = strconv.Itoa(val)
		case int64:
			result[k] = strconv.FormatInt(val, 10)
		case float64:
			result[k] = strconv.FormatFloat(val, 'f', -1, 64)
		case bool:
			result[k] = strconv.FormatBool(val)
		default:
			result[k] = fmt.Sprintf("%v", val)
		}
	}
	return result
}

// Do executes an HTTP request and returns the response.
// It sets headers, query parameters, and body from the request object.
func (c *Client) Do(ctx context.Context, req *core.Request) (*Response, error) {
	r := c.client.R().SetContext(ctx)

	for k, v := range req.Headers {
		r.SetHeader(k, v)
	}

	if req.Query != nil {
		r.SetQueryParams(paramsToStringMap(req.Query))
	}

	if req.Body != nil {
		r.SetBody(req.Body)
	}

	var resp *resty.Response
	var err error

	switch req.Method {
	case "GET":
		resp, err = r.Get(req.Path)
	case "POST":
		resp, err = r.Post(req.Path)
	case "PUT":
		resp, err = r.Put(req.Path)
	case "DELETE":
		resp, err = r.Delete(req.Path)
	case "PATCH":
		resp, err = r.Patch(req.Path)
	default:
		return nil, fmt.Errorf("unsupported http method: %s", req.Method)
	}

	if err != nil {
		c.logger.Error().Err(err).
			Str("method", req.Method).
			Str("path", req.Path).
			Msg("http request failed")
		return nil, fmt.Errorf("http request: %w", err)
	}

	c.logger.Debug().
		Str("method", req.Method).
		Str("path", req.Path).
		Int("status", resp.StatusCode()).
		Int("size", len(resp.Body())).
		Msg("http response")

	headers := make(map[string]string)
	for k, v := range resp.Header() {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &Response{
		StatusCode: resp.StatusCode(),
		Body:       resp.Body(),
		Headers:    headers,
	}, nil
}

// Get performs an HTTP GET request to the specified path with optional query parameters.
func (c *Client) Get(ctx context.Context, path string, query core.Params) (*Response, error) {
	req := core.NewRequest("GET", path)
	if query != nil {
		req.SetQueryParams(query)
	}
	return c.Do(ctx, req)
}

// Post performs an HTTP POST request to the specified path with the given body.
func (c *Client) Post(ctx context.Context, path string, body any) (*Response, error) {
	req := core.NewRequest("POST", path).SetBody(body)
	return c.Do(ctx, req)
}

// Put performs an HTTP PUT request to the specified path with the given body.
func (c *Client) Put(ctx context.Context, path string, body any) (*Response, error) {
	req := core.NewRequest("PUT", path).SetBody(body)
	return c.Do(ctx, req)
}

// Delete performs an HTTP DELETE request to the specified path.
func (c *Client) Delete(ctx context.Context, path string) (*Response, error) {
	req := core.NewRequest("DELETE", path)
	return c.Do(ctx, req)
}

// SetBaseURL sets the base URL for all subsequent requests.
func (c *Client) SetBaseURL(url string) {
	c.client.SetBaseURL(url)
}

// SetHeader sets a default header for all subsequent requests.
func (c *Client) SetHeader(key, value string) {
	c.client.SetHeader(key, value)
}

// SetTimeout sets the request timeout duration.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.client.SetTimeout(timeout)
}

// SetRetry configures the retry behavior for failed requests.
func (c *Client) SetRetry(count int, waitMin, waitMax time.Duration) {
	c.client.SetRetryCount(count)
	c.client.SetRetryWaitTime(waitMin)
	c.client.SetRetryMaxWaitTime(waitMax)
}

// IsSuccess returns true if the response status code indicates success (2xx).
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// IsError returns true if the response status code indicates an error (4xx or 5xx).
func (r *Response) IsError() bool {
	return r.StatusCode >= http.StatusBadRequest
}

// Unmarshal parses the response body into the provided value using sonic.
func (r *Response) Unmarshal(v any) error {
	return sonic.Unmarshal(r.Body, v)
}
