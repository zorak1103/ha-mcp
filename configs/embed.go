// Package configs provides embedded configuration templates.
package configs

import (
	_ "embed"
)

// ConfigYAML contains the embedded YAML configuration template.
//
//go:embed config.example.yaml
var ConfigYAML []byte

// EnvExample contains the embedded environment variables template.
//
//go:embed .env.example
var EnvExample []byte
