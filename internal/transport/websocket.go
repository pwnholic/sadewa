package transport

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/lxzan/gws"
	"github.com/rs/zerolog"

	"sadewa/internal/ws"
)

type WSConfig struct {
	URL               string
	ReconnectEnabled  bool
	ReconnectMaxWait  time.Duration
	ReconnectBaseWait time.Duration
	PingInterval      time.Duration
	PongWait          time.Duration
	BufferSize        int
}

type WSClient struct {
	config  WSConfig
	state   *ws.State
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
		state:         &ws.State{},
		subs:          make(map[string]*wsSubscription),
		connectedChan: make(chan struct{}),
		stopChan:      make(chan struct{}),
		logger:        zerolog.Nop(),
	}
	client.state.Store(ws.StateDisconnected)
	client.handler = &wsEventHandler{client: client}
	return client
}

func (c *WSClient) SetLogger(logger zerolog.Logger) {
	c.logger = logger
}

func (h *wsEventHandler) OnOpen(socket *gws.Conn) {
	h.client.state.Store(ws.StateConnected)

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
	h.client.state.Store(ws.StateDisconnected)

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

func (c *WSClient) Connect(ctx context.Context) error {
	if !c.state.CompareAndSwap(ws.StateDisconnected, ws.StateConnecting) {
		current := c.state.Load()
		if current == ws.StateConnected {
			return nil
		}
		return fmt.Errorf("invalid state for connect: %s", current)
	}

	socket, _, err := gws.NewClient(c.handler, &gws.ClientOption{
		Addr: c.config.URL,
	})
	if err != nil {
		c.state.Store(ws.StateDisconnected)
		return fmt.Errorf("connect websocket: %w", err)
	}

	c.mu.Lock()
	c.conn = socket
	c.mu.Unlock()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		socket.ReadLoop()
	}()

	select {
	case <-c.connectedChan:
		return nil
	case <-ctx.Done():
		_ = socket.NetConn().Close()
		c.state.Store(ws.StateDisconnected)
		return ctx.Err()
	case <-c.stopChan:
		_ = socket.NetConn().Close()
		c.state.Store(ws.StateClosed)
		return fmt.Errorf("client stopped")
	}
}

func (c *WSClient) Close() error {
	if !c.state.CompareAndSwap(ws.StateConnected, ws.StateClosed) &&
		!c.state.CompareAndSwap(ws.StateConnecting, ws.StateClosed) &&
		!c.state.CompareAndSwap(ws.StateReconnecting, ws.StateClosed) &&
		!c.state.CompareAndSwap(ws.StateDisconnected, ws.StateClosed) {
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

func (c *WSClient) State() ws.ConnState {
	return c.state.Load()
}

func (c *WSClient) IsConnected() bool {
	return c.state.Load() == ws.StateConnected
}

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

func (c *WSClient) Subscriptions() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	subs := make([]string, 0, len(c.subs))
	for channel := range c.subs {
		subs = append(subs, channel)
	}
	return subs
}

func (c *WSClient) WriteMessage(data []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil || c.state.Load() != ws.StateConnected {
		return fmt.Errorf("websocket not connected")
	}

	return c.conn.WriteMessage(gws.OpcodeText, data)
}

func (c *WSClient) SendJSON(v any) error {
	data, err := sonic.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	return c.WriteMessage(data)
}

func (c *WSClient) SendPing() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil || c.state.Load() != ws.StateConnected {
		return fmt.Errorf("websocket not connected")
	}

	return c.conn.WritePing(nil)
}

func (c *WSClient) attemptReconnect() {
	if !c.state.CompareAndSwap(ws.StateDisconnected, ws.StateReconnecting) {
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
			c.state.Store(ws.StateReconnecting)
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

type ReconnectConfig struct {
	Enabled     bool
	MaxAttempts int
	BaseWait    time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

func DefaultReconnectConfig() ReconnectConfig {
	return ReconnectConfig{
		Enabled:     true,
		MaxAttempts: 10,
		BaseWait:    1 * time.Second,
		MaxWait:     30 * time.Second,
		Multiplier:  2.0,
	}
}
