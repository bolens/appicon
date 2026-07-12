package resolve

import (
	"errors"
	"os"
	"strings"
)

// ErrAuthSkipped means a stage that requires credentials was skipped because
// token_env/secret_env was unset or the named environment variable was empty.
// It is a benign miss (pipeline continues).
var ErrAuthSkipped = errors.New("missing credentials")

// LookupTokenEnv reads and trims an environment variable by name.
// Returns ok=false when name is empty or the value is blank.
func LookupTokenEnv(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return "", false
	}
	return v, true
}

// RequireTokenEnv looks up token_env; returns ErrAuthSkipped when missing.
func RequireTokenEnv(tokenEnv string) (string, error) {
	tok, ok := LookupTokenEnv(tokenEnv)
	if !ok {
		return "", ErrAuthSkipped
	}
	return tok, nil
}

// RequireTokenAndSecret looks up both env names; returns ErrAuthSkipped when either is missing.
func RequireTokenAndSecret(tokenEnv, secretEnv string) (token, secret string, err error) {
	token, ok := LookupTokenEnv(tokenEnv)
	if !ok {
		return "", "", ErrAuthSkipped
	}
	secret, ok = LookupTokenEnv(secretEnv)
	if !ok {
		return "", "", ErrAuthSkipped
	}
	return token, secret, nil
}
