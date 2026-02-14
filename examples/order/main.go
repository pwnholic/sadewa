package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cockroachdb/apd/v3"

	"sadewa/pkg/core"
	"sadewa/pkg/exchange"
	"sadewa/pkg/exchange/binance"
	"sadewa/pkg/order"
	"sadewa/pkg/session"
)

func main() {
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_SECRET_KEY")

	if apiKey == "" || secretKey == "" {
		fmt.Println("Please set BINANCE_API_KEY and BINANCE_SECRET_KEY environment variables")
		fmt.Println("\nUsage:")
		fmt.Println("  BINANCE_API_KEY=your_key BINANCE_SECRET_KEY=your_secret go run main.go")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := core.DefaultConfig("binance").
		WithCredentials(&core.Credentials{
			APIKey:    apiKey,
			SecretKey: secretKey,
		}).
		WithSandbox(true)

	container := exchange.NewContainer()
	if err := binance.Register(container, config); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register binance: %v\n", err)
		os.Exit(1)
	}

	sess, err := session.NewSession(container, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Close()

	if err := sess.SetExchange("binance"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set exchange: %v\n", err)
		os.Exit(1)
	}

	manager := ordermanager.NewManager(sess, ordermanager.ManagerConfig{
		EnableValidation: true,
	})

	manager.OnOrderUpdate(func(o *core.Order) {
		fmt.Printf("[CALLBACK] Order %s updated: %s\n", o.ID, o.Status.String())
	})

	fmt.Println("=== Get Account Balance ===")
	balances, err := sess.GetBalance(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting balance: %v\n", err)
	} else {
		printBalances(balances)
	}

	fmt.Println("\n=== Place Limit Order ===")
	order, err := placeLimitOrder(ctx, manager, "BTC/USDT", core.SideBuy, "40000", "0.001")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error placing order: %v\n", err)
	} else {
		printOrder(order)
	}

	if order != nil && order.ID != "" {
		fmt.Println("\n=== Query Order ===")
		queriedOrder, err := sess.GetOrder(ctx, &exchange.OrderQuery{
			Symbol:  order.Symbol,
			OrderID: order.ID,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error querying order: %v\n", err)
		} else {
			printOrder(queriedOrder)
		}

		fmt.Println("\n=== Cancel Order ===")
		canceledOrder, err := cancelOrder(ctx, manager, order.Symbol, order.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error canceling order: %v\n", err)
		} else {
			printOrder(canceledOrder)
		}
	}

	fmt.Println("\n=== Get Open Orders ===")
	openOrders, err := sess.GetOpenOrders(ctx, "BTC/USDT")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting open orders: %v\n", err)
	} else {
		fmt.Printf("Found %d open orders\n", len(openOrders))
		for i, o := range openOrders {
			fmt.Printf("  %d. ID: %s | %s | %s @ %s | Status: %s\n",
				i+1, o.ID, o.Side.String(), o.Quantity.String(),
				o.Price.String(), o.Status.String())
		}
	}
}

func placeLimitOrder(ctx context.Context, manager *ordermanager.Manager, symbol string, side core.OrderSide, price, quantity string) (*core.Order, error) {
	order, err := ordermanager.NewOrderBuilder(symbol).
		Side(side).
		Limit().
		Price(price).
		Quantity(quantity).
		GTC().
		Build()
	if err != nil {
		return nil, fmt.Errorf("build order: %w", err)
	}

	if err := manager.PlaceOrder(ctx, order); err != nil {
		return nil, fmt.Errorf("place order: %w", err)
	}

	return order, nil
}

func cancelOrder(ctx context.Context, manager *ordermanager.Manager, symbol, orderID string) (*core.Order, error) {
	if err := manager.CancelOrder(ctx, orderID); err != nil {
		return nil, err
	}

	order, _ := manager.GetOrder(orderID)
	return order, nil
}

func printBalances(balances []core.Balance) {
	fmt.Println("Non-zero balances:")
	for _, b := range balances {
		if !b.Free.IsZero() || !b.Locked.IsZero() {
			fmt.Printf("  %s: Free=%s Locked=%s\n",
				b.Asset, b.Free.String(), b.Locked.String())
		}
	}
}

func printOrder(o *core.Order) {
	fmt.Printf("ID:           %s\n", o.ID)
	fmt.Printf("Client ID:    %s\n", o.ClientOrderID)
	fmt.Printf("Symbol:       %s\n", o.Symbol)
	fmt.Printf("Side:         %s\n", o.Side.String())
	fmt.Printf("Type:         %s\n", o.Type.String())
	fmt.Printf("Price:        %s\n", o.Price.String())
	fmt.Printf("Quantity:     %s\n", o.Quantity.String())
	fmt.Printf("Filled:       %s\n", o.FilledQuantity.String())
	fmt.Printf("Remaining:    %s\n", o.RemainingQty.String())
	fmt.Printf("Status:       %s\n", o.Status.String())
	fmt.Printf("TimeInForce:  %s\n", o.TimeInForce.String())
	fmt.Printf("Created:      %s\n", o.CreatedAt.Format(time.RFC3339))
}

var _ = apd.Decimal{}
