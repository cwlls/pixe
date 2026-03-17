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

// Package doctor provides a diagnosis engine that categorizes pipeline errors
// and skips into human-readable categories with plain-language explanations.
//
// It operates on abstract Entry values populated by the cmd/ layer from either
// a ledger file or the archive database. This package has no dependency on
// archivedb, manifest, or any other internal package — only stdlib.
package doctor
