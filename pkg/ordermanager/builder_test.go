package ordermanager

import (
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/stretchr/testify/assert"

	"sadewa/pkg/core"
)

func TestOrderBuilder_Build(t *testing.T) {
	tests := []struct {
		name       string
		build      func() (*core.Order, error)
		wantErr    bool
		errContain string
	}{
		{
			name: "valid limit buy order",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Limit().
					Price("50000.00").
					Quantity("0.1").
					GTC().
					Build()
			},
			wantErr: false,
		},
		{
			name: "valid market sell order",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("ETH/USDT").
					Sell().
					Market().
					Quantity("1.5").
					Build()
			},
			wantErr: false,
		},
		{
			name: "valid order with client ID",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Limit().
					Price("50000").
					Quantity("0.1").
					ClientOrderID("client-123").
					Build()
			},
			wantErr: false,
		},
		{
			name: "valid order with decimal price",
			build: func() (*core.Order, error) {
				var price apd.Decimal
				price.SetString("50000.50")
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Limit().
					PriceDecimal(price).
					Quantity("0.1").
					Build()
			},
			wantErr: false,
		},
		{
			name: "valid order with decimal quantity",
			build: func() (*core.Order, error) {
				var qty apd.Decimal
				qty.SetString("0.12345678")
				return NewOrderBuilder("BTC/USDT").
					Sell().
					Limit().
					Price("50000").
					QuantityDecimal(qty).
					Build()
			},
			wantErr: false,
		},
		{
			name: "missing symbol",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("").
					Buy().
					Limit().
					Price("50000").
					Quantity("0.1").
					Build()
			},
			wantErr:    true,
			errContain: "symbol is required",
		},
		{
			name: "missing quantity",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Limit().
					Price("50000").
					Build()
			},
			wantErr:    true,
			errContain: "quantity must be positive",
		},
		{
			name: "zero quantity",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Limit().
					Price("50000").
					Quantity("0").
					Build()
			},
			wantErr:    true,
			errContain: "quantity must be positive",
		},
		{
			name: "negative quantity",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Limit().
					Price("50000").
					Quantity("-0.1").
					Build()
			},
			wantErr:    true,
			errContain: "quantity must be positive",
		},
		{
			name: "limit order without price",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Limit().
					Quantity("0.1").
					Build()
			},
			wantErr:    true,
			errContain: "price must be positive for limit orders",
		},
		{
			name: "limit order with zero price",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Limit().
					Price("0").
					Quantity("0.1").
					Build()
			},
			wantErr:    true,
			errContain: "price must be positive for limit orders",
		},
		{
			name: "market order does not require price",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Market().
					Quantity("0.1").
					Build()
			},
			wantErr: false,
		},
		{
			name: "invalid price string",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Limit().
					Price("invalid").
					Quantity("0.1").
					Build()
			},
			wantErr:    true,
			errContain: "parse price",
		},
		{
			name: "invalid quantity string",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Market().
					Quantity("invalid").
					Build()
			},
			wantErr:    true,
			errContain: "parse quantity",
		},
		{
			name: "IOC time in force",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Limit().
					Price("50000").
					Quantity("0.1").
					IOC().
					Build()
			},
			wantErr: false,
		},
		{
			name: "FOK time in force",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Buy().
					Limit().
					Price("50000").
					Quantity("0.1").
					FOK().
					Build()
			},
			wantErr: false,
		},
		{
			name: "stop loss order",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Sell().
					Type(core.TypeStopLoss).
					Quantity("0.1").
					Build()
			},
			wantErr: false,
		},
		{
			name: "stop loss limit order requires price",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Sell().
					Type(core.TypeStopLossLimit).
					Quantity("0.1").
					Build()
			},
			wantErr:    true,
			errContain: "price must be positive for limit orders",
		},
		{
			name: "take profit limit order with price",
			build: func() (*core.Order, error) {
				return NewOrderBuilder("BTC/USDT").
					Sell().
					Type(core.TypeTakeProfitLimit).
					Price("60000").
					Quantity("0.1").
					Build()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := tt.build()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
				assert.Nil(t, order)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, order)
			}
		})
	}
}

func TestOrderBuilder_FluentAPI(t *testing.T) {
	order, err := NewOrderBuilder("BTC/USDT").
		Side(core.SideBuy).
		Type(core.TypeLimit).
		Price("50000.00").
		Quantity("0.1").
		TimeInForce(core.GTC).
		ClientOrderID("test-123").
		Build()

	assert.NoError(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, "BTC/USDT", order.Symbol)
	assert.Equal(t, core.SideBuy, order.Side)
	assert.Equal(t, core.TypeLimit, order.Type)
	assert.Equal(t, core.GTC, order.TimeInForce)
	assert.Equal(t, "test-123", order.ClientOrderID)
}

func TestOrderBuilder_ConvenienceMethods(t *testing.T) {
	t.Run("Buy sets side to buy", func(t *testing.T) {
		order, err := NewOrderBuilder("BTC/USDT").Buy().Market().Quantity("0.1").Build()
		assert.NoError(t, err)
		assert.Equal(t, core.SideBuy, order.Side)
	})

	t.Run("Sell sets side to sell", func(t *testing.T) {
		order, err := NewOrderBuilder("BTC/USDT").Sell().Market().Quantity("0.1").Build()
		assert.NoError(t, err)
		assert.Equal(t, core.SideSell, order.Side)
	})

	t.Run("Market sets type to market", func(t *testing.T) {
		order, err := NewOrderBuilder("BTC/USDT").Buy().Market().Quantity("0.1").Build()
		assert.NoError(t, err)
		assert.Equal(t, core.TypeMarket, order.Type)
	})

	t.Run("Limit sets type to limit", func(t *testing.T) {
		order, err := NewOrderBuilder("BTC/USDT").Buy().Limit().Price("50000").Quantity("0.1").Build()
		assert.NoError(t, err)
		assert.Equal(t, core.TypeLimit, order.Type)
	})

	t.Run("GTC sets time in force", func(t *testing.T) {
		order, err := NewOrderBuilder("BTC/USDT").Buy().Limit().Price("50000").Quantity("0.1").GTC().Build()
		assert.NoError(t, err)
		assert.Equal(t, core.GTC, order.TimeInForce)
	})

	t.Run("IOC sets time in force", func(t *testing.T) {
		order, err := NewOrderBuilder("BTC/USDT").Buy().Limit().Price("50000").Quantity("0.1").IOC().Build()
		assert.NoError(t, err)
		assert.Equal(t, core.IOC, order.TimeInForce)
	})

	t.Run("FOK sets time in force", func(t *testing.T) {
		order, err := NewOrderBuilder("BTC/USDT").Buy().Limit().Price("50000").Quantity("0.1").FOK().Build()
		assert.NoError(t, err)
		assert.Equal(t, core.FOK, order.TimeInForce)
	})
}

func TestOrderBuilder_ChainOnError(t *testing.T) {
	t.Run("error preserves through chain", func(t *testing.T) {
		builder := NewOrderBuilder("BTC/USDT").
			Buy().
			Price("invalid").
			Limit().
			Quantity("0.1")

		order, err := builder.Build()
		assert.Error(t, err)
		assert.Nil(t, order)
		assert.Contains(t, err.Error(), "parse price")
	})
}
