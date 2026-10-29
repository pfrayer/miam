// Package web holds the embedded front-end.
package web

import "embed"

//go:embed index.html
var FS embed.FS
