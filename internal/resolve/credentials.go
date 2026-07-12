package resolve

import (
	"fmt"
	"strings"
)

// CredentialStatus describes whether a stage that needs BYOK env is ready.
type CredentialStatus struct {
	Stage   string   `json:"stage"`
	Ready   bool     `json:"ready"`
	Missing []string `json:"missing,omitempty"` // env var names that are empty/unset
}

// CredentialStatuses reports BYOK readiness for stages in the effective pipeline
// that declare token_env / secret_env (or require them).
func CredentialStatuses(stages []Stage) []CredentialStatus {
	out := make([]CredentialStatus, 0)
	for _, s := range stages {
		var need []string
		switch s.Type {
		case "logo-dev":
			if strings.TrimSpace(s.TokenEnv) != "" {
				need = append(need, s.TokenEnv)
			} else {
				need = append(need, "(token_env unset)")
			}
		case "noun-project":
			if strings.TrimSpace(s.TokenEnv) != "" {
				need = append(need, s.TokenEnv)
			} else {
				need = append(need, "(token_env unset)")
			}
			if strings.TrimSpace(s.SecretEnv) != "" {
				need = append(need, s.SecretEnv)
			} else {
				need = append(need, "(secret_env unset)")
			}
		case "github", "http-index":
			if strings.TrimSpace(s.TokenEnv) == "" {
				continue // optional auth
			}
			need = append(need, s.TokenEnv)
		default:
			continue
		}
		var missing []string
		for _, name := range need {
			if strings.HasPrefix(name, "(") {
				missing = append(missing, name)
				continue
			}
			if _, ok := LookupTokenEnv(name); !ok {
				missing = append(missing, name)
			}
		}
		out = append(out, CredentialStatus{
			Stage:   FormatStage(s),
			Ready:   len(missing) == 0,
			Missing: missing,
		})
	}
	return out
}

// FormatCredentialStatuses is a short text line for status output.
func FormatCredentialStatuses(st []CredentialStatus) string {
	if len(st) == 0 {
		return ""
	}
	parts := make([]string, 0, len(st))
	for _, s := range st {
		if s.Ready {
			parts = append(parts, s.Stage+"=ok")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=missing:%s", s.Stage, strings.Join(s.Missing, ",")))
	}
	return strings.Join(parts, " ")
}
