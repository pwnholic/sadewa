package ordermanager

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sadewa/pkg/core"
)

type mockProtocol struct {
	buildRequestFunc  func(ctx context.Context, op core.Operation, params core.Params) (*core.Request, error)
	signRequestFunc   func(req any, creds core.Credentials) error
	parseResponseFunc func(resp any, result any) error
}

func (m *mockProtocol) BuildRequest(ctx context.Context, op core.Operation, params core.Params) (*core.Request, error) {
	if m.buildRequestFunc != nil {
		return m.buildRequestFunc(ctx, op, params)
	}
	return &core.Request{Method: "GET", Path: "/test"}, nil
}

func (m *mockProtocol) SignRequest(req any, creds core.Credentials) error {
	if m.signRequestFunc != nil {
		return m.signRequestFunc(req, creds)
	}
	return nil
}

func (m *mockProtocol) ParseResponse(resp any, result any) error {
	if m.parseResponseFunc != nil {
		return m.parseResponseFunc(resp, result)
	}
	return nil
}

func TestOrderFilter_Matches(t *testing.T) {
	order := &core.Order{
		Symbol: "BTC/USDT",
		Side:   core.SideSell,
		Status: core.StatusPartiallyFilled,
		Type:   core.TypeLimit,
	}

	tests := []struct {
		name   string
		filter OrderFilter
		want   bool
	}{
		{
			name:   "empty filter matches all",
			filter: OrderFilter{},
			want:   true,
		},
		{
			name:   "symbol match",
			filter: OrderFilter{Symbol: "BTC/USDT"},
			want:   true,
		},
		{
			name:   "symbol mismatch",
			filter: OrderFilter{Symbol: "ETH/USDT"},
			want:   false,
		},
		{
			name:   "side match",
			filter: OrderFilter{Side: core.SideSell},
			want:   true,
		},
		{
			name:   "side mismatch",
			filter: OrderFilter{Side: 99},
			want:   false,
		},
		{
			name:   "status match",
			filter: OrderFilter{Status: core.StatusPartiallyFilled},
			want:   true,
		},
		{
			name:   "status mismatch",
			filter: OrderFilter{Status: core.StatusFilled},
			want:   false,
		},
		{
			name:   "type match",
			filter: OrderFilter{Type: core.TypeLimit},
			want:   true,
		},
		{
			name:   "type mismatch",
			filter: OrderFilter{Type: core.TypeStopLoss},
			want:   false,
		},
		{
			name:   "multiple criteria match",
			filter: OrderFilter{Symbol: "BTC/USDT", Side: core.SideSell, Status: core.StatusPartiallyFilled},
			want:   true,
		},
		{
			name:   "multiple criteria one mismatch",
			filter: OrderFilter{Symbol: "BTC/USDT", Side: 99, Status: core.StatusPartiallyFilled},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.filter.Matches(order))
		})
	}
}

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		name string
		from core.OrderStatus
		to   core.OrderStatus
		want bool
	}{
		{"same status", core.StatusNew, core.StatusNew, true},
		{"NEW -> PARTIALLY_FILLED", core.StatusNew, core.StatusPartiallyFilled, true},
		{"NEW -> FILLED", core.StatusNew, core.StatusFilled, true},
		{"NEW -> CANCELING", core.StatusNew, core.StatusCanceling, true},
		{"NEW -> REJECTED", core.StatusNew, core.StatusRejected, true},
		{"NEW -> EXPIRED", core.StatusNew, core.StatusExpired, true},
		{"NEW -> CANCELED (invalid)", core.StatusNew, core.StatusCanceled, false},
		{"PARTIALLY_FILLED -> FILLED", core.StatusPartiallyFilled, core.StatusFilled, true},
		{"PARTIALLY_FILLED -> CANCELING", core.StatusPartiallyFilled, core.StatusCanceling, true},
		{"PARTIALLY_FILLED -> CANCELED", core.StatusPartiallyFilled, core.StatusCanceled, true},
		{"CANCELING -> CANCELED", core.StatusCanceling, core.StatusCanceled, true},
		{"CANCELING -> FILLED", core.StatusCanceling, core.StatusFilled, true},
		{"CANCELING -> PARTIALLY_FILLED", core.StatusCanceling, core.StatusPartiallyFilled, true},
		{"FILLED -> NEW (invalid)", core.StatusFilled, core.StatusNew, false},
		{"CANCELED -> FILLED (invalid)", core.StatusCanceled, core.StatusFilled, false},
		{"REJECTED -> NEW (invalid)", core.StatusRejected, core.StatusNew, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidTransition(tt.from, tt.to))
		})
	}
}

func TestManager_GetOrder(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})

	order := &core.Order{
		ID:     "order-123",
		Symbol: "BTC/USDT",
		Side:   core.SideBuy,
		Type:   core.TypeLimit,
		Status: core.StatusNew,
	}
	var qty apd.Decimal
	qty.SetString("0.1")
	order.Quantity = qty

	m.orders.Store(order.ID, order)

	t.Run("existing order", func(t *testing.T) {
		got, exists := m.GetOrder("order-123")
		assert.True(t, exists)
		assert.Equal(t, order, got)
	})

	t.Run("non-existing order", func(t *testing.T) {
		got, exists := m.GetOrder("non-existent")
		assert.False(t, exists)
		assert.Nil(t, got)
	})

	t.Run("empty order ID", func(t *testing.T) {
		got, exists := m.GetOrder("")
		assert.False(t, exists)
		assert.Nil(t, got)
	})
}

func TestManager_GetOrderByClientID(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})

	order := &core.Order{
		ID:            "order-123",
		ClientOrderID: "client-456",
		Symbol:        "BTC/USDT",
	}
	var qty apd.Decimal
	qty.SetString("0.1")
	order.Quantity = qty

	m.orders.Store(order.ID, order)
	m.clientOrderIDs.Store(order.ClientOrderID, order.ID)

	t.Run("existing client order ID", func(t *testing.T) {
		got, exists := m.GetOrderByClientID("client-456")
		assert.True(t, exists)
		assert.Equal(t, order, got)
	})

	t.Run("non-existing client order ID", func(t *testing.T) {
		got, exists := m.GetOrderByClientID("non-existent")
		assert.False(t, exists)
		assert.Nil(t, got)
	})

	t.Run("empty client order ID", func(t *testing.T) {
		got, exists := m.GetOrderByClientID("")
		assert.False(t, exists)
		assert.Nil(t, got)
	})
}

func TestManager_UpdateOrderStatus(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})

	order := &core.Order{
		ID:     "order-123",
		Symbol: "BTC/USDT",
		Status: core.StatusNew,
	}
	var qty apd.Decimal
	qty.SetString("0.1")
	order.Quantity = qty

	m.orders.Store(order.ID, order)

	t.Run("valid transition", func(t *testing.T) {
		err := m.UpdateOrderStatus("order-123", core.StatusFilled)
		assert.NoError(t, err)
		assert.Equal(t, core.StatusFilled, order.Status)
	})

	t.Run("invalid transition", func(t *testing.T) {
		order.Status = core.StatusFilled
		err := m.UpdateOrderStatus("order-123", core.StatusNew)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status transition")
	})

	t.Run("non-existing order", func(t *testing.T) {
		err := m.UpdateOrderStatus("non-existent", core.StatusFilled)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order not found")
	})
}

func TestManager_GetOrders(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})

	orders := []*core.Order{
		{ID: "1", Symbol: "BTC/USDT", Side: core.SideBuy, Status: core.StatusNew, Type: core.TypeLimit},
		{ID: "2", Symbol: "BTC/USDT", Side: core.SideSell, Status: core.StatusNew, Type: core.TypeLimit},
		{ID: "3", Symbol: "ETH/USDT", Side: core.SideBuy, Status: core.StatusFilled, Type: core.TypeMarket},
		{ID: "4", Symbol: "BTC/USDT", Side: core.SideBuy, Status: core.StatusFilled, Type: core.TypeLimit},
	}

	for _, o := range orders {
		var qty apd.Decimal
		qty.SetString("0.1")
		o.Quantity = qty
		m.orders.Store(o.ID, o)
	}

	tests := []struct {
		name   string
		filter OrderFilter
		want   int
	}{
		{"all orders", OrderFilter{}, 4},
		{"BTC/USDT only", OrderFilter{Symbol: "BTC/USDT"}, 3},
		{"sell side only", OrderFilter{Side: core.SideSell}, 1},
		{"filled status only", OrderFilter{Status: core.StatusFilled}, 2},
		{"limit type only", OrderFilter{Type: core.TypeLimit}, 3},
		{"BTC/USDT and sell", OrderFilter{Symbol: "BTC/USDT", Side: core.SideSell}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.GetOrders(tt.filter)
			assert.Len(t, result, tt.want)
		})
	}
}

func TestManager_GetOpenOrders(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})

	orders := []*core.Order{
		{ID: "1", Symbol: "BTC/USDT", Status: core.StatusNew},
		{ID: "2", Symbol: "ETH/USDT", Status: core.StatusPartiallyFilled},
		{ID: "3", Symbol: "BTC/USDT", Status: core.StatusFilled},
		{ID: "4", Symbol: "BTC/USDT", Status: core.StatusCanceling},
		{ID: "5", Symbol: "BTC/USDT", Status: core.StatusCanceled},
	}

	for _, o := range orders {
		var qty apd.Decimal
		qty.SetString("0.1")
		o.Quantity = qty
		m.orders.Store(o.ID, o)
	}

	result := m.GetOpenOrders()
	assert.Len(t, result, 3)
}

func TestManager_OnOrderUpdate(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})

	var mu sync.Mutex
	receivedOrders := make([]*core.Order, 0)

	m.OnOrderUpdate(func(order *core.Order) {
		mu.Lock()
		defer mu.Unlock()
		receivedOrders = append(receivedOrders, order)
	})

	order := &core.Order{
		ID:     "order-123",
		Symbol: "BTC/USDT",
		Status: core.StatusNew,
	}
	var qty apd.Decimal
	qty.SetString("0.1")
	order.Quantity = qty

	m.orders.Store(order.ID, order)

	err := m.UpdateOrderStatus("order-123", core.StatusFilled)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, receivedOrders, 1)
	assert.Equal(t, core.StatusFilled, receivedOrders[0].Status)
}

func TestManager_PlaceOrder_NilOrder(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})
	err := m.PlaceOrder(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "order is required")
}

func TestManager_CancelOrder_InvalidInputs(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})

	t.Run("empty order ID", func(t *testing.T) {
		err := m.CancelOrder(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order ID is required")
	})

	t.Run("non-existing order", func(t *testing.T) {
		err := m.CancelOrder(context.Background(), "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order not found")
	})
}

func TestManager_CancelOrder_TerminalState(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})

	order := &core.Order{
		ID:     "order-123",
		Symbol: "BTC/USDT",
		Status: core.StatusFilled,
	}
	var qty apd.Decimal
	qty.SetString("0.1")
	order.Quantity = qty

	m.orders.Store(order.ID, order)

	err := m.CancelOrder(context.Background(), "order-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot cancel order in terminal state")
}

func TestManager_SyncOrder_InvalidInputs(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})

	t.Run("empty order ID", func(t *testing.T) {
		_, err := m.SyncOrder(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order ID is required")
	})

	t.Run("non-existing order", func(t *testing.T) {
		_, err := m.SyncOrder(context.Background(), "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "order not found")
	})
}

func TestNewManager_DefaultConfig(t *testing.T) {
	m := NewManager(nil, ManagerConfig{MaxOrders: 0})
	assert.Equal(t, 10000, m.config.MaxOrders)
}

func TestManager_PlaceOrder_Validation(t *testing.T) {
	m := NewManager(nil, ManagerConfig{EnableValidation: true})

	order := &core.Order{
		Symbol: "",
	}

	err := m.PlaceOrder(context.Background(), order)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "order validation")
}

func TestManager_CallbacksConcurrent(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})

	var wg sync.WaitGroup
	callbackCount := 10
	wg.Add(callbackCount)

	for i := 0; i < callbackCount; i++ {
		go func() {
			defer wg.Done()
			m.OnOrderUpdate(func(order *core.Order) {})
		}()
	}

	wg.Wait()

	order := &core.Order{
		ID:     "order-123",
		Symbol: "BTC/USDT",
		Status: core.StatusNew,
	}
	var qty apd.Decimal
	qty.SetString("0.1")
	order.Quantity = qty

	m.orders.Store(order.ID, order)

	err := m.UpdateOrderStatus("order-123", core.StatusFilled)
	require.NoError(t, err)
}

func TestManager_UpdateOrderStatus_UpdatedAt(t *testing.T) {
	m := NewManager(nil, ManagerConfig{})

	order := &core.Order{
		ID:        "order-123",
		Symbol:    "BTC/USDT",
		Status:    core.StatusNew,
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}
	var qty apd.Decimal
	qty.SetString("0.1")
	order.Quantity = qty

	m.orders.Store(order.ID, order)

	beforeUpdate := order.UpdatedAt
	err := m.UpdateOrderStatus("order-123", core.StatusFilled)
	require.NoError(t, err)
	assert.True(t, order.UpdatedAt.After(beforeUpdate))
}
