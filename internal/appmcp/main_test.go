package appmcp_test

import (
	"os"
	"strings"
	"testing"
)

// Git hooks export repository context. Fixture Git commands must use their
// temporary repositories instead of the caller's index and branch.
func TestMain(m *testing.M) {
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "GIT_") {
			_ = os.Unsetenv(key)
		}
	}
	os.Exit(m.Run())
}
