package errstklint

import (
	"encoding/json"

	"golang.org/x/tools/go/analysis"
)

// New returns the analyzer for golangci-lint plugin system.
// This function is called by golangci-lint when loading the plugin.
// The conf parameter contains the configuration from .golangci.yml
func New(conf any) ([]*analysis.Analyzer, error) {
	// Parse configuration if provided
	if conf != nil {
		cfg := &Config{}

		// golangci-lint passes configuration as map[string]any
		if confMap, ok := conf.(map[string]any); ok {
			// Try to parse exclude patterns
			if excludeVal, ok := confMap["exclude"]; ok {
				switch v := excludeVal.(type) {
				case []any:
					// Convert []any to []string
					cfg.Exclude = make([]string, 0, len(v))
					for _, item := range v {
						if str, ok := item.(string); ok {
							cfg.Exclude = append(cfg.Exclude, str)
						}
					}
				case []string:
					cfg.Exclude = v
				case string:
					// Single pattern as string
					cfg.Exclude = []string{v}
				}
			}
		} else {
			// Fallback: try JSON unmarshaling
			data, err := json.Marshal(conf)
			if err == nil {
				_ = json.Unmarshal(data, cfg)
			}
		}

		SetConfig(cfg)
	}

	return []*analysis.Analyzer{Analyzer}, nil
}
