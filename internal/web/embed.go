// Package web holds the static UI embedded in the binary.
package web

import "embed"

// Files is the browser UI. It is served only to 127.0.0.1.
//
//go:embed index.html app.css app.js
var Files embed.FS
