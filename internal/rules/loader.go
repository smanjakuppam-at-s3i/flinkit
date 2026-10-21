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

package rules

import (
	"fmt"
	"io/fs"
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

// ruleFile is the on-disk shape: a YAML document with a top-level `rules:` list.
type ruleFile struct {
	Rules []Rule `yaml:"rules"`
}

// ParseRules parses a YAML document containing a `rules:` list.
func ParseRules(data []byte) ([]Rule, error) {
	var rf ruleFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, err
	}
	return rf.Rules, nil
}

// LoadFile loads the rules from one YAML file.
func LoadFile(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rules: reading %s: %w", path, err)
	}
	rules, err := ParseRules(data)
	if err != nil {
		return nil, fmt.Errorf("rules: parsing %s: %w", path, err)
	}
	return rules, nil
}

// LoadFS walks an fs.FS below root for *.yaml and *.yml rule files and returns
// a validated, duplicate-free RuleSet. It backs both on-disk loading and the
// embedded built-in rules.
func LoadFS(fsys fs.FS, root string) (*RuleSet, error) {
	var all []Rule
	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch path.Ext(p) {
		case ".yaml", ".yml":
			data, err := fs.ReadFile(fsys, p)
			if err != nil {
				return fmt.Errorf("rules: reading %s: %w", p, err)
			}
			rules, err := ParseRules(data)
			if err != nil {
				return fmt.Errorf("rules: parsing %s: %w", p, err)
			}
			all = append(all, rules...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return NewRuleSet(all)
}

// LoadDir walks a directory on disk for rule files.
func LoadDir(dir string) (*RuleSet, error) {
	return LoadFS(os.DirFS(dir), ".")
}
