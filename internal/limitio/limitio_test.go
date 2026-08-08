package limitio_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bolens/appicon/internal/limitio"
)

func TestReadAllBoundary(t *testing.T) {
	data, err := limitio.ReadAll(strings.NewReader("1234"), 4)
	if err != nil || string(data) != "1234" {
		t.Fatalf("data=%q err=%v", data, err)
	}
	_, err = limitio.ReadAll(strings.NewReader("12345"), 4)
	if !errors.Is(err, limitio.ErrTooLarge) {
		t.Fatalf("err=%v", err)
	}
}

func TestReadAllRejectsNegativeLimit(t *testing.T) {
	_, err := limitio.ReadAll(strings.NewReader(""), -1)
	if !errors.Is(err, limitio.ErrTooLarge) {
		t.Fatalf("err=%v", err)
	}
}
