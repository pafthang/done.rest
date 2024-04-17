package modweb

import (
	"embed"
)

// EmbeddedViews contains all html templates
// FIXME: for some reason "views/*.gohtml" does not work
//
//go:embed web_view
var EmbeddedViews embed.FS

// EmbeddedStatic contains all static assets
//
//go:embed web_stat/* web_comp/*
var EmbeddedStatic embed.FS
