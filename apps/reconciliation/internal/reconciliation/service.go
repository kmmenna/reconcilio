package reconciliation

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation/reconciliationdb"
)

type Service struct {
	pool    *pgxpool.Pool
	queries *reconciliationdb.Queries
	ranking RankingOptions
}

func New(pool *pgxpool.Pool) *Service {
	return NewWithRanking(pool, DefaultRankingOptions())
}

func NewWithRanking(pool *pgxpool.Pool, ranking RankingOptions) *Service {
	return &Service{pool: pool, queries: reconciliationdb.New(pool), ranking: ranking}
}
