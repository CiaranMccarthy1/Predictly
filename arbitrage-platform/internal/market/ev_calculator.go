// internal/market/ev_calculator.go
package market

import (
	"github.com/yourorg/arbitrage-platform/internal/domain"
)

const (
	// minEdge is the minimum EV threshold required to fire a signal.
	// Below this the spread + slippage would eat the edge.
	minEdge = 0.02

	// minLiquidity filters out illiquid markets where our order would
	// move the price significantly.
	minLiquidity = 500.0
)

// Analyze evaluates a MarketContract against our model probability and
// returns a TradeSignal if a +EV edge exists.
//
// EV formula (binary market):
//
//	Side YES → EV = modelProb / yesOdds - 1
//	Side NO  → EV = (1-modelProb) / noOdds - 1
//
// Positive EV means we expect profit per dollar staked over a large
// number of identical bets.
func Analyze(
	c domain.MarketContract,
	modelProb float64,
	masterCapitalUSD float64,
) (domain.TradeSignal, bool) {
	if c.Liquidity < minLiquidity {
		return domain.TradeSignal{}, false
	}

	evYes := calcEV(modelProb, c.YesOdds)
	evNo := calcEV(1-modelProb, c.NoOdds)

	var side string
	var ev float64

	switch {
	case evYes >= evNo && evYes >= minEdge:
		side = "YES"
		ev = evYes
	case evNo > evYes && evNo >= minEdge:
		side = "NO"
		ev = evNo
	default:
		return domain.TradeSignal{}, false
	}

	// Size the position proportionally to edge strength, but never
	// exceed available liquidity.
	size := masterCapitalUSD * ev
	if size > c.Liquidity {
		size = c.Liquidity
	}

	return domain.TradeSignal{
		Contract:   c,
		Side:       side,
		MasterSize: size,
		EV:         ev,
		ModelProb:  modelProb,
	}, true
}

// calcEV computes expected value for a given probability and odds.
// Returns 0 if odds are zero to avoid division by zero.
func calcEV(prob, odds float64) float64 {
	if odds <= 0 {
		return 0
	}
	return prob/odds - 1
}
