package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"resty.dev/v3"

	"sadewa/pkg/core"
)

const (
	ProductionURL = "https://api.binance.com"
	SandboxURL    = "https://testnet.binance.vision"
)

// Protocol implements the core.Protocol interface for Binance exchange.
// It provides request building, response parsing, and authentication for the Binance API.
type Protocol struct{}

// NewProtocol creates a new Binance protocol instance.
func NewProtocol() *Protocol {
	return &Protocol{}
}

// Name returns the protocol identifier "binance".
func (p *Protocol) Name() string {
	return "binance"
}

// Version returns the Binance API version string.
func (p *Protocol) Version() string {
	return "3"
}

// BaseURL returns the base URL for the Binance API.
// If sandbox is true, returns the testnet URL; otherwise returns the production URL.
func (p *Protocol) BaseURL(sandbox bool) string {
	if sandbox {
		return SandboxURL
	}
	return ProductionURL
}

// SupportedOperations returns the list of operations supported by this protocol.
func (p *Protocol) SupportedOperations() []core.Operation {
	return []core.Operation{
		core.OpGetTicker,
		core.OpGetOrderBook,
		core.OpGetTrades,
		core.OpGetKlines,
		core.OpGetBalance,
		core.OpPlaceOrder,
		core.OpCancelOrder,
		core.OpGetOrder,
		core.OpGetOpenOrders,
		core.OpGetOrderHistory,
	}
}

// RateLimits returns the rate limit configuration for Binance API.
func (p *Protocol) RateLimits() core.RateLimitConfig {
	return core.RateLimitConfig{
		RequestsPerSecond: 20,
		OrdersPerSecond:   5,
		Burst:             50,
	}
}

// BuildRequest constructs an exchange-specific HTTP request for the given operation.
// It validates required parameters and sets appropriate query parameters, weights, and caching options.
func (p *Protocol) BuildRequest(ctx context.Context, op core.Operation, params core.Params) (*core.Request, error) {
	switch op {
	case core.OpGetTicker:
		return p.buildGetTickerRequest(params)
	case core.OpGetOrderBook:
		return p.buildGetOrderBookRequest(params)
	case core.OpGetTrades:
		return p.buildGetTradesRequest(params)
	case core.OpGetKlines:
		return p.buildGetKlinesRequest(params)
	case core.OpGetBalance:
		return p.buildGetBalanceRequest(params)
	case core.OpPlaceOrder:
		return p.buildPlaceOrderRequest(params)
	case core.OpCancelOrder:
		return p.buildCancelOrderRequest(params)
	case core.OpGetOrder:
		return p.buildGetOrderRequest(params)
	case core.OpGetOpenOrders:
		return p.buildGetOpenOrdersRequest(params)
	case core.OpGetOrderHistory:
		return p.buildGetOrderHistoryRequest(params)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", op)
	}
}

// ParseResponse parses an HTTP response and normalizes it to canonical types.
// It handles Binance-specific error responses and maps them to appropriate error types.
func (p *Protocol) ParseResponse(op core.Operation, resp *resty.Response) (any, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil response")
	}

	if resp.StatusCode() >= 400 {
		var binanceErr binanceAPIError
		if err := sonic.Unmarshal(resp.Bytes(), &binanceErr); err == nil && binanceErr.Code != 0 {
			return nil, core.NewExchangeErrorWithCode(
				p.Name(),
				mapBinanceErrorCode(binanceErr.Code),
				resp.StatusCode(),
				strconv.Itoa(binanceErr.Code),
				binanceErr.Msg,
			)
		}
		return nil, core.NewExchangeError(
			p.Name(),
			core.ErrorTypeServerError,
			resp.StatusCode(),
			fmt.Sprintf("HTTP error: %s", resp.Status()),
		)
	}

	n := NewNormalizer()
	body := resp.Bytes()

	switch op {
	case core.OpGetTicker:
		var data binanceTicker
		if err := sonic.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("unmarshal ticker: %w", err)
		}
		return n.NormalizeTicker(&data), nil

	case core.OpGetOrderBook:
		var data binanceOrderBook
		if err := sonic.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("unmarshal order book: %w", err)
		}
		return n.NormalizeOrderBook(&data, "")

	case core.OpGetTrades:
		var data []binanceTrade
		if err := sonic.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("unmarshal trades: %w", err)
		}
		return n.NormalizeTrades(data, ""), nil

	case core.OpGetKlines:
		var data []binanceKline
		if err := sonic.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("unmarshal klines: %w", err)
		}
		return n.NormalizeKlines(data, "")

	case core.OpGetBalance:
		var data binanceAccount
		if err := sonic.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("unmarshal account: %w", err)
		}
		return n.NormalizeBalances(&data), nil

	case core.OpPlaceOrder, core.OpGetOrder, core.OpCancelOrder:
		var data binanceOrder
		if err := sonic.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("unmarshal order: %w", err)
		}
		return n.NormalizeOrder(&data)

	case core.OpGetOpenOrders, core.OpGetOrderHistory:
		var data []binanceOrder
		if err := sonic.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("unmarshal orders: %w", err)
		}
		return n.NormalizeOrders(data)

	default:
		var result any
		if err := sonic.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}
		return result, nil
	}
}

// SignRequest signs an HTTP request with HMAC-SHA256 authentication.
// It adds timestamp, recvWindow, and signature parameters to the request.
func (p *Protocol) SignRequest(req *resty.Request, creds core.Credentials) error {
	if creds.SecretKey == "" {
		return fmt.Errorf("secret key is required for signing")
	}

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)

	queryParams := req.QueryParams
	if queryParams == nil {
		queryParams = url.Values{}
	}
	queryParams.Set("timestamp", ts)
	queryParams.Set("recvWindow", "5000")

	queryString := queryParams.Encode()
	signature := signHMAC(queryString, creds.SecretKey)
	queryParams.Set("signature", signature)

	req.SetQueryParamsFromValues(queryParams)
	req.SetHeader("X-MBX-APIKEY", creds.APIKey)

	return nil
}

func (p *Protocol) buildGetTickerRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	req := core.NewRequest(http.MethodGet, "/api/v3/ticker/24hr")
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetWeight(2)
	req.SetCache(fmt.Sprintf("ticker:%s", symbol), 1*time.Second)

	return req, nil
}

func (p *Protocol) buildGetOrderBookRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	limit := getIntParamWithDefault(params, "limit", 100)

	req := core.NewRequest(http.MethodGet, "/api/v3/depth")
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetQuery("limit", strconv.Itoa(limit))
	req.SetWeight(2 + limit/50)
	req.SetCache(fmt.Sprintf("orderbook:%s:%d", symbol, limit), 100*time.Millisecond)

	return req, nil
}

func (p *Protocol) buildGetTradesRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	limit := getIntParamWithDefault(params, "limit", 500)

	req := core.NewRequest(http.MethodGet, "/api/v3/trades")
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetQuery("limit", strconv.Itoa(limit))
	req.SetWeight(2)

	return req, nil
}

func (p *Protocol) buildGetKlinesRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	interval := getStringParamWithDefault(params, "interval", "1m")
	limit := getIntParamWithDefault(params, "limit", 500)

	req := core.NewRequest(http.MethodGet, "/api/v3/klines")
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetQuery("interval", interval)
	req.SetQuery("limit", strconv.Itoa(limit))
	req.SetWeight(2)

	return req, nil
}

func (p *Protocol) buildGetBalanceRequest(_ core.Params) (*core.Request, error) {
	req := core.NewRequest(http.MethodGet, "/api/v3/account")
	req.SetRequireAuth(true)
	req.SetWeight(10)

	return req, nil
}

func (p *Protocol) buildPlaceOrderRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	side, err := getRequiredStringParam(params, "side")
	if err != nil {
		return nil, err
	}

	orderType, err := getRequiredStringParam(params, "type")
	if err != nil {
		return nil, err
	}

	quantity, err := getRequiredStringParam(params, "quantity")
	if err != nil {
		return nil, err
	}

	req := core.NewRequest(http.MethodPost, "/api/v3/order")
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetQuery("side", strings.ToUpper(side))
	req.SetQuery("type", strings.ToUpper(orderType))
	req.SetQuery("quantity", quantity)
	req.SetRequireAuth(true)
	req.SetWeight(1)

	if price, ok := params["price"].(string); ok && price != "" {
		req.SetQuery("price", price)
	}

	if timeInForce, ok := params["time_in_force"].(string); ok && timeInForce != "" {
		req.SetQuery("timeInForce", strings.ToUpper(timeInForce))
	}

	if clientOrderID, ok := params["client_order_id"].(string); ok && clientOrderID != "" {
		req.SetQuery("newClientOrderId", clientOrderID)
	}

	return req, nil
}

func (p *Protocol) buildCancelOrderRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	req := core.NewRequest(http.MethodDelete, "/api/v3/order")
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetRequireAuth(true)
	req.SetWeight(1)

	if orderID, ok := params["order_id"].(string); ok && orderID != "" {
		req.SetQuery("orderId", orderID)
	}

	if clientOrderID, ok := params["client_order_id"].(string); ok && clientOrderID != "" {
		req.SetQuery("origClientOrderId", clientOrderID)
	}

	return req, nil
}

func (p *Protocol) buildGetOrderRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	req := core.NewRequest(http.MethodGet, "/api/v3/order")
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetRequireAuth(true)
	req.SetWeight(2)

	if orderID, ok := params["order_id"].(string); ok && orderID != "" {
		req.SetQuery("orderId", orderID)
	}

	if clientOrderID, ok := params["client_order_id"].(string); ok && clientOrderID != "" {
		req.SetQuery("origClientOrderId", clientOrderID)
	}

	return req, nil
}

func (p *Protocol) buildGetOpenOrdersRequest(params core.Params) (*core.Request, error) {
	req := core.NewRequest(http.MethodGet, "/api/v3/openOrders")
	req.SetRequireAuth(true)
	req.SetWeight(3)

	if symbol, ok := params["symbol"].(string); ok && symbol != "" {
		req.SetQuery("symbol", formatSymbol(symbol))
	}

	return req, nil
}

func (p *Protocol) buildGetOrderHistoryRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	req := core.NewRequest(http.MethodGet, "/api/v3/allOrders")
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetRequireAuth(true)
	req.SetWeight(10)

	if startTime, ok := params["start_time"].(int64); ok && startTime > 0 {
		req.SetQuery("startTime", strconv.FormatInt(startTime, 10))
	}

	if endTime, ok := params["end_time"].(int64); ok && endTime > 0 {
		req.SetQuery("endTime", strconv.FormatInt(endTime, 10))
	}

	if limit, ok := params["limit"].(int); ok && limit > 0 {
		req.SetQuery("limit", strconv.Itoa(limit))
	}

	return req, nil
}

func formatSymbol(symbol string) string {
	return strings.ReplaceAll(symbol, "/", "")
}

func parseSymbol(binanceSymbol string) string {
	quoteCurrencies := []string{"USDT", "BUSD", "USDC", "BTC", "ETH", "BNB"}

	for _, quote := range quoteCurrencies {
		if before, ok := strings.CutSuffix(binanceSymbol, quote); ok {
			base := before
			if base != "" {
				return base + "/" + quote
			}
		}
	}

	return binanceSymbol
}

func signHMAC(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func getRequiredStringParam(params core.Params, key string) (string, error) {
	val, ok := params[key]
	if !ok {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be a string", key)
	}

	if str == "" {
		return "", fmt.Errorf("parameter %s cannot be empty", key)
	}

	return str, nil
}

func getStringParamWithDefault(params core.Params, key, def string) string {
	if val, ok := params[key]; ok {
		if str, ok := val.(string); ok && str != "" {
			return str
		}
	}
	return def
}

func getIntParamWithDefault(params core.Params, key string, def int) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return def
}

type binanceAPIError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func mapBinanceErrorCode(code int) core.ErrorType {
	switch code {
	case -1015:
		return core.ErrorTypeRateLimit
	case -1022:
		return core.ErrorTypeAuthentication
	case -2010, -2011, -2013:
		return core.ErrorTypeInsufficientFunds
	case -1100, -1101, -1102, -1103, -1104, -1105:
		return core.ErrorTypeBadRequest
	default:
		if code >= -1000 && code < -1999 {
			return core.ErrorTypeBadRequest
		}
		if code >= -2000 && code < -2999 {
			return core.ErrorTypeInvalidOrder
		}
		return core.ErrorTypeUnknown
	}
}
