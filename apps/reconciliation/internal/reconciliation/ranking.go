package reconciliation

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation/reconciliationdb"
)

const (
	maxRankingCandidates   = 12
	maxRankingInputBytes   = 12 * 1024
	maxExplanationRunes    = 400
	rankingPublicationTime = 5 * time.Second
)

// CandidateRanker compares every alternative in one request. Identifiers and descriptions
// are prepared by the application before a provider receives them.
type CandidateRanker interface {
	Rank(context.Context, RankingInput) (RankingResult, error)
}

type RankingInput struct {
	InternalDescription string             `json:"internal_description"`
	Candidates          []RankingCandidate `json:"candidates"`
}

type RankingCandidate struct {
	ID          string `json:"candidate_id"`
	Description string `json:"description"`
}

type RankingExplanations struct {
	Portuguese string `json:"pt-BR"`
	English    string `json:"en"`
}

type CandidateAssessment struct {
	CandidateID  string              `json:"candidate_id"`
	Score        *int                `json:"score"`
	Explanations RankingExplanations `json:"explanations"`
}

type RankingUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// RankingResult can carry model and usage metadata even when the provider returns an error.
type RankingResult struct {
	Ranking       []CandidateAssessment
	Model         string
	Usage         *RankingUsage
	InvalidReason string // Fixed adapter code; never include response text or decoder errors.
}

var (
	ErrRankingRefused    = errors.New("ranking provider refused the request")
	ErrRankingIncomplete = errors.New("ranking provider returned an incomplete response")
	ErrInvalidRanking    = errors.New("ranking provider returned an invalid response")
	errRankingJobLimit   = errors.New("ranking job limit reached")
	errRankingDailyLimit = errors.New("ranking daily limit reached")
)

type RankingEvent struct {
	Provider           string
	Model              string
	Attempted          bool
	Duration           time.Duration
	StartedAt          time.Time
	Usage              *RankingUsage
	FallbackReason     string
	InvalidReason      string
	ExpectedCandidates int
	ReturnedCandidates *int
}

type RankingOptions struct {
	Ranker      CandidateRanker
	Provider    string
	Model       string
	PerJobLimit int32
	DailyLimit  int32
	Timeout     time.Duration
	Observe     func(context.Context, RankingEvent)
}

func DefaultRankingOptions() RankingOptions {
	return RankingOptions{Provider: "openai", PerJobLimit: 5, DailyLimit: 50, Timeout: 15 * time.Second}
}

type RankingAssessment struct {
	EvaluationID string              `json:"evaluation_id" format:"uuid"`
	Rank         int32               `json:"rank" minimum:"1"`
	Score        int32               `json:"score" minimum:"0" maximum:"100"`
	Explanations RankingExplanations `json:"explanations"`
}

func (s *Service) rankReviewGroups(ctx context.Context, attempt *AccountAttempt) (map[pgtype.UUID]rankedAlternatives, error) {
	if s.ranking.Ranker == nil && s.ranking.Observe == nil {
		return nil, nil
	}
	pairs, err := s.queries.ListReviewCandidates(ctx, reconciliationdb.ListReviewCandidatesParams{AccountID: attempt.accountID})
	if err != nil {
		return nil, fmt.Errorf("find candidates for ranking: %w", err)
	}
	evaluations := make(map[pgtype.UUID]rankedAlternatives)
	for group := range reviewGroups(pairs) {
		alternativeIDs := make([]pgtype.UUID, 0, len(group))
		for _, pair := range group {
			if pair.ProposalID.Valid {
				alternativeIDs = append(alternativeIDs, pair.ProposalID)
			}
		}
		if len(alternativeIDs) == len(group) {
			covered, err := s.queries.HasCoveringRankingEvaluation(ctx, reconciliationdb.HasCoveringRankingEvaluationParams{
				InternalID: group[0].InternalID, ProposalIds: alternativeIDs,
			})
			if err != nil {
				return nil, fmt.Errorf("check for existing ranking evaluation: %w", err)
			}
			if covered {
				continue
			}
		}

		alternatives, err := s.evaluateRanking(ctx, attempt, group)
		if err != nil {
			return nil, err
		}
		if len(alternatives) > 0 {
			evaluations[group[0].InternalID] = alternatives
		}
	}
	return evaluations, nil
}

type rankedAlternative struct {
	ProposalID   pgtype.UUID
	Rank         int32
	Score        int32
	Explanations RankingExplanations
}

type rankedAlternatives map[pgtype.UUID]rankedAlternative // by external Transaction ID

func (s *Service) evaluateRanking(ctx context.Context, attempt *AccountAttempt, group []reconciliationdb.ListReviewCandidatesRow) (rankedAlternatives, error) {
	event := RankingEvent{Provider: s.ranking.Provider, Model: s.ranking.Model, ExpectedCandidates: len(group)}
	if s.ranking.Observe != nil {
		defer func() { s.ranking.Observe(ctx, event) }()
	}
	if s.ranking.Ranker == nil {
		event.FallbackReason = "disabled"
		return nil, nil
	}

	input, ok := prepareRankingInput(group)
	if !ok {
		event.FallbackReason = "oversized_group"
		return nil, nil
	}
	timeout := s.ranking.Timeout
	if deadline, bounded := ctx.Deadline(); bounded {
		// Preserve time for the final eligibility check and engine fallback.
		timeout = min(timeout, time.Until(deadline)-rankingPublicationTime)
		if timeout <= 0 {
			event.FallbackReason = "job_deadline"
			return nil, nil
		}
	}
	if err := s.reserveRankingUsage(ctx, attempt); err != nil {
		switch {
		case errors.Is(err, errRankingJobLimit):
			event.FallbackReason = "job_limit"
		case errors.Is(err, errRankingDailyLimit):
			event.FallbackReason = "daily_limit"
		default:
			event.FallbackReason = "reservation_error"
			return nil, fmt.Errorf("reserve ranking usage: %w", err)
		}
		return nil, nil
	}

	event.Attempted = true
	event.StartedAt = time.Now()
	providerCtx, cancel := context.WithTimeout(ctx, timeout)
	result, err := s.ranking.Ranker.Rank(providerCtx, input)
	contextErr := providerCtx.Err()
	cancel()
	event.Duration = time.Since(event.StartedAt)
	event.Model = cmp.Or(result.Model, event.Model)
	event.Usage = result.Usage

	switch {
	case errors.Is(contextErr, context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded):
		event.FallbackReason = "timeout"
	case ctx.Err() != nil:
		event.FallbackReason = "cancelled"
		return nil, context.Cause(ctx)
	case errors.Is(err, ErrRankingRefused):
		event.FallbackReason = "refused"
	case errors.Is(err, ErrRankingIncomplete):
		event.FallbackReason = "incomplete"
	case errors.Is(err, ErrInvalidRanking):
		event.FallbackReason = "invalid_ranking"
		event.InvalidReason = cmp.Or(result.InvalidReason, "invalid_provider_response")
		if result.Ranking != nil {
			event.ReturnedCandidates = new(len(result.Ranking))
		}
	case err != nil:
		event.FallbackReason = "provider_error"
	default:
		alternatives, reason := validateRanking(input, result.Ranking, group)
		if reason == "" {
			return alternatives, nil
		}
		event.FallbackReason = "invalid_ranking"
		event.InvalidReason = reason
		event.ReturnedCandidates = new(len(result.Ranking))
	}
	return nil, nil
}

func (s *Service) reserveRankingUsage(ctx context.Context, attempt *AccountAttempt) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := attempt.verify(ctx, queries); err != nil {
			return err
		}
		used, err := queries.ReserveJobRankingUsage(ctx, reconciliationdb.ReserveJobRankingUsageParams{
			JobID: attempt.jobID, JobLimit: s.ranking.PerJobLimit,
		})
		if err != nil {
			return err
		}
		if used == 0 {
			return errRankingJobLimit
		}
		used, err = queries.ReserveDailyRankingUsage(ctx, s.ranking.DailyLimit)
		if err != nil {
			return err
		}
		if used == 0 {
			return errRankingDailyLimit
		}
		return attempt.verify(ctx, queries)
	})
}

func prepareRankingInput(group []reconciliationdb.ListReviewCandidatesRow) (RankingInput, bool) {
	if len(group) > maxRankingCandidates {
		return RankingInput{}, false
	}
	size := len(group[0].InternalDescription)
	for _, pair := range group {
		size += len(pair.ExternalDescription)
	}
	if size > maxRankingInputBytes {
		return RankingInput{}, false
	}
	redactor := newDescriptionRedactor()
	input := RankingInput{
		InternalDescription: redactor.clean(group[0].InternalDescription),
		Candidates:          make([]RankingCandidate, 0, len(group)),
	}
	for i, pair := range group {
		input.Candidates = append(input.Candidates, RankingCandidate{
			ID:          fmt.Sprintf("c_%s_%d", redactor.nonce, i+1),
			Description: redactor.clean(pair.ExternalDescription),
		})
	}
	data, err := json.Marshal(input)
	return input, err == nil && len(data) <= maxRankingInputBytes
}

func validateRanking(input RankingInput, result []CandidateAssessment, group []reconciliationdb.ListReviewCandidatesRow) (rankedAlternatives, string) {
	if len(input.Candidates) != len(result) {
		return nil, "candidate_count_mismatch"
	}
	supplied := make(map[string]reconciliationdb.ListReviewCandidatesRow, len(group))
	for i, candidate := range input.Candidates {
		supplied[candidate.ID] = group[i]
	}
	validated := make(rankedAlternatives, len(result))
	for i, assessment := range result {
		pair, known := supplied[assessment.CandidateID]
		if !known {
			return nil, "unknown_candidate_id"
		}
		if _, duplicate := validated[pair.ExternalID]; duplicate {
			return nil, "duplicate_candidate_id"
		}
		if assessment.Score == nil {
			return nil, "missing_score"
		}
		if *assessment.Score < 0 || *assessment.Score > 100 {
			return nil, "score_out_of_range"
		}
		pt, reason := validateExplanation(assessment.Explanations.Portuguese, "[dado ocultado]")
		if reason != "" {
			return nil, "pt_br_explanation_" + reason
		}
		en, reason := validateExplanation(assessment.Explanations.English, "[redacted detail]")
		if reason != "" {
			return nil, "en_explanation_" + reason
		}
		validated[pair.ExternalID] = rankedAlternative{
			ProposalID: pair.ProposalID, Rank: int32(i + 1), Score: int32(*assessment.Score),
			Explanations: RankingExplanations{Portuguese: pt, English: en},
		}
	}
	return validated, ""
}

func validateExplanation(value, replacement string) (string, string) {
	value = strings.TrimSpace(value)
	if reason := explanationIssue(value); reason != "" {
		return "", reason
	}
	value = cleanRankingExplanation(value, replacement)
	if reason := explanationIssue(value); reason != "" {
		return "", reason + "_after_redaction"
	}
	return value, ""
}

func explanationIssue(value string) string {
	switch {
	case value == "":
		return "missing"
	case !utf8.ValidString(value) || strings.ContainsRune(value, 0):
		return "invalid_text"
	case utf8.RuneCountInString(value) > maxExplanationRunes:
		return "too_long"
	default:
		return ""
	}
}

func reviewGroups(pairs []reconciliationdb.ListReviewCandidatesRow) iter.Seq[[]reconciliationdb.ListReviewCandidatesRow] {
	return func(yield func([]reconciliationdb.ListReviewCandidatesRow) bool) {
		for len(pairs) > 0 {
			end := slices.IndexFunc(pairs, func(pair reconciliationdb.ListReviewCandidatesRow) bool {
				return pair.InternalID != pairs[0].InternalID
			})
			if end < 0 {
				end = len(pairs)
			}
			if !yield(pairs[:end:end]) {
				return
			}
			pairs = pairs[end:]
		}
	}
}
