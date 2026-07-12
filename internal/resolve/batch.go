package resolve

import "context"

// BatchItem is one result from Batch.
type BatchItem struct {
	Query  string
	Result Result
	Err    error
}

// Batch resolves each query with the same options (sequential).
func Batch(ctx context.Context, queries []string, opts Options) []BatchItem {
	out := make([]BatchItem, 0, len(queries))
	for _, q := range queries {
		item := BatchItem{Query: q}
		res, err := Resolve(ctx, q, opts)
		item.Result = res
		item.Err = err
		out = append(out, item)
	}
	return out
}
