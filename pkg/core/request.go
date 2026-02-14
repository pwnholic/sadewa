package core

import (
	"maps"
	"time"
)

type Params map[string]any

type Request struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Query       Params            `json:"query,omitempty"`
	Body        any               `json:"body,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Weight      int               `json:"weight"`
	CacheKey    string            `json:"cache_key,omitempty"`
	CacheTTL    time.Duration     `json:"cache_ttl,omitempty"`
	RequireAuth bool              `json:"require_auth"`
}

func NewRequest(method, path string) *Request {
	return &Request{
		Method:  method,
		Path:    path,
		Query:   make(Params),
		Headers: make(map[string]string),
		Weight:  1,
	}
}

func (r *Request) SetQuery(key string, value any) *Request {
	if r.Query == nil {
		r.Query = make(Params)
	}
	r.Query[key] = value
	return r
}

func (r *Request) SetBody(body any) *Request {
	r.Body = body
	return r
}

func (r *Request) SetHeader(key, value string) *Request {
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}
	r.Headers[key] = value
	return r
}

func (r *Request) SetWeight(weight int) *Request {
	r.Weight = weight
	return r
}

func (r *Request) SetCache(key string, ttl time.Duration) *Request {
	r.CacheKey = key
	r.CacheTTL = ttl
	return r
}

func (r *Request) SetRequireAuth(require bool) *Request {
	r.RequireAuth = require
	return r
}

func (r *Request) SetQueryParams(params Params) *Request {
	if r.Query == nil {
		r.Query = make(Params)
	}
	maps.Copy(r.Query, params)
	return r
}
