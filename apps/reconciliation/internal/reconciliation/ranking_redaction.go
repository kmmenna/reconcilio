package reconciliation

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
)

const sensitiveIdentifiers = `(?i:https?://[^\s,;]+|[\p{L}\p{N}._%+\-]+@[\p{L}\p{N}.\-]+\.[\p{L}]{2,})` +
	`|(?i:\b(?:cpf|cnpj|ssn|iban|account|acct|routing|conta|ag[eê]ncia|card|cart[aã]o|chave pix)\s*[:#=-]?\s*(?:ending in\s+|final\s+)?(?:[X*•]+\s*)?[\p{L}\p{N}*•._/\-]*[0-9][\p{L}\p{N}*•._/\-]*)` +
	`|(?:\+?[0-9][0-9(). \-]{5,}[0-9])`

var sensitiveDescription = regexp.MustCompile(sensitiveIdentifiers +
	`|(?i:\b(?:titular|nome|name|customer|recipient|benefici[aá]ri[oa]|pagamento para|payment to)\s*[:=-]?\s+\p{L}+(?:[ \t]+\p{L}+){0,5})` +
	`|\p{Lu}\p{Ll}{1,}(?:(?:[ \t]+(?:d[aeo]s?|e|and))?[ \t]+\p{Lu}\p{Ll}{1,}){1,4}`,
)

var sensitiveExplanation = regexp.MustCompile(sensitiveIdentifiers + `|\[redacted_[A-Z2-7]+_[0-9]+\]`)

type descriptionRedactor struct {
	nonce        string
	placeholders map[string]string
}

func newDescriptionRedactor() *descriptionRedactor {
	return &descriptionRedactor{nonce: rand.Text(), placeholders: make(map[string]string)}
}

func (r *descriptionRedactor) clean(description string) string {
	return sensitiveDescription.ReplaceAllStringFunc(description, func(value string) string {
		key := strings.ToLower(strings.Join(strings.Fields(value), " "))
		if prior, ok := r.placeholders[key]; ok {
			return prior
		}
		placeholder := fmt.Sprintf("[redacted_%s_%d]", r.nonce, len(r.placeholders)+1)
		r.placeholders[key] = placeholder
		return placeholder
	})
}

func cleanRankingExplanation(value, replacement string) string {
	// Display localized markers for any echoed placeholders and keep original details
	// in the analyst's Transaction record.
	return strings.TrimSpace(sensitiveExplanation.ReplaceAllLiteralString(value, replacement))
}
