// Package migrations embarque les fichiers SQL versionnés du schéma.
package migrations

import "embed"

// FS contient les migrations NNNN_nom.up.sql et NNNN_nom.down.sql.
//
//go:embed *.sql
var FS embed.FS
