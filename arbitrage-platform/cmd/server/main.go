package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourorg/arbitrage-platform/internal/domain"
	"github.com/yourorg/arbitrage-platform/internal/market"
	"github.com/yourorg/arbitrage-platform/internal/service"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ── Channels ────────────────────────────────────────────────────
	contractsCh := make(chan domain.MarketContract, 1024)
	signalsCh := make(chan domain.TradeSignal, 256)

	// ── User Registry ────────────────────────────────────────────────
	registry := domain.NewUserRegistry()
	seedUsers(registry)

	// ── Rate Limiters (exchange-specific) ───────────────────────────
	kalshiLimiter := market.NewTokenBucket(10, 10)
	polymarketLimiter := market.NewTokenBucket(5, 5)

	// ── Scrapers (Mocked for Portfolio) ─────────────────────────────
	kalshiScraper := market.NewMockScraper("kalshi", kalshiLimiter, contractsCh)
	polymarketScraper := market.NewMockScraper("polymarket", polymarketLimiter, contractsCh)

	// ── Trade Engine ─────────────────────────────────────────────────
	engine := service.NewTradeEngine(contractsCh, signalsCh)

	// ── Execution Stub (with Settlement Simulation) ──────────────────
	exchangeExecuteFn := func(ctx context.Context, alloc domain.FollowerAllocation) error {
		newBal, found := registry.DeductBalance(alloc.UserID, alloc.AllocUSD)
		if !found {
			return fmt.Errorf("user %s not found", alloc.UserID)
		}
		log.Printf("[mock-broker] executed buy user=%s size=$%.2f price=%.2f newBal=$%.2f",
			alloc.UserID, alloc.AllocUSD, alloc.Price, newBal)

		// Simulate settlement after 10 seconds
		go func() {
			select {
			case <-time.After(10 * time.Second):
				// Deterministic settlement for pure cross-exchange arbitrage simulation
				// Ensures if we buy YES on Kalshi and NO on Polymarket, exactly ONE wins.
				hash := 0
				for _, char := range alloc.ContractID {
					hash += int(char)
				}
				yesWins := hash%2 == 0
				won := (alloc.Side == "YES" && yesWins) || (alloc.Side == "NO" && !yesWins)
				var payout, pnl float64
				if won {
					// Payout = Quantity * $1.00. Quantity = Cost / Price
					payout = alloc.AllocUSD / alloc.Price
					pnl = payout - alloc.AllocUSD
					registry.CreditBalance(alloc.UserID, payout, pnl)
					log.Printf("[mock-broker] SETTLED WIN user=%s contract=%s pnl=+$%.2f",
						alloc.UserID, alloc.ContractID, pnl)
				} else {
					pnl = -alloc.AllocUSD
					registry.CreditBalance(alloc.UserID, 0, pnl)
					log.Printf("[mock-broker] SETTLED LOSS user=%s contract=%s pnl=-$%.2f",
						alloc.UserID, alloc.ContractID, -pnl)
				}
			case <-ctx.Done():
				return
			}
		}()

		return nil
	}

	// ── Copy Trader ──────────────────────────────────────────────────
	copyTrader := service.NewCopyTrader(signalsCh, registry, exchangeExecuteFn)

	// ── Launch all components ────────────────────────────────────────
	go kalshiScraper.Run(ctx)
	go polymarketScraper.Run(ctx)
	go engine.Start(ctx)
	go copyTrader.Start(ctx)
	go reportMetrics(ctx, registry)

	log.Println("[main] platform running — waiting for shutdown signal")
	<-ctx.Done()
	log.Println("[main] graceful shutdown complete")
}


func seedUsers(r *domain.UserRegistry) {
	for i := 1; i <= 5; i++ {
		r.Set(&domain.User{
			ID:         fmt.Sprintf("user-%d", i),
			BalanceUSD: float64(1000 * i),
			RiskSettings: domain.RiskSettings{
				MaxPositionUSD: 500,
				RiskFraction:   0.05,
			},
		})
	}
}

func reportMetrics(ctx context.Context, r *domain.UserRegistry) {
	ticker := time.NewTicker(5 * time.Second) // Faster tick for demo purposes
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m := &service.Global
			log.Println("=========================================================")
			log.Printf("[metrics] evaluated=%d executed=%d fills=%d rejected=%d",
				m.SignalsEvaluated.Load(),
				m.TradesExecuted.Load(),
				m.FollowerFills.Load(),
				m.RejectedSignals.Load(),
			)
			log.Println("[balances]")
			for _, u := range r.All() {
				log.Printf("  %s: $%.2f (PnL: $%.2f)", u.ID, u.BalanceUSD, u.TotalPnL)
			}
			log.Println("=========================================================")
		}
	}
}
