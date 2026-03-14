package dispatch

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Config represents a parsed .dispatch.yaml file.
type Config struct {
	Mode          string   `yaml:"mode"`
	VariablesPath string   `yaml:"path_to_tfvars"`
	IgnoreInputs  []string `yaml:"ignore_inputs"`
	Flow          []Step   `yaml:"flow"`
}

// Step defines a group of inputs shown together in the UI.
type Step struct {
	Name        string   `yaml:"step"`
	Description string   `yaml:"description"`
	Inputs      []string `yaml:"inputs"`
}

// SupportedModes lists the engine modes that Dispatch understands.
var SupportedModes = map[string]bool{
	"terraform": true,
}

// Parse reads a .dispatch.yaml file and returns the config.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse dispatch config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks that the config is well-formed.
func (c *Config) Validate() error {
	if c.Mode == "" {
		return fmt.Errorf("dispatch config: mode is required")
	}
	if !SupportedModes[c.Mode] {
		return fmt.Errorf("dispatch config: unsupported mode %q", c.Mode)
	}
	if c.VariablesPath == "" {
		return fmt.Errorf("dispatch config: path_to_tfvars is required")
	}
	if len(c.Flow) == 0 {
		return fmt.Errorf("dispatch config: at least one flow step is required")
	}
	for i, step := range c.Flow {
		if step.Name == "" {
			return fmt.Errorf("dispatch config: flow step %d: name is required", i)
		}
		if len(step.Inputs) == 0 {
			return fmt.Errorf("dispatch config: flow step %q: at least one input is required", step.Name)
		}
	}
	return nil
}

// IgnoreSet returns a set for fast lookup of ignored input names.
func (c *Config) IgnoreSet() map[string]bool {
	s := make(map[string]bool, len(c.IgnoreInputs))
	for _, name := range c.IgnoreInputs {
		s[name] = true
	}
	return s
}
