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

	// ── Scrapers (Worker Pool per exchange) ─────────────────────────
	kalshiScraper := market.NewScraper(
		"kalshi",
		"wss://trading-api.kalshi.com/trade-api/ws/v2",
		kalshiLimiter, contractsCh, 4,
	)
	polymarketScraper := market.NewScraper(
		"polymarket",
		"wss://clob.polymarket.com/ws",
		polymarketLimiter, contractsCh, 4,
	)

	// ── Trade Engine ─────────────────────────────────────────────────
	engine := service.NewTradeEngine(contractsCh, signalsCh, modelProbFn)

	// ── Copy Trader ──────────────────────────────────────────────────
	copyTrader := service.NewCopyTrader(signalsCh, registry, exchangeExecuteFn)

	// ── Launch all components ────────────────────────────────────────
	go kalshiScraper.Run(ctx)
	go polymarketScraper.Run(ctx)
	go engine.Start(ctx)
	go copyTrader.Start(ctx)
	go reportMetrics(ctx)

	log.Println("[main] platform running — waiting for shutdown signal")
	<-ctx.Done()
	log.Println("[main] graceful shutdown complete")
}

func modelProbFn(c domain.MarketContract) float64 {
	// TODO: replace with real ML model call
	return 0.60
}

func exchangeExecuteFn(ctx context.Context, alloc domain.FollowerAllocation) error {
	// TODO: replace with real broker API call
	log.Printf("[stub] executing alloc user=%s size=$%.2f side=%s",
		alloc.UserID, alloc.AllocUSD, alloc.Side)
	return nil
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

func reportMetrics(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m := &service.Global
			log.Printf("[metrics] evaluated=%d executed=%d fills=%d rejected=%d",
				m.SignalsEvaluated.Load(),
				m.TradesExecuted.Load(),
				m.FollowerFills.Load(),
				m.RejectedSignals.Load(),
			)
		}
	}
}
