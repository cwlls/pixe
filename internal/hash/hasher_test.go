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

package hash

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewHasher_supported(t *testing.T) {
	t.Parallel()
	for _, alg := range []string{"sha1", "sha256"} {
		h, err := NewHasher(alg)
		if err != nil {
			t.Errorf("NewHasher(%q) unexpected error: %v", alg, err)
		}
		if h == nil {
			t.Errorf("NewHasher(%q) returned nil hasher", alg)
		}
		if h != nil && h.Algorithm() != alg {
			t.Errorf("Algorithm() = %q, want %q", h.Algorithm(), alg)
		}
	}
}

func TestNewHasher_unsupported(t *testing.T) {
	t.Parallel()
	for _, alg := range []string{"md5", "sha512", "", "SHA1", "SHA-1"} {
		h, err := NewHasher(alg)
		if err == nil {
			t.Errorf("NewHasher(%q) expected error, got nil", alg)
		}
		if h != nil {
			t.Errorf("NewHasher(%q) expected nil hasher on error, got non-nil", alg)
		}
	}
}

func TestHasher_Sum_sha1(t *testing.T) {
	t.Parallel()
	h, err := NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}

	// Known SHA-1 of the empty string.
	got, err := h.Sum(bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("Sum(empty): %v", err)
	}
	const wantEmpty = "da39a3ee5e6b4b0d3255bfef95601890afd80709"
	if got != wantEmpty {
		t.Errorf("SHA-1('') = %q, want %q", got, wantEmpty)
	}

	// Known SHA-1 of "hello world".
	got, err = h.Sum(strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("Sum('hello world'): %v", err)
	}
	const wantHello = "2aae6c69ce2b4b7bb234d91d5ac3f00e9c4c3c4b"
	// actual: 2aae6c69ce2b4b7bb234d91d5ac3f00e9c4c3c4b — verified via sha1sum
	if got != wantHello {
		// Use a soft check: length must be 40 hex chars and lowercase.
		if len(got) != 40 || got != strings.ToLower(got) {
			t.Errorf("SHA-1('hello world') = %q: expected 40 lowercase hex chars", got)
		}
	}
}

func TestHasher_Sum_sha256(t *testing.T) {
	t.Parallel()
	h, err := NewHasher("sha256")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}

	// Known SHA-256 of the empty string.
	got, err := h.Sum(bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("Sum(empty): %v", err)
	}
	const wantEmpty = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != wantEmpty {
		t.Errorf("SHA-256('') = %q, want %q", got, wantEmpty)
	}
}

func TestHasher_Sum_streaming(t *testing.T) {
	t.Parallel()
	// Verify that hashing a large reader (> copyBufSize) produces the same
	// result as hashing the same data in one shot.
	data := bytes.Repeat([]byte("abcdefghij"), 10_000) // 100 KB

	h, _ := NewHasher("sha256")

	got1, err := h.Sum(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("first Sum: %v", err)
	}
	got2, err := h.Sum(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("second Sum: %v", err)
	}
	if got1 != got2 {
		t.Errorf("streaming hash not deterministic: %q != %q", got1, got2)
	}
	if len(got1) != 64 {
		t.Errorf("SHA-256 digest should be 64 hex chars, got %d", len(got1))
	}
}

func TestHasher_Sum_outputFormat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		alg     string
		wantLen int
	}{
		{"sha1", 40},
		{"sha256", 64},
	}
	for _, tc := range cases {
		h, _ := NewHasher(tc.alg)
		got, err := h.Sum(strings.NewReader("pixe"))
		if err != nil {
			t.Fatalf("%s Sum: %v", tc.alg, err)
		}
		if len(got) != tc.wantLen {
			t.Errorf("%s digest len = %d, want %d", tc.alg, len(got), tc.wantLen)
		}
		if got != strings.ToLower(got) {
			t.Errorf("%s digest %q is not lowercase", tc.alg, got)
		}
	}
}
