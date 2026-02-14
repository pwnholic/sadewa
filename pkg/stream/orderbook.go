package stream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lxzan/gws"
	"github.com/rs/zerolog"

	"sadewa/internal/ws"
	"sadewa/pkg/core"
)

type OrderBookStreamConfig struct {
	BaseConfig
	URL string
}

func DefaultOrderBookStreamConfig() OrderBookStreamConfig {
	return OrderBookStreamConfig{
		BaseConfig: DefaultBaseConfig(),
	}
}

type OrderBookStream struct {
	config  OrderBookStreamConfig
	state   *ws.State
	conn    *gws.Conn
	handler *orderbookHandler
	logger  zerolog.Logger

	mu          sync.RWMutex
	subs        map[string]*orderbookSubscription
	connectedCh chan struct{}
	closeCh     chan struct{}
	wg          sync.WaitGroup
}

type orderbookSubscription struct {
	symbol  string
	dataCh  chan *core.OrderBook
	errCh   chan error
	closeCh chan struct{}
}

type orderbookHandler struct {
	stream *OrderBookStream
}

func NewOrderBookStream(config OrderBookStreamConfig) *OrderBookStream {
	s := &OrderBookStream{
		config:      config,
		state:       &ws.State{},
		subs:        make(map[string]*orderbookSubscription),
		connectedCh: make(chan struct{}),
		closeCh:     make(chan struct{}),
		logger:      zerolog.Nop(),
	}
	s.handler = &orderbookHandler{stream: s}
	s.state.Store(StateDisconnected)
	return s
}

func (s *OrderBookStream) SetLogger(logger zerolog.Logger) {
	s.logger = logger
}

func (s *OrderBookStream) Connect(ctx context.Context) error {
	if !s.state.CompareAndSwap(StateDisconnected, StateConnecting) {
		current := s.state.Load()
		if current == StateConnected {
			return nil
		}
		return fmt.Errorf("invalid state for connect: %s", current)
	}

	socket, _, err := gws.NewClient(s.handler, &gws.ClientOption{
		Addr: s.config.URL,
	})
	if err != nil {
		s.state.Store(StateDisconnected)
		return fmt.Errorf("connect: %w", err)
	}

	s.mu.Lock()
	s.conn = socket
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		socket.ReadLoop()
	}()

	select {
	case <-s.connectedCh:
		return nil
	case <-ctx.Done():
		_ = socket.NetConn().Close()
		s.state.Store(StateDisconnected)
		return ctx.Err()
	case <-s.closeCh:
		_ = socket.NetConn().Close()
		s.state.Store(StateClosed)
		return fmt.Errorf("stream closed")
	}
}

func (s *OrderBookStream) Close() error {
	if !s.state.CompareAndSwap(StateConnected, StateClosed) &&
		!s.state.CompareAndSwap(StateConnecting, StateClosed) &&
		!s.state.CompareAndSwap(StateReconnecting, StateClosed) {
		return nil
	}

	close(s.closeCh)

	s.mu.Lock()
	if s.conn != nil {
		_ = s.conn.NetConn().Close()
	}
	for _, sub := range s.subs {
		close(sub.closeCh)
		close(sub.dataCh)
		close(sub.errCh)
	}
	s.subs = make(map[string]*orderbookSubscription)
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}

func (s *OrderBookStream) State() ConnState {
	return s.state.Load()
}

func (s *OrderBookStream) Subscribe(symbol string) (<-chan *core.OrderBook, <-chan error) {
	dataCh := make(chan *core.OrderBook, s.config.BufferSize)
	errCh := make(chan error, 1)
	closeCh := make(chan struct{})

	sub := &orderbookSubscription{
		symbol:  symbol,
		dataCh:  dataCh,
		errCh:   errCh,
		closeCh: closeCh,
	}

	s.mu.Lock()
	s.subs[symbol] = sub
	s.mu.Unlock()

	return dataCh, errCh
}

func (s *OrderBookStream) Unsubscribe(symbol string) {
	s.mu.Lock()
	if sub, ok := s.subs[symbol]; ok {
		close(sub.closeCh)
		close(sub.dataCh)
		close(sub.errCh)
		delete(s.subs, symbol)
	}
	s.mu.Unlock()
}

func (h *orderbookHandler) OnOpen(socket *gws.Conn) {
	h.stream.state.Store(StateConnected)

	h.stream.mu.Lock()
	select {
	case <-h.stream.connectedCh:
	default:
		close(h.stream.connectedCh)
	}
	h.stream.mu.Unlock()

	h.stream.logger.Info().Msg("orderbook stream connected")
	_ = socket.SetDeadline(time.Now().Add(h.stream.config.PingInterval + h.stream.config.PongWait))
}

func (h *orderbookHandler) OnClose(socket *gws.Conn, err error) {
	h.stream.state.Store(StateDisconnected)

	h.stream.logger.Warn().Err(err).Msg("orderbook stream disconnected")

	if h.stream.config.Reconnect.Enabled {
		select {
		case <-h.stream.closeCh:
			return
		default:
			go h.stream.reconnect()
		}
	}
}

func (h *orderbookHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(h.stream.config.PingInterval + h.stream.config.PongWait))
	_ = socket.WritePong(nil)
}

func (h *orderbookHandler) OnPong(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(h.stream.config.PingInterval + h.stream.config.PongWait))
}

func (h *orderbookHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	data := message.Bytes()
	if len(data) == 0 {
		return
	}

	h.stream.logger.Debug().Str("data", string(data)).Msg("received orderbook message")
}

func (s *OrderBookStream) reconnect() {
	if !s.state.CompareAndSwap(StateDisconnected, StateReconnecting) {
		return
	}

	for attempt := 0; attempt < s.config.Reconnect.MaxAttempts; attempt++ {
		select {
		case <-s.closeCh:
			return
		default:
		}

		wait := s.calculateBackoff(attempt)
		s.logger.Info().Dur("wait", wait).Int("attempt", attempt+1).Msg("attempting reconnect")

		select {
		case <-time.After(wait):
		case <-s.closeCh:
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := s.Connect(ctx)
		cancel()

		if err != nil {
			s.logger.Error().Err(err).Int("attempt", attempt+1).Msg("reconnect failed")
			s.state.Store(StateReconnecting)
			continue
		}

		s.logger.Info().Msg("reconnected successfully")
		return
	}

	s.state.Store(StateDisconnected)
	s.logger.Error().Msg("max reconnect attempts reached")
}

func (s *OrderBookStream) calculateBackoff(attempt int) time.Duration {
	wait := s.config.Reconnect.BaseWait
	for i := 0; i < attempt; i++ {
		wait = time.Duration(float64(wait) * s.config.Reconnect.Multiplier)
		if wait > s.config.Reconnect.MaxWait {
			return s.config.Reconnect.MaxWait
		}
	}
	return wait
}
