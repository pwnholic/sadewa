package ordermanager

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"

	"sadewa/pkg/core"
)

// OrderBuilder provides a fluent interface for constructing orders.
// It accumulates validation errors and reports them on Build.
//
// Example:
//
//	order, err := ordermanager.NewOrderBuilder("BTC/USDT").
//	    Buy().
//	    Limit().
//	    Price("50000").
//	    Quantity("0.001").
//	    Build()
type OrderBuilder struct {
	order *core.Order
	err   error
}

// NewOrderBuilder creates a new order builder for the given trading symbol.
func NewOrderBuilder(symbol string) *OrderBuilder {
	return &OrderBuilder{
		order: &core.Order{
			Symbol: symbol,
			Status: core.StatusNew,
		},
	}
}

// Side sets the order side (buy or sell).
func (b *OrderBuilder) Side(side core.OrderSide) *OrderBuilder {
	if b.err != nil {
		return b
	}
	b.order.Side = side
	return b
}

// Buy sets the order side to buy.
func (b *OrderBuilder) Buy() *OrderBuilder {
	return b.Side(core.SideBuy)
}

// Sell sets the order side to sell.
func (b *OrderBuilder) Sell() *OrderBuilder {
	return b.Side(core.SideSell)
}

// Type sets the order type (market, limit, etc.).
func (b *OrderBuilder) Type(orderType core.OrderType) *OrderBuilder {
	if b.err != nil {
		return b
	}
	b.order.Type = orderType
	return b
}

// Market sets the order type to market.
func (b *OrderBuilder) Market() *OrderBuilder {
	return b.Type(core.TypeMarket)
}

// Limit sets the order type to limit.
func (b *OrderBuilder) Limit() *OrderBuilder {
	return b.Type(core.TypeLimit)
}

// Price sets the order price from a string representation.
func (b *OrderBuilder) Price(price string) *OrderBuilder {
	if b.err != nil {
		return b
	}
	_, _, err := b.order.Price.SetString(price)
	if err != nil {
		b.err = fmt.Errorf("parse price: %w", err)
	}
	return b
}

// PriceDecimal sets the order price from an apd.Decimal value.
func (b *OrderBuilder) PriceDecimal(price apd.Decimal) *OrderBuilder {
	if b.err != nil {
		return b
	}
	b.order.Price.Set(&price)
	return b
}

// Quantity sets the order quantity from a string representation.
func (b *OrderBuilder) Quantity(qty string) *OrderBuilder {
	if b.err != nil {
		return b
	}
	_, _, err := b.order.Quantity.SetString(qty)
	if err != nil {
		b.err = fmt.Errorf("parse quantity: %w", err)
	}
	return b
}

// QuantityDecimal sets the order quantity from an apd.Decimal value.
func (b *OrderBuilder) QuantityDecimal(qty apd.Decimal) *OrderBuilder {
	if b.err != nil {
		return b
	}
	b.order.Quantity.Set(&qty)
	return b
}

// TimeInForce sets the time-in-force policy for the order.
func (b *OrderBuilder) TimeInForce(tif core.TimeInForce) *OrderBuilder {
	if b.err != nil {
		return b
	}
	b.order.TimeInForce = tif
	return b
}

// GTC sets the time-in-force to Good-Till-Cancelled.
func (b *OrderBuilder) GTC() *OrderBuilder {
	return b.TimeInForce(core.GTC)
}

// IOC sets the time-in-force to Immediate-Or-Cancel.
func (b *OrderBuilder) IOC() *OrderBuilder {
	return b.TimeInForce(core.IOC)
}

// FOK sets the time-in-force to Fill-Or-Kill.
func (b *OrderBuilder) FOK() *OrderBuilder {
	return b.TimeInForce(core.FOK)
}

// ClientOrderID sets a client-assigned identifier for order tracking.
func (b *OrderBuilder) ClientOrderID(id string) *OrderBuilder {
	if b.err != nil {
		return b
	}
	b.order.ClientOrderID = id
	return b
}

// Build validates and returns the constructed order.
// Returns an error if any required fields are missing or invalid.
func (b *OrderBuilder) Build() (*core.Order, error) {
	if b.err != nil {
		return nil, b.err
	}

	if err := validateOrder(b.order); err != nil {
		return nil, err
	}

	return b.order, nil
}

func validateOrder(order *core.Order) error {
	if order.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}

	if order.Quantity.IsZero() || order.Quantity.Negative {
		return fmt.Errorf("quantity must be positive")
	}

	if order.Type == core.TypeLimit || order.Type == core.TypeStopLossLimit || order.Type == core.TypeTakeProfitLimit {
		if order.Price.IsZero() || order.Price.Negative {
			return fmt.Errorf("price must be positive for limit orders")
		}
	}

	if order.Side != core.SideBuy && order.Side != core.SideSell {
		return fmt.Errorf("invalid order side")
	}

	if order.Type < core.TypeMarket || order.Type > core.TypeTakeProfitLimit {
		return fmt.Errorf("invalid order type")
	}

	return nil
}
