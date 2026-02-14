package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"

	"sadewa/pkg/core"
)

var _ = context.Background

type MockProtocol struct {
	name              string
	version           string
	baseURL           string
	buildRequestFunc  func(ctx context.Context, op core.Operation, params core.Params) (*core.Request, error)
	parseResponseFunc func(op core.Operation, resp *resty.Response) (any, error)
	signRequestFunc   func(req *resty.Request, creds core.Credentials) error
	supportedOps      []core.Operation
	rateLimits        core.RateLimitConfig
}

func (m *MockProtocol) Name() string {
	return m.name
}

func (m *MockProtocol) Version() string {
	return m.version
}

func (m *MockProtocol) BaseURL(sandbox bool) string {
	return m.baseURL
}

func (m *MockProtocol) BuildRequest(ctx context.Context, op core.Operation, params core.Params) (*core.Request, error) {
	if m.buildRequestFunc != nil {
		return m.buildRequestFunc(ctx, op, params)
	}
	return core.NewRequest("GET", "/test"), nil
}

func (m *MockProtocol) ParseResponse(op core.Operation, resp *resty.Response) (any, error) {
	if m.parseResponseFunc != nil {
		return m.parseResponseFunc(op, resp)
	}
	return nil, nil
}

func (m *MockProtocol) SignRequest(req *resty.Request, creds core.Credentials) error {
	if m.signRequestFunc != nil {
		return m.signRequestFunc(req, creds)
	}
	return nil
}

func (m *MockProtocol) SupportedOperations() []core.Operation {
	return m.supportedOps
}

func (m *MockProtocol) RateLimits() core.RateLimitConfig {
	return m.rateLimits
}

func TestNewSession(t *testing.T) {
	tests := []struct {
		name    string
		config  *core.Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  core.DefaultConfig("test"),
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "invalid config - empty exchange",
			config: &core.Config{
				Timeout:           10 * time.Second,
				RateLimitRequests: 1200,
				RateLimitPeriod:   time.Minute,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := New(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, session)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, session)
			assert.Equal(t, StateNew, session.State())
			assert.NotNil(t, session.CreatedAt())
			assert.NotNil(t, session.LastUsed())
		})
	}
}

func TestSession_SetProtocol(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	protocol := &MockProtocol{
		name:    "mock",
		version: "1.0",
		baseURL: "https://api.mock.com",
	}

	err = session.SetProtocol(protocol)
	assert.NoError(t, err)
	assert.Equal(t, StateActive, session.State())
	assert.Equal(t, protocol, session.Protocol())
}

func TestSession_SetProtocol_Nil(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	err = session.SetProtocol(nil)
	assert.Error(t, err)
	assert.Equal(t, StateNew, session.State())
}

func TestSession_State(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	assert.Equal(t, StateNew, session.State())

	protocol := &MockProtocol{name: "mock"}
	session.SetProtocol(protocol)
	assert.Equal(t, StateActive, session.State())

	session.Close()
	assert.Equal(t, StateClosed, session.State())
}

func TestSession_Close(t *testing.T) {
	config := core.DefaultConfig("test")
	config.CacheEnabled = true
	session, err := New(config)
	assert.NoError(t, err)

	protocol := &MockProtocol{name: "mock"}
	session.SetProtocol(protocol)

	err = session.Close()
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, session.State())
}

func TestSession_Do_NoProtocol(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	_, err = session.Do(context.Background(), core.OpGetTicker, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "protocol not set")
}

func TestSession_Do_BuildRequestError(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	protocol := &MockProtocol{
		name: "mock",
		buildRequestFunc: func(ctx context.Context, op core.Operation, params core.Params) (*core.Request, error) {
			return nil, errors.New("build error")
		},
	}
	session.SetProtocol(protocol)

	_, err = session.Do(context.Background(), core.OpGetTicker, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "build request")
}

func TestCache_GetSet(t *testing.T) {
	cache := NewCache(1 * time.Second)
	ctx := context.Background()

	err := cache.Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err)

	val, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", val)
}

func TestCache_Get_NotExists(t *testing.T) {
	cache := NewCache(1 * time.Second)
	ctx := context.Background()

	val, err := cache.Get(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestCache_Get_Expired(t *testing.T) {
	cache := NewCache(1 * time.Nanosecond)
	ctx := context.Background()

	err := cache.Set(ctx, "key1", "value1", 1*time.Nanosecond)
	assert.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	val, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestCache_Delete(t *testing.T) {
	cache := NewCache(1 * time.Second)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)

	err := cache.Delete(ctx, "key1")
	assert.NoError(t, err)

	val, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestCache_Clear(t *testing.T) {
	cache := NewCache(1 * time.Second)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	cache.Clear()

	val1, _ := cache.Get(ctx, "key1")
	val2, _ := cache.Get(ctx, "key2")

	assert.Nil(t, val1)
	assert.Nil(t, val2)
}

func TestSession_ClearCache(t *testing.T) {
	config := core.DefaultConfig("test")
	config.CacheEnabled = true
	session, err := New(config)
	assert.NoError(t, err)

	session.cache.Set(context.Background(), "key1", "value1", 0)

	session.ClearCache()

	val, _ := session.cache.Get(context.Background(), "key1")
	assert.Nil(t, val)
}

func TestSession_SetCredentials(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	creds := &core.Credentials{
		APIKey:    "test-key",
		SecretKey: "test-secret",
	}

	session.SetCredentials(creds)
	assert.Equal(t, creds, session.credentials)
}

func TestSession_Config(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	retrievedConfig := session.Config()
	assert.Equal(t, config, retrievedConfig)
}

func TestSession_CreatedAt_LastUsed(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	createdAt := session.CreatedAt()
	lastUsed := session.LastUsed()

	assert.False(t, createdAt.IsZero())
	assert.False(t, lastUsed.IsZero())
}
