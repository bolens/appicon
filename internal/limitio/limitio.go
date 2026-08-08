// Package limitio reads bounded remote payloads without silently truncating them.
package limitio

import (
	"errors"
	"io"
)

// ErrTooLarge means a payload exceeded its configured byte limit.
var ErrTooLarge = errors.New("payload exceeds size limit")

// ReadAll reads at most max bytes and reports ErrTooLarge if more data exists.
func ReadAll(r io.Reader, max int64) ([]byte, error) {
	if max < 0 {
		return nil, ErrTooLarge
	}
	// A byte slice cannot exceed MaxInt. Avoid overflowing max+1 when callers
	// request the largest representable bound.
	maxInt := int64(^uint(0) >> 1)
	if max >= maxInt {
		return io.ReadAll(r)
	}
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, ErrTooLarge
	}
	return data, nil
}
