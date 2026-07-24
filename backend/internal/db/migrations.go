package db

import "embed"

// MigrationsFS embeds the SQL migration files so they ship with the binary
// and run identically in every environment. Files follow golang-migrate's
// naming: {version}_{title}.{up|down}.sql
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
