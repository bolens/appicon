package appmcp

import (
	"context"
	"errors"
	"strings"

	"github.com/bolens/appicon/internal/packs"
	"github.com/bolens/appicon/internal/resolve"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type sourcesListOutput struct {
	Path      string          `json:"path"`
	Effective []string        `json:"effective"`
	File      *bool           `json:"file"`
	Overrides *bool           `json:"overrides"`
	XDG       *bool           `json:"xdg"`
	Sources   []resolve.Stage `json:"sources"`
}

func (h *handlers) sourcesList(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, sourcesListOutput, error) {
	stages, cfg, err := resolve.LoadEffectiveStages(h.opts.ConfigDir, nil)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, sourcesListOutput{}, err
	}
	return nil, sourcesListOutput{
		Path:      resolve.SourcesPath(h.opts.ConfigDir),
		Effective: resolve.FormatStages(stages),
		File:      cfg.File,
		Overrides: cfg.Overrides,
		XDG:       cfg.XDG,
		Sources:   cfg.Sources,
	}, nil
}

type sourcesGetOutput struct {
	Path     string                `json:"path"`
	Exists   bool                  `json:"exists"`
	Defaults bool                  `json:"defaults"`
	Config   resolve.SourcesConfig `json:"config"`
}

func (h *handlers) sourcesGet(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, sourcesGetOutput, error) {
	cfg, err := resolve.LoadSourcesConfig(h.opts.ConfigDir)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, sourcesGetOutput{}, err
	}
	path := resolve.SourcesPath(h.opts.ConfigDir)
	exists, err := fileExists(path)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, sourcesGetOutput{}, err
	}
	return nil, sourcesGetOutput{
		Path:     path,
		Exists:   exists,
		Defaults: !exists || len(cfg.Sources) == 0,
		Config:   cfg,
	}, nil
}

type sourcesSetInput struct {
	Sources   []resolve.Stage `json:"sources" jsonschema:"ordered resolve stages"`
	File      *bool           `json:"file,omitempty" jsonschema:"false disables file stage"`
	Overrides *bool           `json:"overrides,omitempty" jsonschema:"false disables overrides stage"`
	XDG       *bool           `json:"xdg,omitempty" jsonschema:"false disables xdg stage"`
}

type sourcesSetOutput struct {
	Path string `json:"path"`
	OK   bool   `json:"ok"`
}

func (h *handlers) sourcesSet(_ context.Context, _ *mcp.CallToolRequest, in sourcesSetInput) (*mcp.CallToolResult, sourcesSetOutput, error) {
	cfg := resolve.SourcesConfig{
		Sources:   in.Sources,
		File:      in.File,
		Overrides: in.Overrides,
		XDG:       in.XDG,
	}
	if err := resolve.ValidateStages(cfg.Sources); err != nil {
		return &mcp.CallToolResult{IsError: true}, sourcesSetOutput{}, err
	}
	if err := resolve.WriteSourcesConfig(h.opts.ConfigDir, cfg); err != nil {
		return &mcp.CallToolResult{IsError: true}, sourcesSetOutput{}, err
	}
	return nil, sourcesSetOutput{Path: resolve.SourcesPath(h.opts.ConfigDir), OK: true}, nil
}

type packListOutput struct {
	Root  string       `json:"root"`
	Packs []packs.Info `json:"packs"`
}

func (h *handlers) packList(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, packListOutput, error) {
	list, err := packs.List(h.opts.ConfigDir)
	if err != nil {
		return &mcp.CallToolResult{IsError: true}, packListOutput{}, err
	}
	return nil, packListOutput{Root: packs.Root(), Packs: list}, nil
}

type packPathOutput struct {
	Path string `json:"path"`
}

func (h *handlers) packPath(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, packPathOutput, error) {
	return nil, packPathOutput{Path: packs.Root()}, nil
}

type packAddInput struct {
	Name string `json:"name" jsonschema:"pack name"`
	Path string `json:"path" jsonschema:"directory containing icons"`
}

type packOKOutput struct {
	OK bool `json:"ok"`
}

func (h *handlers) packAdd(_ context.Context, _ *mcp.CallToolRequest, in packAddInput) (*mcp.CallToolResult, packOKOutput, error) {
	if err := packs.Add(h.opts.ConfigDir, in.Name, in.Path); err != nil {
		return &mcp.CallToolResult{IsError: true}, packOKOutput{}, err
	}
	return nil, packOKOutput{OK: true}, nil
}

type packInstallInput struct {
	Recipe  string `json:"recipe,omitempty" jsonschema:"recipe name, or omit when url is set"`
	URL     string `json:"url,omitempty" jsonschema:"git URL or https .tar.gz URL"`
	Path    string `json:"path,omitempty" jsonschema:"optional clone/extract destination"`
	Name    string `json:"name,omitempty" jsonschema:"pack name override"`
	Subdir  string `json:"subdir,omitempty" jsonschema:"subdir inside clone used as pack root"`
	Ref     string `json:"ref,omitempty" jsonschema:"git branch or tag"`
	Offline bool   `json:"offline,omitempty" jsonschema:"if true, refuse network install"`
}

func (h *handlers) packInstall(_ context.Context, _ *mcp.CallToolRequest, in packInstallInput) (*mcp.CallToolResult, packOKOutput, error) {
	target := strings.TrimSpace(in.URL)
	if target == "" {
		target = strings.TrimSpace(in.Recipe)
	}
	if target == "" {
		return &mcp.CallToolResult{IsError: true}, packOKOutput{}, errors.New("pack_install requires recipe or url")
	}
	if err := packs.Install(h.opts.ConfigDir, packs.InstallOpts{
		Target:  target,
		Dest:    in.Path,
		Name:    in.Name,
		Subdir:  in.Subdir,
		Ref:     in.Ref,
		Offline: in.Offline,
	}); err != nil {
		return &mcp.CallToolResult{IsError: true}, packOKOutput{}, err
	}
	return nil, packOKOutput{OK: true}, nil
}

type packUpdateInput struct {
	Recipe  string `json:"recipe,omitempty" jsonschema:"optional recipe filter"`
	Offline bool   `json:"offline,omitempty"`
}

func (h *handlers) packUpdate(_ context.Context, _ *mcp.CallToolRequest, in packUpdateInput) (*mcp.CallToolResult, packOKOutput, error) {
	if err := packs.Update(h.opts.ConfigDir, in.Recipe, in.Offline); err != nil {
		return &mcp.CallToolResult{IsError: true}, packOKOutput{}, err
	}
	return nil, packOKOutput{OK: true}, nil
}

type packBundleInput struct {
	Bundle string `json:"bundle" jsonschema:"path to appicon-packs-bundle.tar.gz on this machine"`
}

func (h *handlers) packInstallBundle(_ context.Context, _ *mcp.CallToolRequest, in packBundleInput) (*mcp.CallToolResult, packOKOutput, error) {
	if err := packs.InstallBundle(h.opts.ConfigDir, in.Bundle); err != nil {
		return &mcp.CallToolResult{IsError: true}, packOKOutput{}, err
	}
	return nil, packOKOutput{OK: true}, nil
}
