package bybit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"resty.dev/v3"

	"sadewa/pkg/core"
)

const (
	ProductionURL = "https://api.bybit.com"
	SandboxURL    = "https://api-testnet.bybit.com"
)

// Protocol implements the core.Protocol interface for Bybit exchange.
// It provides request building, response parsing, and authentication for the Bybit API.
type Protocol struct{}

// NewProtocol creates a new Bybit protocol instance.
func NewProtocol() *Protocol {
	return &Protocol{}
}

// Name returns the protocol identifier "bybit".
func (p *Protocol) Name() string {
	return "bybit"
}

// Version returns the Bybit API version string.
func (p *Protocol) Version() string {
	return "5"
}

// BaseURL returns the base URL for the Bybit API.
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

// RateLimits returns the rate limit configuration for Bybit API.
func (p *Protocol) RateLimits() core.RateLimitConfig {
	return core.RateLimitConfig{
		RequestsPerSecond: 20,
		OrdersPerSecond:   10,
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
// It handles Bybit-specific error responses and maps them to appropriate error types.
func (p *Protocol) ParseResponse(op core.Operation, resp *resty.Response) (any, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil response")
	}

	if resp.StatusCode() >= 400 {
		var bybitErr bybitAPIError
		if err := sonic.Unmarshal(resp.Bytes(), &bybitErr); err == nil && bybitErr.RetCode != 0 {
			return nil, core.NewExchangeErrorWithCode(
				p.Name(),
				mapBybitErrorCode(bybitErr.RetCode),
				resp.StatusCode(),
				strconv.Itoa(bybitErr.RetCode),
				bybitErr.RetMsg,
			)
		}
		return nil, core.NewExchangeError(
			p.Name(),
			core.ErrorTypeServerError,
			resp.StatusCode(),
			fmt.Sprintf("HTTP error: %s", resp.Status()),
		)
	}

	// Bybit wraps responses in a result object
	var baseResponse struct {
		Result any `json:"result"`
	}
	if err := sonic.Unmarshal(resp.Bytes(), &baseResponse); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	n := NewNormalizer()
	resultBytes, _ := sonic.Marshal(baseResponse.Result)

	switch op {
	case core.OpGetTicker:
		var data struct {
			List []bybitTicker `json:"list"`
		}
		if err := sonic.Unmarshal(resultBytes, &data); err != nil {
			return nil, fmt.Errorf("unmarshal ticker: %w", err)
		}
		if len(data.List) == 0 {
			return nil, fmt.Errorf("no ticker data")
		}
		return n.NormalizeTicker(&data.List[0]), nil

	case core.OpGetOrderBook:
		var data bybitOrderBook
		if err := sonic.Unmarshal(resultBytes, &data); err != nil {
			return nil, fmt.Errorf("unmarshal order book: %w", err)
		}
		return n.NormalizeOrderBook(&data, "")

	case core.OpGetTrades:
		var data struct {
			List []bybitTrade `json:"list"`
		}
		if err := sonic.Unmarshal(resultBytes, &data); err != nil {
			return nil, fmt.Errorf("unmarshal trades: %w", err)
		}
		return n.NormalizeTrades(data.List, ""), nil

	case core.OpGetKlines:
		var data struct {
			List []bybitKline `json:"list"`
		}
		if err := sonic.Unmarshal(resultBytes, &data); err != nil {
			return nil, fmt.Errorf("unmarshal klines: %w", err)
		}
		return n.NormalizeKlines(data.List, "")

	case core.OpGetBalance:
		var data bybitAccount
		if err := sonic.Unmarshal(resultBytes, &data); err != nil {
			return nil, fmt.Errorf("unmarshal account: %w", err)
		}
		return n.NormalizeBalances(&data), nil

	case core.OpPlaceOrder, core.OpGetOrder, core.OpCancelOrder:
		var data bybitOrder
		if err := sonic.Unmarshal(resultBytes, &data); err != nil {
			return nil, fmt.Errorf("unmarshal order: %w", err)
		}
		return n.NormalizeOrder(&data)

	case core.OpGetOpenOrders, core.OpGetOrderHistory:
		var data struct {
			List []bybitOrder `json:"list"`
		}
		if err := sonic.Unmarshal(resultBytes, &data); err != nil {
			return nil, fmt.Errorf("unmarshal orders: %w", err)
		}
		return n.NormalizeOrders(data.List)

	default:
		var result any
		if err := sonic.Unmarshal(resultBytes, &result); err != nil {
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

	// Build signature string: timestamp + API key + recvWindow + queryString
	recvWindow := "5000"

	// For Bybit V5, we need to sign the query string
	var queryString strings.Builder
	queryString.WriteString("api_key=")
	queryString.WriteString(creds.APIKey)
	queryString.WriteString("&recvWindow=")
	queryString.WriteString(recvWindow)
	queryString.WriteString("&timestamp=")
	queryString.WriteString(ts)

	// Add any existing query params
	for k, vals := range req.QueryParams {
		if len(vals) > 0 {
			queryString.WriteString("&")
			queryString.WriteString(k)
			queryString.WriteString("=")
			queryString.WriteString(vals[0])
		}
	}

	signature := signHMAC(queryString.String(), creds.SecretKey)

	req.SetQueryParamsFromValues(map[string][]string{
		"api_key":    {creds.APIKey},
		"timestamp":  {ts},
		"recvWindow": {recvWindow},
		"sign":       {signature},
	})

	return nil
}

func (p *Protocol) buildGetTickerRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	category := getStringParamWithDefault(params, "category", "spot")

	req := core.NewRequest(http.MethodGet, "/v5/market/tickers")
	req.SetQuery("category", category)
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

	category := getStringParamWithDefault(params, "category", "spot")
	limit := getIntParamWithDefault(params, "limit", 25)

	req := core.NewRequest(http.MethodGet, "/v5/market/orderbook")
	req.SetQuery("category", category)
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetQuery("limit", strconv.Itoa(limit))
	req.SetWeight(2)
	req.SetCache(fmt.Sprintf("orderbook:%s:%d", symbol, limit), 100*time.Millisecond)

	return req, nil
}

func (p *Protocol) buildGetTradesRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	category := getStringParamWithDefault(params, "category", "spot")
	limit := getIntParamWithDefault(params, "limit", 60)

	req := core.NewRequest(http.MethodGet, "/v5/market/recent-trade")
	req.SetQuery("category", category)
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

	category := getStringParamWithDefault(params, "category", "spot")
	interval := getStringParamWithDefault(params, "interval", "1")
	limit := getIntParamWithDefault(params, "limit", 200)

	req := core.NewRequest(http.MethodGet, "/v5/market/kline")
	req.SetQuery("category", category)
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetQuery("interval", interval)
	req.SetQuery("limit", strconv.Itoa(limit))
	req.SetWeight(2)

	return req, nil
}

func (p *Protocol) buildGetBalanceRequest(params core.Params) (*core.Request, error) {
	accountType := getStringParamWithDefault(params, "accountType", "UNIFIED")

	req := core.NewRequest(http.MethodGet, "/v5/account/wallet-balance")
	req.SetQuery("accountType", accountType)
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

	category := getStringParamWithDefault(params, "category", "spot")

	req := core.NewRequest(http.MethodPost, "/v5/order/create")
	req.SetQuery("category", category)
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetQuery("side", capitalize(side))
	req.SetQuery("orderType", capitalize(orderType))
	req.SetQuery("qty", quantity)
	req.SetRequireAuth(true)
	req.SetWeight(1)

	if price, ok := params["price"].(string); ok && price != "" {
		req.SetQuery("price", price)
	}

	if timeInForce, ok := params["time_in_force"].(string); ok && timeInForce != "" {
		req.SetQuery("timeInForce", strings.ToUpper(timeInForce))
	}

	if clientOrderID, ok := params["client_order_id"].(string); ok && clientOrderID != "" {
		req.SetQuery("orderLinkId", clientOrderID)
	}

	return req, nil
}

func (p *Protocol) buildCancelOrderRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	category := getStringParamWithDefault(params, "category", "spot")

	req := core.NewRequest(http.MethodPost, "/v5/order/cancel")
	req.SetQuery("category", category)
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetRequireAuth(true)
	req.SetWeight(1)

	if orderID, ok := params["order_id"].(string); ok && orderID != "" {
		req.SetQuery("orderId", orderID)
	}

	if clientOrderID, ok := params["client_order_id"].(string); ok && clientOrderID != "" {
		req.SetQuery("orderLinkId", clientOrderID)
	}

	return req, nil
}

func (p *Protocol) buildGetOrderRequest(params core.Params) (*core.Request, error) {
	symbol, err := getRequiredStringParam(params, "symbol")
	if err != nil {
		return nil, err
	}

	category := getStringParamWithDefault(params, "category", "spot")

	req := core.NewRequest(http.MethodGet, "/v5/order/realtime")
	req.SetQuery("category", category)
	req.SetQuery("symbol", formatSymbol(symbol))
	req.SetRequireAuth(true)
	req.SetWeight(2)

	if orderID, ok := params["order_id"].(string); ok && orderID != "" {
		req.SetQuery("orderId", orderID)
	}

	if clientOrderID, ok := params["client_order_id"].(string); ok && clientOrderID != "" {
		req.SetQuery("orderLinkId", clientOrderID)
	}

	return req, nil
}

func (p *Protocol) buildGetOpenOrdersRequest(params core.Params) (*core.Request, error) {
	category := getStringParamWithDefault(params, "category", "spot")

	req := core.NewRequest(http.MethodGet, "/v5/order/realtime")
	req.SetQuery("category", category)
	req.SetRequireAuth(true)
	req.SetWeight(3)

	if symbol, ok := params["symbol"].(string); ok && symbol != "" {
		req.SetQuery("symbol", formatSymbol(symbol))
	}

	return req, nil
}

func (p *Protocol) buildGetOrderHistoryRequest(params core.Params) (*core.Request, error) {
	category := getStringParamWithDefault(params, "category", "spot")

	req := core.NewRequest(http.MethodGet, "/v5/order/history")
	req.SetQuery("category", category)
	req.SetRequireAuth(true)
	req.SetWeight(10)

	if symbol, ok := params["symbol"].(string); ok && symbol != "" {
		req.SetQuery("symbol", formatSymbol(symbol))
	}

	if limit, ok := params["limit"].(int); ok && limit > 0 {
		req.SetQuery("limit", strconv.Itoa(limit))
	}

	return req, nil
}

func formatSymbol(symbol string) string {
	return strings.ReplaceAll(symbol, "/", "")
}

func parseSymbol(bybitSymbol string) string {
	quoteCurrencies := []string{"USDT", "USDC", "BTC", "ETH"}

	for _, quote := range quoteCurrencies {
		if before, ok := strings.CutSuffix(bybitSymbol, quote); ok {
			base := before
			if base != "" {
				return base + "/" + quote
			}
		}
	}

	return bybitSymbol
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
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

type bybitAPIError struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
}

func mapBybitErrorCode(code int) core.ErrorType {
	switch code {
	case 10001, 10002, 10003:
		return core.ErrorTypeBadRequest
	case 10004, 10005:
		return core.ErrorTypeAuthentication
	case 10006, 10010, 10017, 10018:
		return core.ErrorTypeRateLimit
	case 110007, 110012, 110013, 110043:
		return core.ErrorTypeInsufficientFunds
	case 110001, 110002, 110003, 110004, 110005:
		return core.ErrorTypeInvalidOrder
	default:
		if code >= 10000 && code < 11000 {
			return core.ErrorTypeBadRequest
		}
		if code >= 11000 && code < 12000 {
			return core.ErrorTypeInvalidOrder
		}
		return core.ErrorTypeUnknown
	}
}
