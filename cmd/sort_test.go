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

// TestSortCmd_sourceNotRequired verifies that the --source flag is not marked
// as required by Cobra (it defaults to cwd at runtime), while --dest remains
// required.
func TestSortCmd_sourceNotRequired(t *testing.T) {
	sourceFlag := sortCmd.Flags().Lookup("source")
	if sourceFlag == nil {
		t.Fatal("--source flag not registered on sortCmd")
	}
	// Cobra stores the required annotation under the key "cobra_annotation_bash_completion_one_required_flag".
	// A simpler check: the flag's annotations map should not contain the required key.
	const requiredKey = "cobra_annotation_bash_completion_one_required_flag"
	if _, required := sourceFlag.Annotations[requiredKey]; required {
		t.Error("--source should not be marked required; it defaults to cwd")
	}

	destFlag := sortCmd.Flags().Lookup("dest")
	if destFlag == nil {
		t.Fatal("--dest flag not registered on sortCmd")
	}
	if _, required := destFlag.Annotations[requiredKey]; !required {
		t.Error("--dest should still be marked required")
	}
}
