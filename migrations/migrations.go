// Package migrations embeds the SQL migration files into the binary so the
// app no longer reads them from disk at runtime. This lets the deployed binary
// run migrations on startup without shipping the migrations/ directory
// alongside it (and removes the run-from-repo-root requirement).
//
// The embed lives here, next to the .sql files, because //go:embed can only
// reference files in its own directory or below — internal/db is too far away
// to embed this folder directly.
package migrations

import "embed"

// FS holds every NNNNNN_name.up.sql / .down.sql pair in this directory.
//
//go:embed *.sql
var FS embed.FS
