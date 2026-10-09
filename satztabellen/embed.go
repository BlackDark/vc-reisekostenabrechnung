// Package satztabellen embeds the BMF-name to ISO 3166-1 map.
package satztabellen

import "embed"

// Files holds laender.csv (German BMF name, ISO code).
//
//go:embed laender.csv
var Files embed.FS
