// Package web embeds the front end (about page, the PWA and its assets) into the
// binary, so the image is a single file with no runtime asset folder.
package web

import "embed"

// Files holds everything under static/.
//
//go:embed static
var Files embed.FS
