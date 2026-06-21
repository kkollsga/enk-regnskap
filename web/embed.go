// Package web inneholder frontend-ressursene (HTML-maler, CSS, JS, i18n) som
// embeddes i binaeren via embed.FS.
package web

import "embed"

//go:embed templates/*.html
var Templates embed.FS

//go:embed static/*
var Static embed.FS

//go:embed i18n/*.json
var I18n embed.FS
