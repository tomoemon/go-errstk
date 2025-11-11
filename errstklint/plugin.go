package errstklint

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("errstklint", New)
}

// ErrstklintPlugin is the plugin implementation for golangci-lint
type ErrstklintPlugin struct {
	settings Config
}

// New returns the plugin instance for golangci-lint plugin system.
// This function is called by golangci-lint when loading the plugin.
func New(settings any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Config](settings)
	if err != nil {
		return nil, err
	}

	return &ErrstklintPlugin{settings: s}, nil
}

// BuildAnalyzers returns the analyzers for this plugin
func (p *ErrstklintPlugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	// Apply configuration to the analyzer
	SetConfig(&p.settings)
	return []*analysis.Analyzer{Analyzer}, nil
}

// GetLoadMode returns the load mode for this plugin
func (p *ErrstklintPlugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
