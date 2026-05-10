package market

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/yourorg/arbitrage-platform/internal/domain"
)

// MockScraper generates synthetic market data for portfolio demonstration.
type MockScraper struct {
	exchange string
	limiter  *TokenBucket
	out      chan<- domain.MarketContract
	rng      *rand.Rand
}

// NewMockScraper creates a new simulated exchange scraper.
func NewMockScraper(exchange string, limiter *TokenBucket, out chan<- domain.MarketContract) *MockScraper {
	return &MockScraper{
		exchange: exchange,
		limiter:  limiter,
		out:      out,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Run starts the simulated market data stream.
func (s *MockScraper) Run(ctx context.Context) {
	log.Printf("[MockScraper:%s] starting simulation stream", s.exchange)

	// Generate data at a steady pace
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[MockScraper:%s] context cancelled, exiting", s.exchange)
			return
		case <-ticker.C:
			if err := s.limiter.Wait(ctx); err != nil {
				return
			}
			
			// Generate realistic looking odds
			yesOdds := 0.3 + s.rng.Float64()*0.4 // 0.30 - 0.70
			noOdds := 1.0 - yesOdds + (s.rng.Float64()*0.02 - 0.01) // Slightly inefficient market
			
			if noOdds <= 0 {
				noOdds = 0.01
			}

			// Use a shared pool of contract IDs (0-9) so the engine can match Kalshi and Polymarket
			sharedContractID := s.rng.Intn(10)
			
			contract := domain.MarketContract{
				ID:          fmt.Sprintf("CONTRACT-%d", sharedContractID),
				Exchange:    s.exchange,
				Question:    fmt.Sprintf("Will event %d happen?", sharedContractID),
				YesOdds:     yesOdds,
				NoOdds:      noOdds,
				Liquidity:   1000.0 + s.rng.Float64()*9000.0, // $1k - $10k liquidity
				LastUpdated: time.Now(),
			}
			

			select {
			case s.out <- contract:
			case <-ctx.Done():
				return
			}
		}
	}
}
