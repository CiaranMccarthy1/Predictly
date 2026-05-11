package service

import (
	"context"
	"log"
	"sync"

	"github.com/yourorg/arbitrage-platform/internal/domain"
)

type CopyTrader struct {
	signalsCh <-chan domain.TradeSignal
	registry  *domain.UserRegistry
	executeFn func(ctx context.Context, alloc domain.UserAllocation) error
}

func NewCopyTrader(
	signalsCh <-chan domain.TradeSignal,
	registry *domain.UserRegistry,
	executeFn func(ctx context.Context, alloc domain.UserAllocation) error,
) *CopyTrader {
	return &CopyTrader{signalsCh: signalsCh, registry: registry, executeFn: executeFn}
}

func (ct *CopyTrader) Start(ctx context.Context) {
	log.Println("[CopyTrader] ready to broadcast signals")
	for {
		select {
		case <-ctx.Done():
			log.Println("[CopyTrader] shutting down")
			return
		case sig, ok := <-ct.signalsCh:
			if !ok {
				return
			}
			go ct.broadcast(ctx, sig)
		}
	}
}

func (ct *CopyTrader) broadcast(ctx context.Context, sig domain.TradeSignal) {
	users := ct.registry.All()
	allocations := ct.proRataAllocate(sig, users)

	var wg sync.WaitGroup
	for _, alloc := range allocations {
		wg.Add(1)
		a := alloc
		go func() {
			defer wg.Done()
			if err := ct.executeFn(ctx, a); err != nil {
				log.Printf("[CopyTrader] fill error user=%s: %v", a.UserID, err)
				return
			}
			Global.UserFills.Add(1)
			log.Printf("[CopyTrader] filled user=%s alloc=$%.2f", a.UserID, a.AllocUSD)
		}()
	}
	wg.Wait()
	log.Printf("[CopyTrader] broadcast complete contract=%s fills=%d", sig.Contract.ID, len(allocations))
}

func (ct *CopyTrader) proRataAllocate(sig domain.TradeSignal, users []*domain.User) []domain.UserAllocation {
	type demand struct {
		user   *domain.User
		amount float64
	}
	var demands []demand
	totalDemand := 0.0

	for _, u := range users {
		ask := u.BalanceUSD * u.RiskSettings.RiskFraction
		if ask > u.RiskSettings.MaxPositionUSD {
			ask = u.RiskSettings.MaxPositionUSD
		}
		if ask <= 0 {
			continue
		}
		demands = append(demands, demand{user: u, amount: ask})
		totalDemand += ask
	}

	liquidity := sig.Contract.Liquidity
	scaleFactor := 1.0
	if totalDemand > liquidity {
		scaleFactor = liquidity / totalDemand
	}

	allocs := make([]domain.UserAllocation, 0, len(demands))
	for _, d := range demands {
		price := sig.Contract.YesOdds
		if sig.Side == "NO" {
			price = sig.Contract.NoOdds
		}
		allocs = append(allocs, domain.UserAllocation{
			UserID:     d.user.ID,
			ContractID: sig.Contract.ID,
			Side:       sig.Side,
			AllocUSD:   d.amount * scaleFactor,
			Price:      price,
			Status:     domain.StatusPending,
		})
	}
	return allocs
}
