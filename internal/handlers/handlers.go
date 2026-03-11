package handlers

import (
	"github.com/hayward-solutions/dispatch.v2/internal/tmpl"
)

// renderer is the shared template renderer, set during server initialization.
var renderer *tmpl.Renderer

// SetRenderer sets the shared template renderer for all handlers.
func SetRenderer(r *tmpl.Renderer) {
	renderer = r
}
