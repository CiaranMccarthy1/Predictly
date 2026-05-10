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

// AnalyzeArbitrage compares order book prices across two exchanges for the same contract
// and returns trade signals if a risk-free arbitrage exists.
func AnalyzeArbitrage(
	c1, c2 domain.MarketContract,
	masterCapitalUSD float64,
) ([]domain.TradeSignal, bool) {
	
	liquidity := c1.Liquidity
	if c2.Liquidity < liquidity {
		liquidity = c2.Liquidity
	}

	if liquidity < minLiquidity {
		return nil, false
	}

	// Arbitrage Check 1: Buy YES on c1, Buy NO on c2
	cost1 := c1.YesOdds + c2.NoOdds
	
	// Arbitrage Check 2: Buy NO on c1, Buy YES on c2
	cost2 := c1.NoOdds + c2.YesOdds

	var signals []domain.TradeSignal
	var edge float64

	if cost1 < 1.0 - minEdge {
		// Guaranteed profit: cost < $1.00, payout = $1.00
		edge = 1.0 - cost1
		size := masterCapitalUSD * edge
		if size > liquidity {
			size = liquidity
		}
		
		signals = append(signals, domain.TradeSignal{
			Contract:   c1,
			Side:       "YES",
			MasterSize: size,
			EV:         edge,
		})
		signals = append(signals, domain.TradeSignal{
			Contract:   c2,
			Side:       "NO",
			MasterSize: size,
			EV:         edge,
		})
		return signals, true
	} else if cost2 < 1.0 - minEdge {
		edge = 1.0 - cost2
		size := masterCapitalUSD * edge
		if size > liquidity {
			size = liquidity
		}

		signals = append(signals, domain.TradeSignal{
			Contract:   c1,
			Side:       "NO",
			MasterSize: size,
			EV:         edge,
		})
		signals = append(signals, domain.TradeSignal{
			Contract:   c2,
			Side:       "YES",
			MasterSize: size,
			EV:         edge,
		})
		return signals, true
	}

	return nil, false
}
