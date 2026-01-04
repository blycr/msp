package web

import "embed"

//go:embed index.html app.css app.js assets
var FS embed.FS
