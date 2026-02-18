//go:build embed_web

package server

import "embed"

//go:embed all:dist
var embeddedFiles embed.FS

const hasEmbeddedAssets = true
