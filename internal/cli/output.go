package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/lehtaku/alpr/internal/domain"
)

type errorBody struct {
	Code    domain.Code    `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

// writeJSON prints a --json result.
func writeJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// writeError prints err as one JSON object with --json, otherwise as "alpr: <message>".
func writeError(w io.Writer, err *domain.Error, asJSON bool) {
	if !asJSON {
		_, _ = fmt.Fprintln(w, "alpr:", err.Message)
		return
	}

	details := err.Details
	if details == nil {
		details = map[string]any{}
	}
	_ = json.NewEncoder(w).Encode(struct {
		Error errorBody `json:"error"`
	}{errorBody{Code: err.Code, Message: err.Message, Details: details}})
}
