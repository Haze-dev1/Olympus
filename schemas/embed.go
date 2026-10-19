// Package schemas publishes the input schemas used by the offline core.
package schemas

import "embed"

// Files contains the stack and recipe JSON Schemas.
//
//go:embed *.schema.json
var Files embed.FS
