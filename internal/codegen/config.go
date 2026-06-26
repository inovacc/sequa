package codegen

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the sequa.yaml schema — the codegen counterpart to sqlc.yaml.
type Config struct {
	Version string     `yaml:"version"`
	SQL     []SQLBlock `yaml:"sql"`
}

// SQLBlock describes one codegen unit: where the schema (migrations) and
// queries live, and how to emit Go.
type SQLBlock struct {
	Engine  string `yaml:"engine"`  // postgresql (default/only in M3)
	Schema  string `yaml:"schema"`  // path to the migrations directory
	Queries string `yaml:"queries"` // path to a .sql file or dir of annotated queries (optional)
	Gen     Gen    `yaml:"gen"`
}

// Gen holds the per-language generation settings.
type Gen struct {
	Go GenGo `yaml:"go"`
}

// GenGo configures Go output.
type GenGo struct {
	Package string `yaml:"package"`
	Out     string `yaml:"out"`
}

// LoadConfig reads and validates a sequa.yaml file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.SQL) == 0 {
		return fmt.Errorf("config: at least one 'sql' block is required")
	}
	for i, b := range c.SQL {
		if b.Schema == "" {
			return fmt.Errorf("config: sql[%d].schema is required", i)
		}
		if b.Gen.Go.Package == "" {
			return fmt.Errorf("config: sql[%d].gen.go.package is required", i)
		}
		if b.Gen.Go.Out == "" {
			return fmt.Errorf("config: sql[%d].gen.go.out is required", i)
		}
	}
	return nil
}
