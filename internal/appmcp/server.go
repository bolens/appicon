// Package appmcp exposes appicon resolve/prefetch/cache over MCP (stdio).
package appmcp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/bolens/appicon/internal/daemon"
	"github.com/bolens/appicon/internal/packs"
	"github.com/bolens/appicon/internal/resolve"
	"github.com/bolens/appicon/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Options configure resolve defaults for tools (tests inject XDG roots here).
type Options struct {
	Resolve resolve.Options
}

const serverInstructions = `appicon resolves desktop/brand icon queries to local file paths.

Rules for agents:
- Prefer these MCP tools over shelling the CLI when connected.
- A resolve miss (path null, error set, IsError false) is a supported outcome — callers keep glyphs. Do not treat miss as a hard failure.
- Prefer override_set for long-tail remaps; do not invent speculative aliases or embed SVGL/CDN URLs in other repos.
- CDN stages (simple-icons, dashboard-icons) are opt-in network lookups; pack_install clones local trees — they are not the same.
- Hot paths should prefetch then resolve with offline=true.
- Use sources_* / pack_* / status to inspect and configure; never download icons yourself.
`

// NewServer builds an MCP server whose tools call internal/resolve.
func NewServer(opts Options) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "appicon",
		Version: version.Version,
	}, &mcp.ServerOptions{
		Instructions: serverInstructions,
	})
	h := &handlers{opts: opts.Resolve}

	mcp.AddTool(s, &mcp.Tool{
		Name: "resolve",
		Description: "Resolve desktop/brand icon queries to local file paths (mirrors appicon resolve --json). " +
			"Pass query for one result, or queries for a batch ({results:[…]}). " +
			"Miss returns path:null with error set and IsError=false (supported). " +
			"Set explain=true for tried stages + hint. Prefer override_set for remaps; do not invent CDN/SVGL URLs.",
	}, h.resolve)
	mcp.AddTool(s, &mcp.Tool{
		Name: "prefetch",
		Description: "Warm the appicon cache for one or more queries (mirrors appicon prefetch). " +
			"Supports order, offline, theme, and from_desktop; use before offline resolve on hot paths.",
	}, h.prefetch)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "status",
		Description: "Report config/cache/pack paths, effective order, daemon socket, and helper tools (mirrors appicon status --json).",
	}, h.status)
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
		Description: "Get one override remap (mirrors appicon override get). Missing key returns target:null without IsError.",
	}, h.overrideGet)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "override_set",
		Description: "Set a query remap in overrides.json (mirrors appicon override set). Prefer this for long-tail WM classes / broken .desktop ids.",
	}, h.overrideSet)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "override_rm",
		Description: "Remove a query remap from overrides.json (mirrors appicon override rm).",
	}, h.overrideRm)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "override_suggest",
		Description: "Suggest override targets for a miss query or recent misses (mirrors appicon override suggest).",
	}, h.overrideSuggest)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "sources_list",
		Description: "List effective resolve stage order (mirrors appicon sources list --json).",
	}, h.sourcesList)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "sources_get",
		Description: "Read sources.json (or describe defaults when missing). Mirrors appicon sources get --json.",
	}, h.sourcesGet)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "sources_set",
		Description: "Overwrite sources.json (destructive; validates stage types). Mirrors appicon sources set.",
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
		Name: "pack_install",
		Description: "Clone a recipe or URL into packs and register it (network; mirrors appicon pack install). " +
			"Pass recipe (simple-icons|dashboard-icons) or url (git / .tar.gz). Local packs ≠ CDN stages.",
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
	Query   string   `json:"query,omitempty" jsonschema:"single icon query (app id, WM class, .desktop id, display name, Steam appid, or file path)"`
	Queries []string `json:"queries,omitempty" jsonschema:"multiple icon queries for batch resolve"`
	Format  string   `json:"format,omitempty" jsonschema:"svg or png (default svg)"`
	Size    int      `json:"size,omitempty" jsonschema:"pixel size for png / XDG preference (default 48)"`
	Theme   string   `json:"theme,omitempty" jsonschema:"prefer dark or light variants when available"`
	Offline bool     `json:"offline,omitempty" jsonschema:"skip network; use cache, XDG, and local packs only"`
	Order   []string `json:"order,omitempty" jsonschema:"optional stage type order override (same as resolve --order)"`
	Explain bool     `json:"explain,omitempty" jsonschema:"include tried stages and miss hint"`
}

type resolveOutput struct {
	Query   string              `json:"query,omitempty"`
	Path    *string             `json:"path"`
	Source  string              `json:"source,omitempty"`
	Theme   string              `json:"theme,omitempty"`
	Format  string              `json:"format,omitempty"`
	Cached  bool                `json:"cached,omitempty"`
	Error   *string             `json:"error"`
	Tried   []string            `json:"tried,omitempty"`
	Hint    string              `json:"hint,omitempty"`
	Results []resolveResultItem `json:"results,omitempty"`
}

type resolveResultItem struct {
	Query  string   `json:"query"`
	Path   *string  `json:"path"`
	Source string   `json:"source,omitempty"`
	Theme  string   `json:"theme,omitempty"`
	Format string   `json:"format,omitempty"`
	Cached bool     `json:"cached,omitempty"`
	Error  *string  `json:"error"`
	Tried  []string `json:"tried,omitempty"`
	Hint   string   `json:"hint,omitempty"`
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

	queries := append([]string(nil), in.Queries...)
	if in.Query != "" {
		queries = append([]string{in.Query}, queries...)
	}
	if len(queries) == 0 {
		msg := "resolve requires query or queries"
		return &mcp.CallToolResult{IsError: true}, resolveOutput{Error: &msg}, nil
	}

	if len(queries) == 1 {
		item, hard := h.resolveOne(ctx, queries[0], opts, in.Explain)
		out := resolveOutput{
			Query:  item.Query,
			Path:   item.Path,
			Source: item.Source,
			Theme:  item.Theme,
			Format: item.Format,
			Cached: item.Cached,
			Error:  item.Error,
			Tried:  item.Tried,
			Hint:   item.Hint,
		}
		if hard {
			return &mcp.CallToolResult{IsError: true}, out, nil
		}
		return nil, out, nil
	}

	out := resolveOutput{Results: make([]resolveResultItem, 0, len(queries)), Path: nil, Error: nil}
	hardErr := false
	for _, q := range queries {
		item, hard := h.resolveOne(ctx, q, opts, in.Explain)
		out.Results = append(out.Results, item)
		if hard {
			hardErr = true
		}
	}
	if hardErr {
		return &mcp.CallToolResult{IsError: true}, out, nil
	}
	return nil, out, nil
}

func (h *handlers) resolveOne(ctx context.Context, query string, opts resolve.Options, explain bool) (resolveResultItem, bool) {
	out := resolveResultItem{
		Query:  query,
		Theme:  opts.Theme,
		Format: opts.Format,
	}
	if out.Format == "" {
		out.Format = "svg"
	}
	res, err := resolve.Resolve(ctx, query, opts)
	if err != nil {
		msg := err.Error()
		out.Error = &msg
		if explain {
			out.Tried = res.Tried
			if errors.Is(err, resolve.ErrNotFound) {
				out.Hint = res.Hint
				if out.Hint == "" {
					out.Hint = resolve.MissHint(opts.ConfigDir, opts.Order)
				}
			}
		}
		return out, !errors.Is(err, resolve.ErrNotFound)
	}
	path := res.Path
	out.Path = &path
	out.Source = res.Source
	out.Theme = res.Theme
	out.Format = res.Format
	out.Cached = res.Cached
	if explain {
		out.Tried = res.Tried
	}
	return out, false
}

type prefetchInput struct {
	Queries     []string `json:"queries,omitempty" jsonschema:"one or more icon queries to warm in the cache"`
	FromDesktop bool     `json:"from_desktop,omitempty" jsonschema:"include queries derived from installed .desktop files"`
	Format      string   `json:"format,omitempty" jsonschema:"svg or png (default svg)"`
	Size        int      `json:"size,omitempty" jsonschema:"pixel size preference (default 48)"`
	Theme       string   `json:"theme,omitempty" jsonschema:"prefer dark or light variants when available"`
	Offline     bool     `json:"offline,omitempty" jsonschema:"skip network while prefetching"`
	Order       []string `json:"order,omitempty" jsonschema:"optional stage type order override (same as resolve --order)"`
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

	queries := append([]string(nil), in.Queries...)
	if in.FromDesktop {
		queries = append(queries, resolve.DesktopPrefetchQueries(opts)...)
	}
	seen := map[string]struct{}{}
	uniq := make([]string, 0, len(queries))
	for _, q := range queries {
		k := strings.ToLower(strings.TrimSpace(q))
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		uniq = append(uniq, strings.TrimSpace(q))
	}

	out := prefetchOutput{Results: make([]prefetchItem, 0, len(uniq))}
	for _, q := range uniq {
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

type statusToolInfo struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
	OK   bool   `json:"ok"`
}

type statusOutput struct {
	Version            string           `json:"version"`
	ConfigDir          string           `json:"config_dir"`
	SourcesPath        string           `json:"sources_path"`
	OverridesPath      string           `json:"overrides_path"`
	CacheDir           string           `json:"cache_dir"`
	CacheFiles         int              `json:"cache_files"`
	CacheBytes         int64            `json:"cache_bytes"`
	PacksRoot          string           `json:"packs_root"`
	Packs              int              `json:"packs"`
	Order              []string         `json:"order"`
	DaemonSocket       string           `json:"daemon_socket"`
	DaemonSocketExists bool             `json:"daemon_socket_exists"`
	Tools              []statusToolInfo `json:"tools"`
}

func (h *handlers) status(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, statusOutput, error) {
	cfg := h.opts.ConfigDir
	stages, _, err := resolve.LoadEffectiveStages(cfg, nil)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, statusOutput{}, err
	}
	cache, err := resolve.CacheStats()
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, statusOutput{}, err
	}
	packList, err := packs.List(cfg)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, statusOutput{}, err
	}
	sock := daemon.SocketPath()
	_, sockErr := os.Stat(sock)
	tools := make([]statusToolInfo, 0, 3)
	for _, name := range []string{"resvg", "rsvg-convert", "git"} {
		p, lookErr := exec.LookPath(name)
		tools = append(tools, statusToolInfo{Name: name, Path: p, OK: lookErr == nil})
	}
	return nil, statusOutput{
		Version:            version.Version,
		ConfigDir:          resolve.ConfigDir(),
		SourcesPath:        resolve.SourcesPath(cfg),
		OverridesPath:      resolve.OverridesPath(cfg),
		CacheDir:           cache.Dir,
		CacheFiles:         cache.Files,
		CacheBytes:         cache.Bytes,
		PacksRoot:          packs.Root(),
		Packs:              len(packList),
		Order:              resolve.FormatStages(stages),
		DaemonSocket:       sock,
		DaemonSocketExists: sockErr == nil,
		Tools:              tools,
	}, nil
}

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

type overrideSuggestInput struct {
	Query      string `json:"query,omitempty" jsonschema:"query to suggest remaps for"`
	FromMisses bool   `json:"from_misses,omitempty" jsonschema:"suggest for recent miss queries instead of a single query"`
	Apply      bool   `json:"apply,omitempty" jsonschema:"apply the first candidate via override_set"`
}

type overrideSuggestOutput struct {
	Suggestions []resolve.Suggestion `json:"suggestions,omitempty"`
	Suggestion  *resolve.Suggestion  `json:"suggestion,omitempty"`
	Applied     *string              `json:"applied,omitempty"`
}

func (h *handlers) overrideSuggest(_ context.Context, _ *mcp.CallToolRequest, in overrideSuggestInput) (*mcp.CallToolResult, overrideSuggestOutput, error) {
	opts := h.opts
	if opts.Format == "" {
		opts.Format = "svg"
	}
	if opts.Size <= 0 {
		opts.Size = 48
	}
	if in.FromMisses {
		list, err := resolve.SuggestFromMisses(h.opts.ConfigDir, opts, 20)
		if err != nil {
			return &mcp.CallToolResult{IsError: true}, overrideSuggestOutput{}, err
		}
		return nil, overrideSuggestOutput{Suggestions: list}, nil
	}
	if strings.TrimSpace(in.Query) == "" {
		msg := "override_suggest requires query or from_misses"
		return &mcp.CallToolResult{IsError: true}, overrideSuggestOutput{}, errors.New(msg)
	}
	s, err := resolve.SuggestOverride(h.opts.ConfigDir, in.Query, opts)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, overrideSuggestOutput{}, err
	}
	out := overrideSuggestOutput{Suggestion: &s}
	if in.Apply && len(s.Candidates) > 0 {
		if err := resolve.SetOverride(h.opts.ConfigDir, s.Query, s.Candidates[0]); err != nil {
			return &mcp.CallToolResult{IsError: true}, out, err
		}
		out.Applied = &s.Candidates[0]
	}
	return nil, out, nil
}
