// internal/service/trade_engine.go
package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/yourorg/arbitrage-platform/internal/domain"
	"github.com/yourorg/arbitrage-platform/internal/market"
)

const masterCapitalUSD = 10_000.0 // Platform's deployed capital per signal

// TradeEngine consumes raw contracts, runs arbitrage analysis, and emits
// TradeSignals to the CopyTrader via a channel.
type TradeEngine struct {
	contractsCh <-chan domain.MarketContract // from Scrapers
	signalsCh   chan<- domain.TradeSignal    // to CopyTrader

	mu    sync.RWMutex
	books map[string]map[string]domain.MarketContract // ContractID -> Exchange -> Contract
}

func NewTradeEngine(
	contractsCh <-chan domain.MarketContract,
	signalsCh chan<- domain.TradeSignal,
) *TradeEngine {
	return &TradeEngine{
		contractsCh: contractsCh,
		signalsCh:   signalsCh,
		books:       make(map[string]map[string]domain.MarketContract),
	}
}

// Start is the main analysis loop. Each contract is evaluated in its
// own goroutine so a slow analysis never blocks the incoming stream.
func (e *TradeEngine) Start(ctx context.Context) {
	log.Println("[TradeEngine] starting cross-exchange analysis loop")
	for {
		select {
		case <-ctx.Done():
			log.Println("[TradeEngine] shutting down")
			return
		case c, ok := <-e.contractsCh:
			if !ok {
				return
			}
			go e.evaluateArbitrage(ctx, c)
		}
	}
}

func (e *TradeEngine) evaluateArbitrage(ctx context.Context, c domain.MarketContract) {
	Global.SignalsEvaluated.Add(1)

	e.mu.Lock()
	if e.books[c.ID] == nil {
		e.books[c.ID] = make(map[string]domain.MarketContract)
	}
	e.books[c.ID][c.Exchange] = c
	
	// Get the contracts to compare
	book := e.books[c.ID]
	kalshiContract, hasKalshi := book["kalshi"]
	polyContract, hasPoly := book["polymarket"]
	e.mu.Unlock()

	if !hasKalshi || !hasPoly {
		return // Need data from both exchanges to compare
	}

	signals, isArbitrage := market.AnalyzeArbitrage(kalshiContract, polyContract, masterCapitalUSD)
	if !isArbitrage {
		Global.RejectedSignals.Add(1)
		return
	}

	for _, signal := range signals {
		signal.Timestamp = time.Now()
		log.Printf("[TradeEngine] ARBITRAGE DETECTED | exchange=%s contract=%s side=%s edge=%.4f",
			signal.Contract.Exchange, signal.Contract.ID, signal.Side, signal.EV)

		Global.TradesExecuted.Add(1)

		select {
		case e.signalsCh <- signal:
		case <-ctx.Done():
		}
	}
}
