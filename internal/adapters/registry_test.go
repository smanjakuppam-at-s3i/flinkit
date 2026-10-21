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
	"reflect"
	"testing"
)

// stubAdapter is a minimal PlatformAdapter for registry tests. It embeds the
// interface so only the methods the tests touch need implementing; any other
// call would panic, which is fine because the tests never make one.
type stubAdapter struct {
	PlatformAdapter
	name string
}

func (s stubAdapter) Name() string { return s.name }

func TestRegistryNewAndNames(t *testing.T) {
	r := NewRegistry()
	r.Register("oss", func(cfg Config) (PlatformAdapter, error) {
		return stubAdapter{name: "oss"}, nil
	})
	r.Register("k8s", func(cfg Config) (PlatformAdapter, error) {
		return stubAdapter{name: "k8s"}, nil
	})

	a, err := r.New("oss", Config{})
	if err != nil {
		t.Fatalf("New(oss): %v", err)
	}
	if a.Name() != "oss" {
		t.Errorf("Name = %q, want oss", a.Name())
	}

	if got, want := r.Names(), []string{"k8s", "oss"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Names = %v, want %v (sorted)", got, want)
	}
}

func TestRegistryUnknown(t *testing.T) {
	r := NewRegistry()
	if _, err := r.New("missing", Config{}); err == nil {
		t.Fatal("New for unregistered adapter: expected error, got nil")
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	r := NewRegistry()
	f := func(cfg Config) (PlatformAdapter, error) { return stubAdapter{}, nil }
	r.Register("oss", f)

	defer func() {
		if recover() == nil {
			t.Fatal("duplicate Register: expected panic, got none")
		}
	}()
	r.Register("oss", f)
}
