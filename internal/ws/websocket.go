package ws

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/lxzan/gws"
	"github.com/rs/zerolog"
)

// WSConfig holds configuration options for a websocket client.
type WSConfig struct {
	// URL is the websocket server endpoint to connect to.
	URL string
	// ReconnectEnabled determines whether automatic reconnection is enabled.
	ReconnectEnabled bool
	// ReconnectMaxWait is the maximum duration to wait between reconnection attempts.
	ReconnectMaxWait time.Duration
	// ReconnectBaseWait is the initial duration to wait before the first reconnection attempt.
	ReconnectBaseWait time.Duration
	// PingInterval is the duration between ping messages sent to keep the connection alive.
	PingInterval time.Duration
	// PongWait is the maximum time to wait for a pong response before considering the connection dead.
	PongWait time.Duration
	// BufferSize is the capacity of channel buffers for subscription messages.
	BufferSize int
}

// WSClient manages a websocket connection with reconnection and subscription support.
type WSClient struct {
	config  WSConfig
	state   *State
	conn    *gws.Conn
	handler *wsEventHandler
	logger  zerolog.Logger

	mu                sync.RWMutex
	subs              map[string]*wsSubscription
	connectedChan     chan struct{}
	stopChan          chan struct{}
	wg                sync.WaitGroup
	reconnectAttempts int
}

type wsSubscription struct {
	channel string
	dataCh  chan []byte
	errCh   chan error
	closeCh chan struct{}
}

type wsEventHandler struct {
	client *WSClient
}

// NewWSClient creates a new websocket client with the given configuration.
// Default values are applied for any zero-valued configuration fields.
func NewWSClient(config WSConfig) *WSClient {
	if config.ReconnectBaseWait == 0 {
		config.ReconnectBaseWait = 1 * time.Second
	}
	if config.ReconnectMaxWait == 0 {
		config.ReconnectMaxWait = 30 * time.Second
	}
	if config.PingInterval == 0 {
		config.PingInterval = 10 * time.Second
	}
	if config.PongWait == 0 {
		config.PongWait = 20 * time.Second
	}
	if config.BufferSize == 0 {
		config.BufferSize = 100
	}

	client := &WSClient{
		config:        config,
		state:         &State{},
		subs:          make(map[string]*wsSubscription),
		connectedChan: make(chan struct{}),
		stopChan:      make(chan struct{}),
		logger:        zerolog.Nop(),
	}
	client.state.Store(StateDisconnected)
	client.handler = &wsEventHandler{client: client}
	return client
}

// SetLogger configures the logger for the websocket client.
func (c *WSClient) SetLogger(logger zerolog.Logger) {
	c.logger = logger
}

func (h *wsEventHandler) OnOpen(socket *gws.Conn) {
	h.client.state.Store(StateConnected)

	h.client.mu.Lock()
	h.client.reconnectAttempts = 0
	select {
	case <-h.client.connectedChan:
	default:
		close(h.client.connectedChan)
	}
	h.client.mu.Unlock()

	h.client.logger.Info().
		Str("url", h.client.config.URL).
		Msg("websocket connected")

	_ = socket.SetDeadline(time.Now().Add(h.client.config.PingInterval + h.client.config.PongWait))
}

func (h *wsEventHandler) OnClose(socket *gws.Conn, err error) {
	h.client.state.Store(StateDisconnected)

	h.client.mu.Lock()
	h.client.connectedChan = make(chan struct{})
	h.client.mu.Unlock()

	h.client.logger.Warn().
		Err(err).
		Str("url", h.client.config.URL).
		Msg("websocket disconnected")

	if h.client.config.ReconnectEnabled {
		select {
		case <-h.client.stopChan:
			return
		default:
			go h.client.attemptReconnect()
		}
	}
}

func (h *wsEventHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(h.client.config.PingInterval + h.client.config.PongWait))
	_ = socket.WritePong(nil)
}

func (h *wsEventHandler) OnPong(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(h.client.config.PingInterval + h.client.config.PongWait))
}

func (h *wsEventHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	data := message.Bytes()
	if len(data) == 0 {
		return
	}

	h.client.logger.Debug().Str("data", string(data)).Msg("received websocket message")

	h.client.mu.RLock()
	subs := make([]*wsSubscription, 0, len(h.client.subs))
	for _, sub := range h.client.subs {
		subs = append(subs, sub)
	}
	h.client.mu.RUnlock()

	for _, sub := range subs {
		select {
		case <-sub.closeCh:
			continue
		case sub.dataCh <- data:
		default:
			h.client.logger.Warn().Str("channel", sub.channel).Msg("channel buffer full, dropping message")
		}
	}
}

// Connect establishes a websocket connection to the configured URL.
// It returns an error if the connection fails or the client is in an invalid state.
func (c *WSClient) Connect(ctx context.Context) error {
	if !c.state.CompareAndSwap(StateDisconnected, StateConnecting) {
		current := c.state.Load()
		if current == StateConnected {
			return nil
		}
		return fmt.Errorf("invalid state for connect: %s", current)
	}

	socket, _, err := gws.NewClient(c.handler, &gws.ClientOption{
		Addr: c.config.URL,
	})
	if err != nil {
		c.state.Store(StateDisconnected)
		return fmt.Errorf("connect websocket: %w", err)
	}

	c.mu.Lock()
	c.conn = socket
	c.mu.Unlock()

	c.wg.Go(func() {
		socket.ReadLoop()
	})

	select {
	case <-c.connectedChan:
		return nil
	case <-ctx.Done():
		_ = socket.NetConn().Close()
		c.state.Store(StateDisconnected)
		return ctx.Err()
	case <-c.stopChan:
		_ = socket.NetConn().Close()
		c.state.Store(StateClosed)
		return fmt.Errorf("client stopped")
	}
}

// Close gracefully shuts down the websocket client and releases all resources.
func (c *WSClient) Close() error {
	if !c.state.CompareAndSwap(StateConnected, StateClosed) &&
		!c.state.CompareAndSwap(StateConnecting, StateClosed) &&
		!c.state.CompareAndSwap(StateReconnecting, StateClosed) &&
		!c.state.CompareAndSwap(StateDisconnected, StateClosed) {
		return nil
	}

	close(c.stopChan)

	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.NetConn().Close()
	}
	for _, sub := range c.subs {
		close(sub.closeCh)
		close(sub.dataCh)
		close(sub.errCh)
	}
	c.subs = make(map[string]*wsSubscription)
	c.mu.Unlock()

	c.wg.Wait()
	return nil
}

// State returns the current connection state of the websocket.
func (c *WSClient) State() ConnState {
	return c.state.Load()
}

// IsConnected returns true if the websocket has an active connection.
func (c *WSClient) IsConnected() bool {
	return c.state.Load() == StateConnected
}

// SubscribeChannel registers a subscription for the given channel and returns
// separate channels for receiving data and errors. Messages are delivered
// to the data channel, while subscription errors are sent to the error channel.
func (c *WSClient) SubscribeChannel(channel string) (<-chan []byte, <-chan error) {
	dataCh := make(chan []byte, c.config.BufferSize)
	errCh := make(chan error, 1)
	closeCh := make(chan struct{})

	sub := &wsSubscription{
		channel: channel,
		dataCh:  dataCh,
		errCh:   errCh,
		closeCh: closeCh,
	}

	c.mu.Lock()
	c.subs[channel] = sub
	c.mu.Unlock()

	c.logger.Debug().Str("channel", channel).Msg("subscribed to channel")

	return dataCh, errCh
}

// Subscribe registers a handler function for messages on the given channel.
// The handler is called in a separate goroutine for each received message.
func (c *WSClient) Subscribe(channel string, handler func([]byte) error) error {
	dataCh, errCh := c.SubscribeChannel(channel)

	c.wg.Go(func() {
		for {
			select {
			case data, ok := <-dataCh:
				if !ok {
					return
				}
				if err := handler(data); err != nil {
					c.logger.Error().Err(err).Str("channel", channel).Msg("handler error")
				}
			case err, ok := <-errCh:
				if !ok {
					return
				}
				if err != nil {
					c.logger.Error().Err(err).Str("channel", channel).Msg("subscription error")
				}
			case <-c.stopChan:
				return
			}
		}
	})

	return nil
}

// Unsubscribe removes the subscription for the given channel.
// It is an alias for UnsubscribeChannel.
func (c *WSClient) Unsubscribe(channel string) {
	c.UnsubscribeChannel(channel)
}

// UnsubscribeChannel removes the subscription for the given channel and closes its channels.
func (c *WSClient) UnsubscribeChannel(channel string) {
	c.mu.Lock()
	if sub, ok := c.subs[channel]; ok {
		close(sub.closeCh)
		close(sub.dataCh)
		close(sub.errCh)
		delete(c.subs, channel)
	}
	c.mu.Unlock()

	c.logger.Debug().Str("channel", channel).Msg("unsubscribed from channel")
}

// Subscriptions returns a list of all active subscription channel names.
func (c *WSClient) Subscriptions() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	subs := make([]string, 0, len(c.subs))
	for channel := range c.subs {
		subs = append(subs, channel)
	}
	return subs
}

// WriteMessage sends raw bytes over the websocket connection.
// It returns an error if the connection is not active.
func (c *WSClient) WriteMessage(data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil || c.state.Load() != StateConnected {
		return fmt.Errorf("websocket not connected")
	}

	return c.conn.WriteMessage(gws.OpcodeText, data)
}

// SendJSON marshals the given value to JSON and sends it over the websocket.
// It returns an error if marshaling fails or the connection is not active.
func (c *WSClient) SendJSON(v any) error {
	data, err := sonic.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	return c.WriteMessage(data)
}

// SendPing sends a ping frame to the server to keep the connection alive.
// It returns an error if the connection is not active.
func (c *WSClient) SendPing() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil || c.state.Load() != StateConnected {
		return fmt.Errorf("websocket not connected")
	}

	return c.conn.WritePing(nil)
}

func (c *WSClient) attemptReconnect() {
	if !c.state.CompareAndSwap(StateDisconnected, StateReconnecting) {
		return
	}

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		c.mu.Lock()
		attempts := c.reconnectAttempts
		c.reconnectAttempts++
		c.mu.Unlock()

		wait := c.calculateBackoff(attempts)
		c.logger.Info().
			Dur("wait", wait).
			Int("attempt", attempts+1).
			Msg("attempting reconnect")

		select {
		case <-time.After(wait):
		case <-c.stopChan:
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := c.Connect(ctx); err != nil {
			c.logger.Error().Err(err).
				Int("attempt", attempts+1).
				Msg("reconnect failed")
			cancel()
			c.state.Store(StateReconnecting)
			continue
		}
		cancel()

		c.logger.Info().Msg("reconnected successfully")
		return
	}
}

func (c *WSClient) calculateBackoff(attempts int) time.Duration {
	wait := min(c.config.ReconnectBaseWait*time.Duration(1<<uint(attempts)), c.config.ReconnectMaxWait)
	return wait
}

// ReconnectConfig holds configuration for automatic reconnection behavior.
type ReconnectConfig struct {
	// Enabled determines whether automatic reconnection is active.
	Enabled bool
	// MaxAttempts is the maximum number of reconnection attempts before giving up.
	MaxAttempts int
	// BaseWait is the initial wait duration before the first reconnection attempt.
	BaseWait time.Duration
	// MaxWait is the maximum wait duration between reconnection attempts.
	MaxWait time.Duration
	// Multiplier is the factor by which the wait duration increases after each attempt.
	Multiplier float64
}

// DefaultReconnectConfig returns a ReconnectConfig with sensible default values.
func DefaultReconnectConfig() ReconnectConfig {
	return ReconnectConfig{
		Enabled:     true,
		MaxAttempts: 10,
		BaseWait:    1 * time.Second,
		MaxWait:     30 * time.Second,
		Multiplier:  2.0,
	}
}
