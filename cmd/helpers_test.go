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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// resolveAlias
// ---------------------------------------------------------------------------

func TestResolveAlias_found(t *testing.T) {
	t.Parallel()
	aliases := map[string]string{"nas": "/Volumes/NAS/Photos"}
	got, err := resolveAlias("+nas", aliases)
	if err != nil {
		t.Fatalf("resolveAlias: unexpected error: %v", err)
	}
	if got != "/Volumes/NAS/Photos" {
		t.Errorf("resolveAlias = %q, want %q", got, "/Volumes/NAS/Photos")
	}
}

func TestResolveAlias_notFound(t *testing.T) {
	t.Parallel()
	aliases := map[string]string{"nas": "/Volumes/NAS/Photos"}
	_, err := resolveAlias("+missing", aliases)
	if err == nil {
		t.Fatal("resolveAlias(+missing) expected error, got nil")
	}
	// Error message should list the available alias.
	if !strings.Contains(err.Error(), "+nas") {
		t.Errorf("error message should list available aliases; got: %v", err)
	}
}

func TestResolveAlias_noPrefix(t *testing.T) {
	t.Parallel()
	aliases := map[string]string{"nas": "/Volumes/NAS/Photos"}
	got, err := resolveAlias("/some/literal/path", aliases)
	if err != nil {
		t.Fatalf("resolveAlias(literal): unexpected error: %v", err)
	}
	if got != "/some/literal/path" {
		t.Errorf("resolveAlias(literal) = %q, want %q", got, "/some/literal/path")
	}
}

func TestResolveAlias_emptyName(t *testing.T) {
	t.Parallel()
	_, err := resolveAlias("+", map[string]string{"nas": "/Volumes/NAS"})
	if err == nil {
		t.Fatal("resolveAlias(+) expected error for empty alias name, got nil")
	}
}

func TestResolveAlias_noAliases(t *testing.T) {
	t.Parallel()
	_, err := resolveAlias("+nas", map[string]string{})
	if err == nil {
		t.Fatal("resolveAlias with empty aliases map expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no aliases") {
		t.Errorf("error message should mention 'no aliases'; got: %v", err)
	}
}

func TestResolveAlias_nilAliasMap(t *testing.T) {
	t.Parallel()
	_, err := resolveAlias("+nas", nil)
	if err == nil {
		t.Fatal("resolveAlias with nil aliases map expected error, got nil")
	}
}

func TestResolveAlias_tildeExpansion(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory:", err)
	}
	aliases := map[string]string{"home": "~/Photos"}
	got, err := resolveAlias("+home", aliases)
	if err != nil {
		t.Fatalf("resolveAlias(+home): unexpected error: %v", err)
	}
	want := filepath.Join(home, "Photos")
	if got != want {
		t.Errorf("resolveAlias tilde expansion = %q, want %q", got, want)
	}
}

func TestResolveAlias_noTildeExpansionForLiteral(t *testing.T) {
	t.Parallel()
	// A literal path starting with ~ (not via alias) is returned unchanged.
	got, err := resolveAlias("~/Photos", map[string]string{})
	if err != nil {
		t.Fatalf("resolveAlias(~/Photos): unexpected error: %v", err)
	}
	if got != "~/Photos" {
		t.Errorf("resolveAlias(~/Photos) = %q, want %q", got, "~/Photos")
	}
}

func TestResolveAlias_multipleAliasesListedOnError(t *testing.T) {
	t.Parallel()
	aliases := map[string]string{
		"nas":    "/Volumes/NAS",
		"backup": "/Volumes/Backup",
		"local":  "/tmp/local",
	}
	_, err := resolveAlias("+unknown", aliases)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// All three aliases should appear in the error message, sorted.
	for _, name := range []string{"+backup", "+local", "+nas"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error message missing %q; got: %v", name, err)
		}
	}
}
