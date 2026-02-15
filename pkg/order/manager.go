package ordermanager

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/rs/zerolog"

	"sadewa/pkg/core"
	"sadewa/pkg/exchange"
	"sadewa/pkg/session"
)

// OrderCallback is a function that receives order updates when orders change state.
// Deprecated: Use SubscribeOrders for channel-based updates instead.
type OrderCallback func(*core.Order)

// orderSubscriber holds a subscriber's channel for order updates.
type orderSubscriber struct {
	orderCh chan *core.Order
	errCh   chan error
}

// ManagerConfig holds configuration options for the order manager.
type ManagerConfig struct {
	// MaxOrders is the maximum number of orders to track. Defaults to 10000.
	MaxOrders int `json:"max_orders"`
	// EnableValidation controls whether orders are validated before submission.
	EnableValidation bool `json:"enable_validation"`
}

// Manager coordinates order lifecycle across a trading session.
// It tracks orders locally and synchronizes with the exchange.
type Manager struct {
	session        *session.Session
	config         ManagerConfig
	logger         zerolog.Logger
	orders         sync.Map
	clientOrderIDs sync.Map
	callbacks      []OrderCallback
	callbacksMu    sync.RWMutex
	subscribers    []orderSubscriber
	subscribersMu  sync.RWMutex
}

// NewManager creates a new order manager for the given session.
func NewManager(sess *session.Session, config ManagerConfig) *Manager {
	if config.MaxOrders <= 0 {
		config.MaxOrders = 10000
	}

	return &Manager{
		session:   sess,
		config:    config,
		logger:    zerolog.Nop(),
		orders:    sync.Map{},
		callbacks: make([]OrderCallback, 0),
	}
}

// PlaceOrder submits an order to the exchange and begins tracking it locally.
func (m *Manager) PlaceOrder(ctx context.Context, order *core.Order) error {
	if order == nil {
		return fmt.Errorf("order is required")
	}

	if m.config.EnableValidation {
		if err := validateOrder(order); err != nil {
			return fmt.Errorf("order validation: %w", err)
		}
	}

	now := time.Now()
	if order.CreatedAt.IsZero() {
		order.CreatedAt = now
	}
	order.UpdatedAt = now

	if order.Status == core.StatusNew {
		order.Status = core.StatusNew
	}

	order.RemainingQty.Set(&order.Quantity)
	order.FilledQuantity = apd.Decimal{}

	req := &exchange.OrderRequest{
		Symbol:        order.Symbol,
		Side:          order.Side,
		Type:          order.Type,
		Price:         order.Price,
		Quantity:      order.Quantity,
		TimeInForce:   order.TimeInForce,
		ClientOrderID: order.ClientOrderID,
	}

	placedOrder, err := m.session.PlaceOrder(ctx, req)
	if err != nil {
		return fmt.Errorf("place order: %w", err)
	}

	if placedOrder.ID != "" {
		order.ID = placedOrder.ID
	}
	if placedOrder.Status != core.StatusNew {
		order.Status = placedOrder.Status
	}
	if !placedOrder.FilledQuantity.IsZero() {
		order.FilledQuantity.Set(&placedOrder.FilledQuantity)
	}
	if !placedOrder.RemainingQty.IsZero() {
		order.RemainingQty.Set(&placedOrder.RemainingQty)
	}

	m.orders.Store(order.ID, order)
	if order.ClientOrderID != "" {
		m.clientOrderIDs.Store(order.ClientOrderID, order.ID)
	}

	m.notifyCallbacks(order)

	return nil
}

// CancelOrder requests cancellation of an order by ID.
// Returns an error if the order is in a terminal state.
func (m *Manager) CancelOrder(ctx context.Context, orderID string) error {
	if orderID == "" {
		return fmt.Errorf("order ID is required")
	}

	order, exists := m.GetOrder(orderID)
	if !exists {
		return fmt.Errorf("order not found: %s", orderID)
	}

	if order.Status.IsTerminal() {
		return fmt.Errorf("cannot cancel order in terminal state: %s", order.Status)
	}

	_, err := m.session.CancelOrder(ctx, &exchange.CancelRequest{
		Symbol:  order.Symbol,
		OrderID: orderID,
	})
	if err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}

	if err := m.UpdateOrderStatus(orderID, core.StatusCanceling); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return nil
}

// GetOrder retrieves a tracked order by its exchange-assigned ID.
func (m *Manager) GetOrder(orderID string) (*core.Order, bool) {
	if orderID == "" {
		return nil, false
	}

	value, ok := m.orders.Load(orderID)
	if !ok {
		return nil, false
	}

	order, ok := value.(*core.Order)
	if !ok {
		return nil, false
	}

	return order, true
}

// GetOrderByClientID retrieves a tracked order by its client-assigned ID.
func (m *Manager) GetOrderByClientID(clientOrderID string) (*core.Order, bool) {
	if clientOrderID == "" {
		return nil, false
	}

	value, ok := m.clientOrderIDs.Load(clientOrderID)
	if !ok {
		return nil, false
	}

	orderID, ok := value.(string)
	if !ok {
		return nil, false
	}

	return m.GetOrder(orderID)
}

// UpdateOrderStatus updates the status of a tracked order.
// Returns an error if the transition is invalid.
func (m *Manager) UpdateOrderStatus(orderID string, status core.OrderStatus) error {
	order, exists := m.GetOrder(orderID)
	if !exists {
		return fmt.Errorf("order not found: %s", orderID)
	}

	if !isValidTransition(order.Status, status) {
		return fmt.Errorf("invalid status transition: %s -> %s", order.Status, status)
	}

	order.Status = status
	order.UpdatedAt = time.Now()

	m.notifyCallbacks(order)

	return nil
}

// SyncOrder fetches the latest order state from the exchange and updates the local copy.
func (m *Manager) SyncOrder(ctx context.Context, orderID string) (*core.Order, error) {
	if orderID == "" {
		return nil, fmt.Errorf("order ID is required")
	}

	existingOrder, exists := m.GetOrder(orderID)
	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	updatedOrder, err := m.session.GetOrder(ctx, &exchange.OrderQuery{
		Symbol:  existingOrder.Symbol,
		OrderID: orderID,
	})
	if err != nil {
		return nil, fmt.Errorf("sync order: %w", err)
	}

	if !isValidTransition(existingOrder.Status, updatedOrder.Status) {
		return nil, fmt.Errorf("invalid status transition from exchange: %s -> %s", existingOrder.Status, updatedOrder.Status)
	}

	existingOrder.Status = updatedOrder.Status
	existingOrder.FilledQuantity.Set(&updatedOrder.FilledQuantity)
	existingOrder.RemainingQty.Set(&updatedOrder.RemainingQty)
	existingOrder.UpdatedAt = time.Now()

	if !updatedOrder.Price.IsZero() {
		existingOrder.Price.Set(&updatedOrder.Price)
	}

	m.notifyCallbacks(existingOrder)

	return existingOrder, nil
}

// GetOrders returns all tracked orders that match the given filter.
func (m *Manager) GetOrders(filter OrderFilter) []*core.Order {
	var result []*core.Order

	m.orders.Range(func(key, value any) bool {
		order, ok := value.(*core.Order)
		if !ok {
			return true
		}

		if filter.Matches(order) {
			result = append(result, order)
		}

		return true
	})

	return result
}

// GetOpenOrders returns all tracked orders that are not in a terminal state.
func (m *Manager) GetOpenOrders() []*core.Order {
	var result []*core.Order

	m.orders.Range(func(key, value any) bool {
		order, ok := value.(*core.Order)
		if !ok {
			return true
		}

		if !order.Status.IsTerminal() {
			result = append(result, order)
		}

		return true
	})

	return result
}

// CancelAllOrders cancels all non-terminal orders, optionally filtered by symbol.
func (m *Manager) CancelAllOrders(ctx context.Context, symbol string) error {
	var filter OrderFilter
	if symbol != "" {
		filter.Symbol = symbol
	}

	orders := m.GetOrders(filter)

	for _, order := range orders {
		if !order.Status.IsTerminal() && order.Status != core.StatusCanceling {
			if err := m.CancelOrder(ctx, order.ID); err != nil {
				m.logger.Warn().
					Err(err).
					Str("order_id", order.ID).
					Msg("failed to cancel order")
			}
		}
	}

	return nil
}

// OnOrderUpdate registers a callback to be invoked when any tracked order changes.
// Deprecated: Use SubscribeOrders for channel-based updates instead.
func (m *Manager) OnOrderUpdate(callback OrderCallback) {
	m.callbacksMu.Lock()
	defer m.callbacksMu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// SubscribeOrders returns channels for receiving order updates.
// The order channel receives order updates, and the error channel receives any errors.
// Channels are closed when the context is cancelled.
func (m *Manager) SubscribeOrders(ctx context.Context) (<-chan *core.Order, <-chan error) {
	orderCh := make(chan *core.Order, 100)
	errCh := make(chan error, 1)

	sub := orderSubscriber{
		orderCh: orderCh,
		errCh:   errCh,
	}

	m.subscribersMu.Lock()
	m.subscribers = append(m.subscribers, sub)
	m.subscribersMu.Unlock()

	// Spawn goroutine to handle context cancellation
	go func() {
		<-ctx.Done()
		m.removeSubscriber(sub)
	}()

	return orderCh, errCh
}

// removeSubscriber removes a subscriber and closes its channels.
func (m *Manager) removeSubscriber(sub orderSubscriber) {
	m.subscribersMu.Lock()
	defer m.subscribersMu.Unlock()

	for i, s := range m.subscribers {
		if s.orderCh == sub.orderCh {
			m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
			close(sub.orderCh)
			close(sub.errCh)
			return
		}
	}
}

func (m *Manager) notifyCallbacks(order *core.Order) {
	// Notify legacy callbacks
	m.callbacksMu.RLock()
	callbacks := make([]OrderCallback, len(m.callbacks))
	copy(callbacks, m.callbacks)
	m.callbacksMu.RUnlock()

	for _, callback := range callbacks {
		callback(order)
	}

	// Notify channel subscribers
	m.subscribersMu.RLock()
	subscribers := make([]orderSubscriber, len(m.subscribers))
	copy(subscribers, m.subscribers)
	m.subscribersMu.RUnlock()

	for _, sub := range subscribers {
		select {
		case sub.orderCh <- order:
		default:
			m.logger.Warn().Str("order_id", order.ID).Msg("order subscriber channel full, message dropped")
		}
	}
}

// OrderFilter defines criteria for filtering orders in queries.
type OrderFilter struct {
	// Symbol filters by trading pair symbol.
	Symbol string `json:"symbol,omitempty"`
	// Side filters by order side (buy/sell).
	Side core.OrderSide `json:"side,omitempty"`
	// Status filters by order status.
	Status core.OrderStatus `json:"status,omitempty"`
	// Type filters by order type.
	Type core.OrderType `json:"type,omitempty"`
}

// Matches returns true if the order satisfies all non-zero filter criteria.
func (f *OrderFilter) Matches(order *core.Order) bool {
	if f.Symbol != "" && order.Symbol != f.Symbol {
		return false
	}

	if f.Side != 0 && order.Side != f.Side {
		return false
	}

	if f.Status != 0 && order.Status != f.Status {
		return false
	}

	if f.Type != 0 && order.Type != f.Type {
		return false
	}

	return true
}

func isValidTransition(from, to core.OrderStatus) bool {
	if from == to {
		return true
	}

	validTransitions := map[core.OrderStatus][]core.OrderStatus{
		core.StatusNew: {
			core.StatusPartiallyFilled,
			core.StatusFilled,
			core.StatusCanceling,
			core.StatusRejected,
			core.StatusExpired,
		},
		core.StatusPartiallyFilled: {
			core.StatusFilled,
			core.StatusCanceling,
			core.StatusCanceled,
		},
		core.StatusCanceling: {
			core.StatusCanceled,
			core.StatusFilled,
			core.StatusPartiallyFilled,
		},
	}

	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}

	return slices.Contains(allowed, to)
}
