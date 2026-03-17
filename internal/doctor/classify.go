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

package doctor

import "strings"

// Classify returns the Category that best matches the given status and reason.
// It iterates Categories in definition order and returns the first match.
//
// Matching rules:
//  1. The entry's status must be in the category's statuses list.
//  2. At least one of the category's patterns must be a substring of reason
//     (case-insensitive). A category with no patterns matches any reason.
//
// The Uncategorized catch-all is always last in Categories and matches any
// error/failed/mismatch/tag_failed status that no earlier category claimed.
// If status is "duplicate" or "copy", nil is returned — those are not problems.
func Classify(status, reason string) *Category {
	statusLower := strings.ToLower(status)
	reasonLower := strings.ToLower(reason)

	for i := range Categories {
		cat := &Categories[i]
		if !matchesStatus(cat.statuses, statusLower) {
			continue
		}
		if matchesPatterns(cat.patterns, reasonLower) {
			return cat
		}
	}
	return nil
}

// matchesStatus returns true if status appears in the statuses slice.
func matchesStatus(statuses []string, status string) bool {
	for _, s := range statuses {
		if s == status {
			return true
		}
	}
	return false
}

// matchesPatterns returns true if any pattern is a substring of reason,
// or if patterns is empty (catch-all).
func matchesPatterns(patterns []string, reason string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if strings.Contains(reason, strings.ToLower(p)) {
			return true
		}
	}
	return false
}
