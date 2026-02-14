# Order Examples

This example demonstrates order management operations:

- Get Account Balance
- Place Limit Order (buy/sell)
- Query Order Status
- Cancel Order
- Get Open Orders

## Prerequisites

Set your Binance API credentials as environment variables:

```bash
export BINANCE_API_KEY="your-api-key"
export BINANCE_SECRET_KEY="your-secret-key"
```

**Note:** This example uses sandbox/testnet mode by default.

## Usage

```bash
go run main.go
```

## Expected Output

```
=== Get Account Balance ===
Non-zero balances:
  BTC: Free=0.5 Locked=0.1
  USDT: Free=50000.00 Locked=1000.00

=== Place Limit Order ===
ID:           12345678
Client ID:    my-order-001
Symbol:       BTC/USDT
Side:         BUY
Type:         LIMIT
Price:        40000.00
Quantity:     0.001
Status:       NEW
...

=== Query Order ===
ID:           12345678
Status:       NEW
...

=== Cancel Order ===
ID:           12345678
Status:       CANCELED
...

=== Get Open Orders ===
Found 2 open orders
  1. ID: 12345679 | BUY | 0.001 @ 39000.00 | Status: NEW
  2. ID: 12345680 | SELL | 0.002 @ 45000.00 | Status: NEW
```

## Order Builder

The OrderBuilder provides a fluent API for constructing orders:

```go
order, err := ordermanager.NewOrderBuilder("BTC/USDT").
    Buy().                    // or Sell()
    Limit().                  // or Market()
    Price("50000").           // required for limit orders
    Quantity("0.001").        // required for all orders
    GTC().                    // or IOC(), FOK()
    ClientOrderID("my-order"). // optional
    Build()
```

## Order Manager Callbacks

Register callbacks to receive order status updates:

```go
manager.OnOrderUpdate(func(o *core.Order) {
    fmt.Printf("Order %s: %s\n", o.ID, o.Status)
})
```
