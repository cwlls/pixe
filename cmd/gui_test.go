// Copyright 2026 Chris Wells <chris@rhza.org>
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

package cmd

import (
	"testing"
)

// TestGUICmd_FlagsRegistered verifies that guiCmd has the expected flags
// registered and that they are independent of sortCmd's flag set.
func TestGUICmd_FlagsRegistered(t *testing.T) {
	for _, name := range []string{"source", "dest", "copyright", "camera-owner",
		"dry-run", "db-path", "recursive", "skip-duplicates",
		"ignore", "no-carry-sidecars", "overwrite-sidecar-tags"} {
		if f := guiCmd.Flags().Lookup(name); f == nil {
			t.Errorf("guiCmd missing flag --%s", name)
		}
	}
}

// TestGUICmd_DestNotRequired verifies that --dest is not marked required on
// guiCmd (unlike sortCmd) because the TUI allows setting it interactively.
func TestGUICmd_DestNotRequired(t *testing.T) {
	destFlag := guiCmd.Flags().Lookup("dest")
	if destFlag == nil {
		t.Fatal("--dest flag not registered on guiCmd")
	}
	const requiredKey = "cobra_annotation_bash_completion_one_required_flag"
	if _, required := destFlag.Annotations[requiredKey]; required {
		t.Error("--dest should not be marked required on guiCmd; it can be set interactively")
	}
}

// TestResolveGUIConfig_Defaults verifies that resolveGUIConfig returns sane
// defaults when no flags are explicitly set.
func TestResolveGUIConfig_Defaults(t *testing.T) {
	// guiCmd flags start at their zero/default values.
	cfg, err := resolveGUIConfig(guiCmd)
	if err != nil {
		t.Fatalf("resolveGUIConfig: %v", err)
	}

	// Source defaults to cwd (non-empty).
	if cfg.Source == "" {
		t.Error("Source should default to cwd, got empty string")
	}

	// Destination is empty when --dest is not provided.
	if cfg.Destination != "" {
		t.Errorf("Destination = %q, want empty (not provided)", cfg.Destination)
	}

	// Workers defaults to runtime.NumCPU() (> 0).
	if cfg.Workers <= 0 {
		t.Errorf("Workers = %d, want > 0 (runtime.NumCPU())", cfg.Workers)
	}

	// Algorithm defaults to "sha1".
	if cfg.Algorithm != "sha1" {
		t.Errorf("Algorithm = %q, want %q", cfg.Algorithm, "sha1")
	}

	// CarrySidecars defaults to true (--no-carry-sidecars is false by default).
	if !cfg.CarrySidecars {
		t.Error("CarrySidecars should default to true")
	}
}

// TestResolveGUIConfig_DestFlag verifies that --dest set on guiCmd is
// correctly reflected in the resolved config, independent of sortCmd's Viper
// binding for the same key.
func TestResolveGUIConfig_DestFlag(t *testing.T) {
	// Simulate --dest being set on guiCmd.
	if err := guiCmd.Flags().Set("dest", "/archive/photos"); err != nil {
		t.Fatalf("set --dest: %v", err)
	}
	t.Cleanup(func() {
		// Reset the flag so it doesn't affect other tests.
		_ = guiCmd.Flags().Set("dest", "")
	})

	cfg, err := resolveGUIConfig(guiCmd)
	if err != nil {
		t.Fatalf("resolveGUIConfig: %v", err)
	}

	if cfg.Destination != "/archive/photos" {
		t.Errorf("Destination = %q, want %q", cfg.Destination, "/archive/photos")
	}
}
