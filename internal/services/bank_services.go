package services

import (
	"context"
	"sort"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/banks"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/pkg/logger"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type BanksService struct {
	Clients map[string]banks.Client // ключи: "vbank","abank","sbank"
}

func NewBanksService(clients map[string]banks.Client) *BanksService {
	return &BanksService{Clients: clients}
}

func (s *BanksService) AccountsAll(ctx context.Context, userID uuid.UUID) ([]banks.Account, error) {
	out := make([]banks.Account, 0, 8)
	g, ctx := errgroup.WithContext(ctx)

	type result struct {
		accounts []banks.Account
	}
	resCh := make(chan result, len(s.Clients))

	for name, cl := range s.Clients {
		cl := cl
		name := name
		g.Go(func() error {
			accs, err := cl.Accounts(ctx, userID)
			if err != nil {
				logger.Log.Warnf("banks: Accounts(%s) error: %v", name, err)
				return nil
			}
			resCh <- result{accounts: accs}
			return nil
		})
	}

	_ = g.Wait()
	close(resCh)

	for r := range resCh {
		out = append(out, r.accounts...)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *BanksService) TransactionsAll(ctx context.Context, userID uuid.UUID, since time.Time, limit int) ([]banks.Transaction, error) {
	all := make([]banks.Transaction, 0, 512)
	g, ctx := errgroup.WithContext(ctx)

	type result struct {
		txs []banks.Transaction
	}
	resCh := make(chan result, len(s.Clients))

	for name, cl := range s.Clients {
		cl := cl
		name := name
		g.Go(func() error {
			txs, err := cl.Transactions(ctx, userID, since, limit) 
			if err != nil {
				logger.Log.Warnf("banks: Transactions(%s) error: %v", name, err)
				return nil
			}
			resCh <- result{txs: txs}
			return nil
		})
	}

	_ = g.Wait()
	close(resCh)

	for r := range resCh {
		all = append(all, r.txs...)
	}

	sort.SliceStable(all, func(i, j int) bool {
		return all[i].BookedAt.After(all[j].BookedAt)
	})

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}
