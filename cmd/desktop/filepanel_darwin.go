//go:build darwin

package main

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa -framework WebKit
#include "filepanel_darwin.h"
*/
import "C"

import "unsafe"

// enableFileOpenPanel aktiverer fil-input-velgeren (NSOpenPanel) for WKWebView-en
// i det gitte vinduet (NSWindow*).
func enableFileOpenPanel(win unsafe.Pointer) {
	if win != nil {
		C.enableFileOpenPanel(win)
	}
}
