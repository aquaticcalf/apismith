package ui

import "embed"

// Files is the console SPA, served by the local process.
//
//go:embed *.html *.css *.js
var Files embed.FS
