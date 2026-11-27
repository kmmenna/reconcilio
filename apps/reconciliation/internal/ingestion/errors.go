package ingestion

// importIssue retains the CSV location and reason independently of the HTTP adapter.
type importIssue struct {
	Line   int    `json:"line,omitzero"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
	Value  string `json:"value,omitempty"`
}

type invalidImport struct {
	code   string
	detail string
	issues []importIssue
}

func (e *invalidImport) Error() string { return e.detail }

func importProblem(code, detail string, issues ...importIssue) error {
	return &invalidImport{code: code, detail: detail, issues: issues}
}

type conflictsChanged struct{ preview importView }

func (*conflictsChanged) Error() string { return "import conflicts changed" }

type conflictsRequireReview struct{ preview importView }

func (*conflictsRequireReview) Error() string { return "import conflicts require review" }

type sourceSideChanged struct {
	previewSide string
	currentSide string
}

func (*sourceSideChanged) Error() string { return "source side changed since preview" }

type coverageBlocked struct{ overlap coverageOverlap }

func (*coverageBlocked) Error() string { return "import coverage overlaps an earlier import" }
