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

	"github.com/spf13/viper"
)

// ---------------------------------------------------------------------------
// initConfig — helpers
// ---------------------------------------------------------------------------

// captureStderr redirects os.Stderr to a pipe for the duration of fn, then
// returns everything written to it. The original os.Stderr is restored before
// captureStderr returns.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("captureStderr: os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w

	fn()

	_ = w.Close()
	os.Stderr = orig

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	_ = r.Close()
	return string(buf[:n])
}

// resetInitConfig restores all package-level state touched by initConfig so
// tests do not bleed into each other.
func resetInitConfig(t *testing.T) {
	t.Helper()
	viper.Reset()
	cfgFile = ""
	configErr = nil
}

// writeConfig writes content to a file named ".pixe.yaml" inside dir.
func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, ".pixe.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// initConfig — tests
// ---------------------------------------------------------------------------

// TestInitConfig_validConfig verifies that a well-formed config file is loaded
// and its values are available via Viper.
func TestInitConfig_validConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "dest: /tmp/archive\nalgorithm: sha256\n")
	resetInitConfig(t)
	defer resetInitConfig(t)

	viper.SetConfigName(".pixe")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(dir)

	stderr := captureStderr(t, initConfig)

	if viper.ConfigFileUsed() == "" {
		t.Error("expected ConfigFileUsed to be non-empty")
	}
	if got := viper.GetString("dest"); got != "/tmp/archive" {
		t.Errorf("dest = %q, want %q", got, "/tmp/archive")
	}
	if got := viper.GetString("algorithm"); got != "sha256" {
		t.Errorf("algorithm = %q, want %q", got, "sha256")
	}
	if !strings.Contains(stderr, "Using config file:") {
		t.Errorf("expected 'Using config file:' on stderr; got: %q", stderr)
	}
	if configErr != nil {
		t.Errorf("configErr should be nil for valid config; got: %v", configErr)
	}
}

// TestInitConfig_noConfigFile verifies that when no config file exists in the
// search path, stderr is silent and configErr remains nil.
func TestInitConfig_noConfigFile(t *testing.T) {
	dir := t.TempDir() // empty — no .pixe.yaml
	resetInitConfig(t)
	defer resetInitConfig(t)

	viper.SetConfigName(".pixe")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(dir)

	stderr := captureStderr(t, initConfig)

	if stderr != "" {
		t.Errorf("expected silent stderr when no config file exists; got: %q", stderr)
	}
	if configErr != nil {
		t.Errorf("configErr should be nil when no config file found; got: %v", configErr)
	}
}

// TestInitConfig_malformedYAML_autoDiscovery verifies that when a config file
// is auto-discovered but contains invalid YAML, a warning is printed to stderr
// and configErr remains nil (non-fatal — sort can still proceed with CLI flags).
func TestInitConfig_malformedYAML_autoDiscovery(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "dest: :\nbad yaml {{{\n")
	resetInitConfig(t)
	defer resetInitConfig(t)

	viper.SetConfigName(".pixe")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(dir)

	stderr := captureStderr(t, initConfig)

	if !strings.Contains(stderr, "Warning:") {
		t.Errorf("expected 'Warning:' on stderr for malformed auto-discovered config; got: %q", stderr)
	}
	if configErr != nil {
		t.Errorf("configErr should be nil for auto-discovered parse error (non-fatal); got: %v", configErr)
	}
}

// TestInitConfig_explicitConfig_malformed verifies that when --config points
// to a file that cannot be parsed, configErr is set (fatal).
func TestInitConfig_explicitConfig_malformed(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "dest: :\nbad yaml {{{\n")
	resetInitConfig(t)
	defer resetInitConfig(t)

	cfgFile = p

	captureStderr(t, initConfig)

	if configErr == nil {
		t.Fatal("configErr should be non-nil when explicit --config file fails to parse")
	}
	if !strings.Contains(configErr.Error(), p) {
		t.Errorf("configErr should contain the file path %q; got: %v", p, configErr)
	}
}

// TestInitConfig_explicitConfig_notFound verifies that when --config points to
// a nonexistent file, configErr is set (fatal).
func TestInitConfig_explicitConfig_notFound(t *testing.T) {
	resetInitConfig(t)
	defer resetInitConfig(t)

	cfgFile = "/nonexistent/path/.pixe.yaml"

	captureStderr(t, initConfig)

	if configErr == nil {
		t.Fatal("configErr should be non-nil when explicit --config file does not exist")
	}
	if !strings.Contains(configErr.Error(), cfgFile) {
		t.Errorf("configErr should contain the file path %q; got: %v", cfgFile, configErr)
	}
}

// TestInitConfig_plusSigilInConfig verifies that a config file using the "+"
// alias sigil (e.g. dest: +nas) is parsed without error — confirming that "+"
// does not trigger a YAML reserved-character failure the way "@" did.
func TestInitConfig_plusSigilInConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "dest: +nas\naliases:\n  nas: /tmp/archive\n")
	resetInitConfig(t)
	defer resetInitConfig(t)

	viper.SetConfigName(".pixe")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(dir)

	stderr := captureStderr(t, initConfig)

	if configErr != nil {
		t.Fatalf("configErr should be nil for valid config with + sigil; got: %v", configErr)
	}
	if strings.Contains(stderr, "Warning:") {
		t.Errorf("unexpected Warning on stderr for valid + sigil config; got: %q", stderr)
	}
	if got := viper.GetString("dest"); got != "+nas" {
		t.Errorf("dest = %q, want %q", got, "+nas")
	}
}
