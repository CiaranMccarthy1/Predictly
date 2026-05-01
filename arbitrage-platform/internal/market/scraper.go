// internal/market/scraper.go
package market

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yourorg/arbitrage-platform/internal/domain"
)

// Scraper maintains a WebSocket connection to a prediction market exchange
// and decodes incoming order-book updates into MarketContract values.
// A worker pool of `numWorkers` goroutines processes incoming messages
// concurrently so that a slow parse never stalls the read loop.
type Scraper struct {
	exchange   string
	wsURL      string
	limiter    *TokenBucket
	out        chan<- domain.MarketContract
	numWorkers int
}

// NewScraper creates a new exchange scraper.
//
//	exchange:   human-readable name ("kalshi", "polymarket")
//	wsURL:      WebSocket endpoint
//	limiter:    per-exchange rate limiter
//	out:        destination channel for parsed contracts
//	numWorkers: size of the message-processing pool
func NewScraper(
	exchange, wsURL string,
	limiter *TokenBucket,
	out chan<- domain.MarketContract,
	numWorkers int,
) *Scraper {
	return &Scraper{
		exchange:   exchange,
		wsURL:      wsURL,
		limiter:    limiter,
		out:        out,
		numWorkers: numWorkers,
	}
}

// Run connects to the WebSocket endpoint and dispatches messages to the
// worker pool. It automatically reconnects on failure with exponential
// back-off capped at 30 seconds.
func (s *Scraper) Run(ctx context.Context) {
	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Scraper:%s] context cancelled, exiting", s.exchange)
			return
		default:
		}

		err := s.connectAndStream(ctx)
		if err != nil {
			log.Printf("[Scraper:%s] connection error: %v — reconnecting in %v",
				s.exchange, err, backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Exponential backoff: 1s → 2s → 4s → ... → 30s cap
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

// connectAndStream dials the WebSocket, spawns worker pool goroutines, and
// feeds raw messages into the pool via an internal channel. Returns on
// connection error or context cancellation.
func (s *Scraper) connectAndStream(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, s.wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	log.Printf("[Scraper:%s] connected to %s", s.exchange, s.wsURL)

	// Internal message bus for the worker pool
	msgCh := make(chan []byte, 256)

	var wg sync.WaitGroup
	for i := 0; i < s.numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			s.worker(ctx, workerID, msgCh)
		}(i)
	}

	// Read loop — runs until error or context cancel
	readErr := s.readLoop(ctx, conn, msgCh)

	// Shutdown workers
	close(msgCh)
	wg.Wait()

	return readErr
}

// readLoop continuously reads from the WebSocket connection and pushes
// raw message bytes into msgCh.
func (s *Scraper) readLoop(ctx context.Context, conn *websocket.Conn, msgCh chan<- []byte) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		select {
		case msgCh <- msg:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// worker pulls raw messages from msgCh, respects the rate limiter, and
// parses them into MarketContract structs before pushing downstream.
func (s *Scraper) worker(ctx context.Context, id int, msgCh <-chan []byte) {
	for {
		select {
		case <-ctx.Done():
			return
		case raw, ok := <-msgCh:
			if !ok {
				return
			}
			// Respect exchange rate limit before processing
			if err := s.limiter.Wait(ctx); err != nil {
				return
			}
			s.processMessage(ctx, raw)
		}
	}
}

// processMessage decodes a raw WebSocket message into a MarketContract.
// Exchange-specific parsing logic goes here.
func (s *Scraper) processMessage(ctx context.Context, raw []byte) {
	var contract domain.MarketContract
	if err := json.Unmarshal(raw, &contract); err != nil {
		log.Printf("[Scraper:%s] unmarshal error: %v", s.exchange, err)
		return
	}

	contract.Exchange = s.exchange
	contract.LastUpdated = time.Now()

	select {
	case s.out <- contract:
	case <-ctx.Done():
	}
}
