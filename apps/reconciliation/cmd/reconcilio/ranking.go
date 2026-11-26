package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/openairank"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation"
)

func configureRanking() (reconciliation.RankingOptions, error) {
	options := reconciliation.DefaultRankingOptions()
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	options.Model = strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if apiKey != "" && options.Model != "" {
		ranker, err := openairank.New(apiKey, options.Model, os.Getenv("OPENAI_RESPONSES_URL"))
		if err != nil {
			return options, err
		}
		options.Ranker = ranker
	}
	for key, dest := range map[string]*int32{
		"RECONCILIO_AI_PER_JOB_LIMIT": &options.PerJobLimit,
		"RECONCILIO_AI_DAILY_LIMIT":   &options.DailyLimit,
	} {
		if value := os.Getenv(key); value != "" {
			limit, err := strconv.ParseInt(value, 10, 32)
			if err != nil || limit < 0 {
				return options, fmt.Errorf("%s must be a non-negative integer", key)
			}
			*dest = int32(limit)
		}
	}
	if options.Ranker != nil {
		options.Observe = func(ctx context.Context, event reconciliation.RankingEvent) {
			if event.FallbackReason != "" {
				slog.InfoContext(ctx, "Review group using engine order", "reason", event.FallbackReason, "provider", event.Provider)
			}
		}
	}
	return options, nil
}
