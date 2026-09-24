// Package migrations menyertakan seluruh berkas migrasi SQL ke dalam binary.
// Dengan begitu `make migrate` berjalan identik di lokal maupun di server:
// tidak perlu image migrator terpisah dan tidak perlu menyalin berkas SQL.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
