// Package webui embeds the built SPA. Docker replaces dist/ with the Vite build.
package webui

import "embed"

// Dist is the SPA build output.
//
//go:embed all:dist
var Dist embed.FS
