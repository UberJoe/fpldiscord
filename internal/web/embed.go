package web

import "embed"

// distFS carries the built Vite SPA. Locally it is produced by `vite build`
// (outDir ../internal/web/dist); in the Docker image stage 1 emits it and stage
// 2 copies it into this directory before `go build`. A placeholder index.html is
// committed so the module builds and tests on a clean checkout without a Node
// step.
//
//go:embed all:dist
var distFS embed.FS
