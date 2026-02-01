package web

import "embed"

//go:embed all:assets
var Assets embed.FS

//go:embed templates
var Templates embed.FS
