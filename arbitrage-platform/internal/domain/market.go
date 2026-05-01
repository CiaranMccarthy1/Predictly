// internal/domain/market.go
package domain

import "time"

// MarketContract represents a single tradeable prediction market contract.
type MarketContract struct {
	ID          string
	Exchange    string
	Question    string
	YesOdds     float64 // Implied probability from exchange (0-1)
	NoOdds      float64
	Liquidity   float64 // Total available liquidity in USD
	LastUpdated time.Time
}

// TradeSignal is produced by the EV engine and consumed by the CopyTrader.
type TradeSignal struct {
	Contract  MarketContract
	Side      string  // "YES" | "NO"
	MasterSize float64 // Total USD the platform allocates
	EV        float64 // Expected value per dollar staked
	ModelProb float64 // Our model's probability estimate
	Timestamp time.Time
}
