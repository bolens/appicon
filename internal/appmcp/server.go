// Package appmcp exposes appicon resolve/prefetch/cache over MCP (stdio).
package appmcp

import (
	"context"
	"errors"

	"github.com/bolens/appicon/internal/resolve"
	"github.com/bolens/appicon/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Options configure resolve defaults for tools (tests inject XDG roots here).
type Options struct {
	Resolve resolve.Options
}

// NewServer builds an MCP server whose tools call internal/resolve.
func NewServer(opts Options) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "appicon",
		Version: version.Version,
	}, nil)
	h := &handlers{opts: opts.Resolve}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "resolve",
		Description: "Resolve a desktop/brand icon query to a local file path (mirrors appicon resolve --json).",
	}, h.resolve)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "prefetch",
		Description: "Warm the appicon cache for one or more queries (mirrors appicon prefetch).",
	}, h.prefetch)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "cache_stats",
		Description: "Report appicon cache directory size and file count (mirrors appicon cache stats).",
	}, h.cacheStats)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "cache_clear",
		Description: "Delete the appicon cache directory (destructive; mirrors appicon cache clear).",
	}, h.cacheClear)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "cache_prune",
		Description: "Prune stale/orphan cache entries (mirrors appicon cache prune).",
	}, h.cachePrune)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "version",
		Description: "Return the appicon version string (mirrors appicon version).",
	}, h.version)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "override_list",
		Description: "List query remaps from overrides.json (mirrors appicon override list --json).",
	}, h.overrideList)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "override_get",
		Description: "Get one override remap (mirrors appicon override get).",
	}, h.overrideGet)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "override_set",
		Description: "Set a query remap in overrides.json (mirrors appicon override set).",
	}, h.overrideSet)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "override_rm",
		Description: "Remove a query remap from overrides.json (mirrors appicon override rm).",
	}, h.overrideRm)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "sources_list",
		Description: "List effective resolve stage order (mirrors appicon sources list --json).",
	}, h.sourcesList)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "sources_get",
		Description: "Read sources.json (or describe defaults when missing).",
	}, h.sourcesGet)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "sources_set",
		Description: "Overwrite sources.json (destructive; validates stage types).",
	}, h.sourcesSet)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "pack_list",
		Description: "List configured pack stages (mirrors appicon pack list --json).",
	}, h.packList)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "pack_path",
		Description: "Print recommended packs root (mirrors appicon pack path).",
	}, h.packPath)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "pack_add",
		Description: "Register a local pack directory in sources.json (mirrors appicon pack add).",
	}, h.packAdd)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "pack_install",
		Description: "Clone a recipe or URL into packs and register it (destructive/network; mirrors appicon pack install). Pass recipe (simple-icons|dashboard-icons) or url (git / .tar.gz).",
	}, h.packInstall)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "pack_update",
		Description: "Refresh cloned recipe packs (network; mirrors appicon pack update).",
	}, h.packUpdate)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "pack_install_bundle",
		Description: "Install packs from a local .tar.gz bundle (mirrors pack install --from-bundle).",
	}, h.packInstallBundle)

	return s
}

// RunStdio serves MCP over stdin/stdout until the client disconnects.
func RunStdio(ctx context.Context, opts Options) error {
	return NewServer(opts).Run(ctx, &mcp.StdioTransport{})
}

type handlers struct {
	opts resolve.Options
}

type resolveInput struct {
	Query   string   `json:"query" jsonschema:"icon query: app id, WM class, .desktop id, display name, Steam appid, or file path"`
	Format  string   `json:"format,omitempty" jsonschema:"svg or png (default svg)"`
	Size    int      `json:"size,omitempty" jsonschema:"pixel size for png / XDG preference (default 48)"`
	Theme   string   `json:"theme,omitempty" jsonschema:"prefer dark or light variants when available"`
	Offline bool     `json:"offline,omitempty" jsonschema:"skip network; use cache, XDG, and local packs only"`
	Order   []string `json:"order,omitempty" jsonschema:"optional stage type order override (same as resolve --order)"`
}

type resolveOutput struct {
	Query  string  `json:"query"`
	Path   *string `json:"path"`
	Source string  `json:"source"`
	Theme  string  `json:"theme"`
	Format string  `json:"format"`
	Cached bool    `json:"cached"`
	Error  *string `json:"error"`
}

func (h *handlers) resolve(ctx context.Context, _ *mcp.CallToolRequest, in resolveInput) (*mcp.CallToolResult, resolveOutput, error) {
	opts := h.opts
	if in.Format != "" {
		opts.Format = in.Format
	}
	if in.Size > 0 {
		opts.Size = in.Size
	}
	if in.Theme != "" {
		opts.Theme = in.Theme
	}
	opts.Offline = opts.Offline || in.Offline
	if len(in.Order) > 0 {
		opts.Order = in.Order
	}

	out := resolveOutput{
		Query:  in.Query,
		Theme:  opts.Theme,
		Format: opts.Format,
	}
	if out.Format == "" {
		out.Format = "svg"
	}

	res, err := resolve.Resolve(ctx, in.Query, opts)
	if err != nil {
		msg := err.Error()
		out.Error = &msg
		// Misses are expected outcomes for callers (glyph fallback); other errors too.
		if !errors.Is(err, resolve.ErrNotFound) {
			return &mcp.CallToolResult{IsError: true}, out, nil
		}
		return nil, out, nil
	}
	path := res.Path
	out.Path = &path
	out.Source = res.Source
	out.Theme = res.Theme
	out.Format = res.Format
	out.Cached = res.Cached
	return nil, out, nil
}

type prefetchInput struct {
	Queries []string `json:"queries" jsonschema:"one or more icon queries to warm in the cache"`
	Format  string   `json:"format,omitempty" jsonschema:"svg or png (default svg)"`
	Size    int      `json:"size,omitempty" jsonschema:"pixel size preference (default 48)"`
	Theme   string   `json:"theme,omitempty" jsonschema:"prefer dark or light variants when available"`
	Offline bool     `json:"offline,omitempty" jsonschema:"skip network while prefetching"`
	Order   []string `json:"order,omitempty" jsonschema:"optional stage type order override (same as resolve --order)"`
}

type prefetchItem struct {
	Query  string  `json:"query"`
	Path   *string `json:"path"`
	Source string  `json:"source,omitempty"`
	Error  *string `json:"error"`
}

type prefetchOutput struct {
	Results []prefetchItem `json:"results"`
}

func (h *handlers) prefetch(ctx context.Context, _ *mcp.CallToolRequest, in prefetchInput) (*mcp.CallToolResult, prefetchOutput, error) {
	opts := h.opts
	if in.Format != "" {
		opts.Format = in.Format
	}
	if in.Size > 0 {
		opts.Size = in.Size
	}
	if in.Theme != "" {
		opts.Theme = in.Theme
	}
	opts.Offline = opts.Offline || in.Offline
	if len(in.Order) > 0 {
		opts.Order = in.Order
	}

	out := prefetchOutput{Results: make([]prefetchItem, 0, len(in.Queries))}
	for _, q := range in.Queries {
		item := prefetchItem{Query: q}
		res, err := resolve.Resolve(ctx, q, opts)
		if err != nil {
			msg := err.Error()
			item.Error = &msg
		} else {
			path := res.Path
			item.Path = &path
			item.Source = res.Source
		}
		out.Results = append(out.Results, item)
	}
	return nil, out, nil
}

type emptyInput struct{}

type cacheStatsOutput struct {
	Dir   string `json:"dir"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

func (h *handlers) cacheStats(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, cacheStatsOutput, error) {
	st, err := resolve.CacheStats()
	if err != nil {
		return nil, cacheStatsOutput{}, err
	}
	return nil, cacheStatsOutput{Dir: st.Dir, Files: st.Files, Bytes: st.Bytes}, nil
}

type cacheClearOutput struct {
	Cleared bool `json:"cleared"`
}

func (h *handlers) cacheClear(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, cacheClearOutput, error) {
	if err := resolve.ClearCache(); err != nil {
		return nil, cacheClearOutput{}, err
	}
	return nil, cacheClearOutput{Cleared: true}, nil
}

type cachePruneOutput struct {
	RemovedFiles int   `json:"removed_files"`
	RemovedBytes int64 `json:"removed_bytes"`
}

func (h *handlers) cachePrune(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, cachePruneOutput, error) {
	st, err := resolve.PruneCache()
	if err != nil {
		return nil, cachePruneOutput{}, err
	}
	return nil, cachePruneOutput{RemovedFiles: st.RemovedFiles, RemovedBytes: st.RemovedBytes}, nil
}

type versionOutput struct {
	Version string `json:"version"`
}

func (h *handlers) version(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, versionOutput, error) {
	return nil, versionOutput{Version: version.Version}, nil
}

type overrideListOutput struct {
	Overrides map[string]string `json:"overrides"`
	Path      string            `json:"path"`
}

func (h *handlers) overrideList(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, overrideListOutput, error) {
	cfg := h.opts.ConfigDir
	m, err := resolve.ListOverrides(cfg)
	if err != nil {
		return nil, overrideListOutput{}, err
	}
	return nil, overrideListOutput{Overrides: m, Path: resolve.OverridesPath(cfg)}, nil
}

type overrideKeyInput struct {
	Query string `json:"query" jsonschema:"override key (app id / query to remap)"`
}

type overrideGetOutput struct {
	Query  string  `json:"query"`
	Target *string `json:"target"`
	Error  *string `json:"error"`
}

func (h *handlers) overrideGet(_ context.Context, _ *mcp.CallToolRequest, in overrideKeyInput) (*mcp.CallToolResult, overrideGetOutput, error) {
	out := overrideGetOutput{Query: in.Query}
	v, err := resolve.GetOverride(h.opts.ConfigDir, in.Query)
	if err != nil {
		msg := err.Error()
		out.Error = &msg
		if !errors.Is(err, resolve.ErrOverrideNotFound) {
			return &mcp.CallToolResult{IsError: true}, out, nil
		}
		return nil, out, nil
	}
	out.Target = &v
	return nil, out, nil
}

type overrideSetInput struct {
	Query  string `json:"query" jsonschema:"override key (app id / query to remap)"`
	Target string `json:"target" jsonschema:"canonical query or icon id to resolve instead"`
}

type overrideSetOutput struct {
	Query  string `json:"query"`
	Target string `json:"target"`
	OK     bool   `json:"ok"`
}

func (h *handlers) overrideSet(_ context.Context, _ *mcp.CallToolRequest, in overrideSetInput) (*mcp.CallToolResult, overrideSetOutput, error) {
	if err := resolve.SetOverride(h.opts.ConfigDir, in.Query, in.Target); err != nil {
		return &mcp.CallToolResult{IsError: true}, overrideSetOutput{}, err
	}
	return nil, overrideSetOutput{Query: in.Query, Target: in.Target, OK: true}, nil
}

type overrideRmOutput struct {
	Query string `json:"query"`
	OK    bool   `json:"ok"`
}

func (h *handlers) overrideRm(_ context.Context, _ *mcp.CallToolRequest, in overrideKeyInput) (*mcp.CallToolResult, overrideRmOutput, error) {
	if err := resolve.RemoveOverride(h.opts.ConfigDir, in.Query); err != nil {
		if errors.Is(err, resolve.ErrOverrideNotFound) {
			return nil, overrideRmOutput{Query: in.Query, OK: false}, nil
		}
		return &mcp.CallToolResult{IsError: true}, overrideRmOutput{Query: in.Query}, err
	}
	return nil, overrideRmOutput{Query: in.Query, OK: true}, nil
}
