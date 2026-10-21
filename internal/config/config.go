// Copyright 2026 S3I Groups LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package config loads ~/.flinkit/config.yaml into named connection profiles.
// Credentials never live in the file as literals: option values may reference
// environment variables as ${VAR}, expanded when a profile is resolved.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Profile describes how to reach one platform.
type Profile struct {
	Source   string            `mapstructure:"source"`   // adapter name: oss|cp|cc|k8s|repo
	Platform string            `mapstructure:"platform"` // cc|cp|oss
	Endpoint string            `mapstructure:"endpoint"`
	Options  map[string]string `mapstructure:"options"`
}

// Config is the parsed config file: a set of named profiles plus the name of
// the default one.
type Config struct {
	Default  string             `mapstructure:"default"`
	Profiles map[string]Profile `mapstructure:"profiles"`
}

// DefaultPath returns the conventional config location, ~/.flinkit/config.yaml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: locating home directory: %w", err)
	}
	return filepath.Join(home, ".flinkit", "config.yaml"), nil
}

// Load reads and parses the config file at path.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: reading %s: %w", path, err)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("config: parsing %s: %w", path, err)
	}
	return &c, nil
}

// envConfig overrides the config file location, chiefly for tests and CI.
const envConfig = "FLINKIT_CONFIG"

// LoadDefault loads the config from FLINKIT_CONFIG if set, otherwise from
// DefaultPath, tolerating a missing default file by returning an empty Config
// so commands can run with no config present.
func LoadDefault() (*Config, error) {
	if p := os.Getenv(envConfig); p != "" {
		return Load(p)
	}
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	return Load(path)
}
