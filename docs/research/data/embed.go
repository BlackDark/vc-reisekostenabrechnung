// Package data embeds the BMF Auslandspauschalen extracts.
package data

import "embed"

// Files holds auslandspauschalen_{2024,2025,2026}.csv.
//
//go:embed auslandspauschalen_2024.csv auslandspauschalen_2025.csv auslandspauschalen_2026.csv
var Files embed.FS
