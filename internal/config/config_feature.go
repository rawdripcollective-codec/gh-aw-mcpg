// Package config provides configuration loading and parsing.
// This file defines the feature registration framework for modular config handling.
package config

// FeatureConfig represents a modular configuration feature.
// Each feature defines its own config processing in a separate file.
type FeatureConfig interface {
	// FeatureName returns the name of the feature for logging
	FeatureName() string
}

// DefaultsSetter sets default values for a Config.
// Features register these to apply their defaults during loading.
type DefaultsSetter func(cfg *Config)

// StdinConverter converts stdin-specific config to internal Config.
// Features register these to handle their stdin config fields.
type StdinConverter func(cfg *Config, stdinCfg *StdinConfig)

var (
	// defaultsSetters holds all registered default setters
	defaultsSetters []DefaultsSetter

	// stdinConverters holds all registered stdin converters
	stdinConverters []StdinConverter
)

// RegisterDefaults registers a function that sets defaults for a config feature.
// Called during init() in feature-specific config files.
func RegisterDefaults(fn DefaultsSetter) {
	defaultsSetters = append(defaultsSetters, fn)
}

// RegisterStdinConverter registers a function that converts stdin config for a feature.
// Called during init() in feature-specific config files.
func RegisterStdinConverter(fn StdinConverter) {
	stdinConverters = append(stdinConverters, fn)
}

// applyDefaults applies all registered default setters to a Config.
func applyDefaults(cfg *Config) {
	for _, setter := range defaultsSetters {
		setter(cfg)
	}
}

// applyStdinConverters applies all registered stdin converters.
func applyStdinConverters(cfg *Config, stdinCfg *StdinConfig) {
	for _, converter := range stdinConverters {
		converter(cfg, stdinCfg)
	}
}
