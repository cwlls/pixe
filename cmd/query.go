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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe/internal/archivedb"
	"github.com/cwlls/pixe/internal/dblocator"
)

var (
	// queryDB is the read-only database handle opened by PersistentPreRunE
	// and shared by all query subcommands.
	queryDB *archivedb.DB

	// queryDir is the resolved absolute path to the archive directory (dirB).
	queryDir string

	// jsonOut controls whether subcommands emit JSON instead of table output.
	jsonOut bool
)

// queryCmd is the "pixe query" parent command.
var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the archive database",
	Long: `Read-only interrogation of the Pixe archive database.

Subcommands expose the contents of the archive database without modifying
any files. All output is read from the database only — no filesystem access
to the archive is performed.

Use --json to receive machine-readable output suitable for scripting.`,
	PersistentPreRunE:  openQueryDB,
	PersistentPostRunE: closeQueryDB,
}

// openQueryDB is the PersistentPreRunE hook that resolves the database path
// and opens it read-only. The resulting handle is stored in queryDB.
func openQueryDB(cmd *cobra.Command, args []string) error {
	dir := viper.GetString("query_dest")
	if dir == "" {
		return fmt.Errorf("--dest is required")
	}

	// Resolve to absolute path and validate.
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve --dest: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("--dest %q: %w", abs, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("--dest %q is not a directory", abs)
	}
	queryDir = abs

	dbPath := viper.GetString("query_db_path")

	loc, err := dblocator.Resolve(queryDir, dbPath)
	if err != nil {
		return fmt.Errorf("resolve database location: %w", err)
	}
	if loc.Notice != "" {
		fmt.Fprintln(os.Stderr, loc.Notice)
	}

	db, err := archivedb.OpenReadOnly(loc.DBPath)
	if err != nil {
		if strings.Contains(err.Error(), "database not found") {
			return fmt.Errorf(
				"no archive database found for %s. Run 'pixe sort' first to create one",
				queryDir,
			)
		}
		return fmt.Errorf("open archive database: %w", err)
	}
	queryDB = db
	return nil
}

// closeQueryDB is the PersistentPostRunE hook that closes the database handle.
func closeQueryDB(_ *cobra.Command, _ []string) error {
	if queryDB != nil {
		return queryDB.Close()
	}
	return nil
}

func init() {
	rootCmd.AddCommand(queryCmd)

	queryCmd.PersistentFlags().StringP("dest", "d", "", "archive directory containing the database (required)")
	queryCmd.PersistentFlags().String("db-path", "", "explicit path to the SQLite archive database (overrides auto-resolution)")
	queryCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "emit JSON output instead of a table")

	_ = viper.BindPFlag("query_dest", queryCmd.PersistentFlags().Lookup("dest"))
	_ = viper.BindPFlag("query_db_path", queryCmd.PersistentFlags().Lookup("db-path"))
}
