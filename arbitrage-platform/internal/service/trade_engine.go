// internal/service/trade_engine.go
package service

import (
	"context"
	"log"
	"time"

	"github.com/yourorg/arbitrage-platform/internal/domain"
	"github.com/yourorg/arbitrage-platform/internal/market"
)

const masterCapitalUSD = 10_000.0 // Platform's deployed capital per signal

// TradeEngine consumes raw contracts, runs EV analysis, and emits
// TradeSignals to the CopyTrader via a channel.
type TradeEngine struct {
	contractsCh <-chan domain.MarketContract // from Scrapers
	signalsCh   chan<- domain.TradeSignal    // to CopyTrader
	modelProbFn func(domain.MarketContract) float64
}

func NewTradeEngine(
	contractsCh <-chan domain.MarketContract,
	signalsCh chan<- domain.TradeSignal,
	modelProbFn func(domain.MarketContract) float64,
) *TradeEngine {
	return &TradeEngine{
		contractsCh: contractsCh,
		signalsCh:   signalsCh,
		modelProbFn: modelProbFn,
	}
}

// Start is the main analysis loop. Each contract is evaluated in its
// own goroutine so a slow analysis never blocks the incoming stream.
func (e *TradeEngine) Start(ctx context.Context) {
	log.Println("[TradeEngine] starting analysis loop")
	for {
		select {
		case <-ctx.Done():
			log.Println("[TradeEngine] shutting down")
			return
		case c, ok := <-e.contractsCh:
			if !ok {
				return
			}
			// Spawn an independent goroutine per contract evaluation.
			// A stalled exchange API call cannot block the pool.
			go e.evaluate(ctx, c)
		}
	}
}

func (e *TradeEngine) evaluate(ctx context.Context, c domain.MarketContract) {
	Global.SignalsEvaluated.Add(1)

	modelProb := e.modelProbFn(c)
	signal, isPositiveEV := market.Analyze(c, modelProb, masterCapitalUSD)
	if !isPositiveEV {
		Global.RejectedSignals.Add(1)
		return
	}

	signal.Timestamp = time.Now()
	log.Printf("[TradeEngine] +EV signal | contract=%s side=%s EV=%.4f",
		c.ID, signal.Side, signal.EV)

	Global.TradesExecuted.Add(1)

	select {
	case e.signalsCh <- signal:
	case <-ctx.Done():
	}
}
