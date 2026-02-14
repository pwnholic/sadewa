package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"sadewa/pkg/core"
)

func TestNewClient(t *testing.T) {
	config := core.DefaultConfig("test")
	logger := zerolog.Nop()

	client := NewClient(config, logger)

	assert.NotNil(t, client)
}

func TestClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/test", r.URL.Path)
		assert.Equal(t, "value", r.URL.Query().Get("key"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"success"}`))
	}))
	defer server.Close()

	config := core.DefaultConfig("test")
	logger := zerolog.Nop()
	client := NewClient(config, logger)
	client.SetBaseURL(server.URL)

	resp, err := client.Get(context.Background(), "/test", core.Params{"key": "value"})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, resp.IsSuccess())
	assert.False(t, resp.IsError())
}

func TestClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/test", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"created":true}`))
	}))
	defer server.Close()

	config := core.DefaultConfig("test")
	logger := zerolog.Nop()
	client := NewClient(config, logger)
	client.SetBaseURL(server.URL)

	body := map[string]string{"name": "test"}
	resp, err := client.Post(context.Background(), "/test", body)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 201, resp.StatusCode)
}

func TestClient_Delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/test/123", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	config := core.DefaultConfig("test")
	logger := zerolog.Nop()
	client := NewClient(config, logger)
	client.SetBaseURL(server.URL)

	resp, err := client.Delete(context.Background(), "/test/123")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 204, resp.StatusCode)
}

func TestClient_SetTimeout(t *testing.T) {
	config := core.DefaultConfig("test")
	logger := zerolog.Nop()
	client := NewClient(config, logger)

	client.SetTimeout(30 * time.Second)
}

func TestClient_SetHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-value", r.Header.Get("X-Custom-Header"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := core.DefaultConfig("test")
	logger := zerolog.Nop()
	client := NewClient(config, logger)
	client.SetBaseURL(server.URL)
	client.SetHeader("X-Custom-Header", "test-value")

	resp, err := client.Get(context.Background(), "/test", nil)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestResponse_Unmarshal(t *testing.T) {
	resp := &Response{
		StatusCode: 200,
		Body:       []byte(`{"name":"test","value":123}`),
	}

	var result struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	err := resp.Unmarshal(&result)

	assert.NoError(t, err)
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, 123, result.Value)
}

func TestResponse_IsSuccess(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"200 OK", 200, true},
		{"201 Created", 201, true},
		{"204 No Content", 204, true},
		{"301 Redirect", 301, false},
		{"400 Bad Request", 400, false},
		{"500 Server Error", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{StatusCode: tt.statusCode}
			assert.Equal(t, tt.expected, resp.IsSuccess())
		})
	}
}

func TestResponse_IsError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"200 OK", 200, false},
		{"400 Bad Request", 400, true},
		{"404 Not Found", 404, true},
		{"500 Server Error", 500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{StatusCode: tt.statusCode}
			assert.Equal(t, tt.expected, resp.IsError())
		})
	}
}

func TestParamsToStringMap(t *testing.T) {
	params := core.Params{
		"string": "value",
		"int":    42,
		"int64":  int64(123456789),
		"float":  3.14,
		"bool":   true,
	}

	result := paramsToStringMap(params)

	assert.Equal(t, "value", result["string"])
	assert.Equal(t, "42", result["int"])
	assert.Equal(t, "123456789", result["int64"])
	assert.Equal(t, "true", result["bool"])
	assert.NotEmpty(t, result["float"])
}
