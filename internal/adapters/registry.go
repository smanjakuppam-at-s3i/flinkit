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

package adapters

import (
	"fmt"
	"sort"
	"sync"
)

// Registry maps adapter names to their factories. Adapters register themselves
// (typically in an init function); commands look an adapter up by --source.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register adds a factory under name. It panics on an empty name, a nil
// factory, or a duplicate registration, so wiring mistakes fail loudly at
// startup rather than silently shadowing an adapter.
func (r *Registry) Register(name string, f Factory) {
	if name == "" {
		panic("adapters: Register called with an empty name")
	}
	if f == nil {
		panic("adapters: Register called with a nil factory for " + name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.factories[name]; dup {
		panic("adapters: duplicate Register for " + name)
	}
	r.factories[name] = f
}

// New constructs the adapter registered under name.
func (r *Registry) New(name string, cfg Config) (PlatformAdapter, error) {
	r.mu.RLock()
	f, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("adapters: no adapter registered as %q", name)
	}
	return f(cfg)
}

// Names returns the registered adapter names, sorted.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Default is the process-wide registry that adapters register into.
var Default = NewRegistry()

// Register adds a factory to the default registry.
func Register(name string, f Factory) { Default.Register(name, f) }
