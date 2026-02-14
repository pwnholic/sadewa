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

type TickerStreamConfig struct {
	BaseConfig
	URL string
}

func DefaultTickerStreamConfig() TickerStreamConfig {
	return TickerStreamConfig{
		BaseConfig: DefaultBaseConfig(),
	}
}

type TickerStream struct {
	config  TickerStreamConfig
	state   *ws.State
	conn    *gws.Conn
	handler *tickerHandler
	logger  zerolog.Logger

	mu          sync.RWMutex
	subs        map[string]*tickerSubscription
	connectedCh chan struct{}
	closeCh     chan struct{}
	wg          sync.WaitGroup
}

type tickerSubscription struct {
	symbol  string
	dataCh  chan *core.Ticker
	errCh   chan error
	closeCh chan struct{}
}

type tickerHandler struct {
	stream *TickerStream
}

func NewTickerStream(config TickerStreamConfig) *TickerStream {
	s := &TickerStream{
		config:      config,
		state:       &ws.State{},
		subs:        make(map[string]*tickerSubscription),
		connectedCh: make(chan struct{}),
		closeCh:     make(chan struct{}),
		logger:      zerolog.Nop(),
	}
	s.handler = &tickerHandler{stream: s}
	s.state.Store(StateDisconnected)
	return s
}

func (s *TickerStream) SetLogger(logger zerolog.Logger) {
	s.logger = logger
}

func (s *TickerStream) Connect(ctx context.Context) error {
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

func (s *TickerStream) Close() error {
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
	s.subs = make(map[string]*tickerSubscription)
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}

func (s *TickerStream) State() ConnState {
	return s.state.Load()
}

func (s *TickerStream) Subscribe(symbol string) (<-chan *core.Ticker, <-chan error) {
	dataCh := make(chan *core.Ticker, s.config.BufferSize)
	errCh := make(chan error, 1)
	closeCh := make(chan struct{})

	sub := &tickerSubscription{
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

func (s *TickerStream) Unsubscribe(symbol string) {
	s.mu.Lock()
	if sub, ok := s.subs[symbol]; ok {
		close(sub.closeCh)
		close(sub.dataCh)
		close(sub.errCh)
		delete(s.subs, symbol)
	}
	s.mu.Unlock()
}

func (h *tickerHandler) OnOpen(socket *gws.Conn) {
	h.stream.state.Store(StateConnected)

	h.stream.mu.Lock()
	select {
	case <-h.stream.connectedCh:
	default:
		close(h.stream.connectedCh)
	}
	h.stream.mu.Unlock()

	h.stream.logger.Info().Msg("ticker stream connected")
	_ = socket.SetDeadline(time.Now().Add(h.stream.config.PingInterval + h.stream.config.PongWait))
}

func (h *tickerHandler) OnClose(socket *gws.Conn, err error) {
	h.stream.state.Store(StateDisconnected)

	h.stream.logger.Warn().Err(err).Msg("ticker stream disconnected")

	if h.stream.config.Reconnect.Enabled {
		select {
		case <-h.stream.closeCh:
			return
		default:
			go h.stream.reconnect()
		}
	}
}

func (h *tickerHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(h.stream.config.PingInterval + h.stream.config.PongWait))
	_ = socket.WritePong(nil)
}

func (h *tickerHandler) OnPong(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(h.stream.config.PingInterval + h.stream.config.PongWait))
}

func (h *tickerHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	data := message.Bytes()
	if len(data) == 0 {
		return
	}

	h.stream.logger.Debug().Str("data", string(data)).Msg("received ticker message")
}

func (s *TickerStream) reconnect() {
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

func (s *TickerStream) calculateBackoff(attempt int) time.Duration {
	wait := s.config.Reconnect.BaseWait
	for i := 0; i < attempt; i++ {
		wait = time.Duration(float64(wait) * s.config.Reconnect.Multiplier)
		if wait > s.config.Reconnect.MaxWait {
			return s.config.Reconnect.MaxWait
		}
	}
	return wait
}
