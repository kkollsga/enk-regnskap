//go:build darwin

package main

/*
#cgo darwin LDFLAGS: -framework Cocoa
#include "menu_darwin.h"
*/
import "C"

import webview "github.com/webview/webview_go"

// gWebview holder vindusinstansen slik at native menyhandlinger kan kjøre JS.
var gWebview webview.WebView

//export goMenuSwitchCompany
func goMenuSwitchCompany() { evalInWebview("location.href='/projects'") }

//export goMenuGenerateDemo
func goMenuGenerateDemo() {
	evalInWebview("fetch('/dev/dummy-data',{method:'POST'}).then(function(){location.reload()})")
}

//export goMenuLang
func goMenuLang(lang *C.char) {
	evalInWebview("location.href='/set-lang?lang=" + C.GoString(lang) + "'")
}

// installAppMenu bygger den native macOS-menylinjen.
func installAppMenu() { C.installAppMenu() }

func evalInWebview(js string) {
	if gWebview != nil {
		w := gWebview
		w.Dispatch(func() { w.Eval(js) })
	}
}
