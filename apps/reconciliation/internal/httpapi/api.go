package httpapi

import (
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// Problem describes an HTTP failure with a machine-readable code.
type Problem struct {
	huma.ErrorModel
	Code    string  `json:"code"`
	Issues  []Issue `json:"issues,omitempty"`
	Context any     `json:"context,omitempty"`
}

// Issue identifies a failed input field and, for CSV data, its physical line.
type Issue struct {
	Line   int    `json:"line,omitempty" minimum:"1"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func NewProblem(status int, code, detail string) *Problem {
	return &Problem{
		ErrorModel: huma.ErrorModel{Status: status, Title: http.StatusText(status), Detail: detail},
		Code:       code,
	}
}

// Huma uses a process-wide factory for both validation responses and OpenAPI schemas.
var configureProblems = sync.OnceFunc(func() {
	huma.NewError = func(status int, detail string, causes ...error) huma.StatusError {
		code := "request_failed"
		switch status {
		case http.StatusUnprocessableEntity:
			code = "validation_failed"
		case http.StatusInternalServerError:
			code = "internal_error"
		}
		problem := NewProblem(status, code, detail)
		for _, cause := range causes {
			if cause != nil {
				problem.Add(cause)
			}
		}
		for i, detail := range problem.Errors {
			if detail != nil {
				// Keep messages and field locations without echoing input values.
				safe := *detail
				safe.Value = nil
				problem.Errors[i] = &safe
			}
		}
		return problem
	}
})

func New(mux *http.ServeMux) huma.API {
	configureProblems()
	return humago.New(mux, huma.DefaultConfig("Reconcilio API", "0.1.0"))
}
