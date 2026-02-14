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

type TradeStreamConfig struct {
	BaseConfig
	URL string
}

func DefaultTradeStreamConfig() TradeStreamConfig {
	return TradeStreamConfig{
		BaseConfig: DefaultBaseConfig(),
	}
}

type TradeStream struct {
	config  TradeStreamConfig
	state   *ws.State
	conn    *gws.Conn
	handler *tradeHandler
	logger  zerolog.Logger

	mu          sync.RWMutex
	subs        map[string]*tradeSubscription
	connectedCh chan struct{}
	closeCh     chan struct{}
	wg          sync.WaitGroup
}

type tradeSubscription struct {
	symbol  string
	dataCh  chan *core.Trade
	errCh   chan error
	closeCh chan struct{}
}

type tradeHandler struct {
	stream *TradeStream
}

func NewTradeStream(config TradeStreamConfig) *TradeStream {
	s := &TradeStream{
		config:      config,
		state:       &ws.State{},
		subs:        make(map[string]*tradeSubscription),
		connectedCh: make(chan struct{}),
		closeCh:     make(chan struct{}),
		logger:      zerolog.Nop(),
	}
	s.handler = &tradeHandler{stream: s}
	s.state.Store(StateDisconnected)
	return s
}

func (s *TradeStream) SetLogger(logger zerolog.Logger) {
	s.logger = logger
}

func (s *TradeStream) Connect(ctx context.Context) error {
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

func (s *TradeStream) Close() error {
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
	s.subs = make(map[string]*tradeSubscription)
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}

func (s *TradeStream) State() ConnState {
	return s.state.Load()
}

func (s *TradeStream) Subscribe(symbol string) (<-chan *core.Trade, <-chan error) {
	dataCh := make(chan *core.Trade, s.config.BufferSize)
	errCh := make(chan error, 1)
	closeCh := make(chan struct{})

	sub := &tradeSubscription{
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

func (s *TradeStream) Unsubscribe(symbol string) {
	s.mu.Lock()
	if sub, ok := s.subs[symbol]; ok {
		close(sub.closeCh)
		close(sub.dataCh)
		close(sub.errCh)
		delete(s.subs, symbol)
	}
	s.mu.Unlock()
}

func (h *tradeHandler) OnOpen(socket *gws.Conn) {
	h.stream.state.Store(StateConnected)

	h.stream.mu.Lock()
	select {
	case <-h.stream.connectedCh:
	default:
		close(h.stream.connectedCh)
	}
	h.stream.mu.Unlock()

	h.stream.logger.Info().Msg("trade stream connected")
	_ = socket.SetDeadline(time.Now().Add(h.stream.config.PingInterval + h.stream.config.PongWait))
}

func (h *tradeHandler) OnClose(socket *gws.Conn, err error) {
	h.stream.state.Store(StateDisconnected)

	h.stream.logger.Warn().Err(err).Msg("trade stream disconnected")

	if h.stream.config.Reconnect.Enabled {
		select {
		case <-h.stream.closeCh:
			return
		default:
			go h.stream.reconnect()
		}
	}
}

func (h *tradeHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(h.stream.config.PingInterval + h.stream.config.PongWait))
	_ = socket.WritePong(nil)
}

func (h *tradeHandler) OnPong(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(h.stream.config.PingInterval + h.stream.config.PongWait))
}

func (h *tradeHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	data := message.Bytes()
	if len(data) == 0 {
		return
	}

	h.stream.logger.Debug().Str("data", string(data)).Msg("received trade message")
}

func (s *TradeStream) reconnect() {
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

func (s *TradeStream) calculateBackoff(attempt int) time.Duration {
	wait := s.config.Reconnect.BaseWait
	for i := 0; i < attempt; i++ {
		wait = time.Duration(float64(wait) * s.config.Reconnect.Multiplier)
		if wait > s.config.Reconnect.MaxWait {
			return s.config.Reconnect.MaxWait
		}
	}
	return wait
}
