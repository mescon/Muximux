//go:build !embed_web

package server

import "embed"

var embeddedFiles embed.FS

const hasEmbeddedAssets = false
