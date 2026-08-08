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
		if err := ctx.Err(); err != nil {
			item.Err = err
		} else {
			item.Result, item.Err = Resolve(ctx, q, opts)
		}
		out = append(out, item)
	}
	return out
}
