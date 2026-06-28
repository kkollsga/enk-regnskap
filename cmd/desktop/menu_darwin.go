//go:build darwin

package main

/*
#cgo darwin LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "menu_darwin.h"
*/
import "C"

import (
	_ "embed"
	"encoding/json"
	"unsafe"

	webview "github.com/webview/webview_go"
)

// gWebview holder vindusinstansen slik at native menyhandlinger kan kjøre JS.
var gWebview webview.WebView

// agentSkillPrompt er ferdig-prompten som ber en AI-agent lage en /enk
// slash-kommando for å snakke med appen over MCP. Kopieres til utklippstavlen
// fra Agent-menyen.
//
//go:embed enk_agent_prompt.md
var agentSkillPrompt string

//export goMenuSwitchCompany
func goMenuSwitchCompany() { evalInWebview("location.href='/projects'") }

//export goMenuCopyAgentPrompt
func goMenuCopyAgentPrompt() {
	cs := C.CString(agentSkillPrompt)
	defer C.free(unsafe.Pointer(cs))
	C.copyToClipboard(cs)
	evalInWebview(toastJS("Agent-prompt for /enk kopiert til utklippstavlen ✓"))
}

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

// toastJS bygger en selvstendig liten «toast»-melding (uavhengig av app-CSS)
// som vises nederst og forsvinner av seg selv.
func toastJS(msg string) string {
	b, _ := json.Marshal(msg)
	return "(function(){var m=" + string(b) + ";var d=document.createElement('div');" +
		"d.textContent=m;d.style.cssText='position:fixed;bottom:24px;left:50%;transform:translateX(-50%);" +
		"background:#1D3557;color:#fff;padding:12px 18px;border-radius:8px;font:14px -apple-system,Helvetica,Arial,sans-serif;" +
		"z-index:99999;box-shadow:0 4px 16px rgba(0,0,0,.25);opacity:0;transition:opacity .2s';" +
		"document.body.appendChild(d);requestAnimationFrame(function(){d.style.opacity=1});" +
		"setTimeout(function(){d.style.opacity=0;setTimeout(function(){d.remove()},300)},2600);})();"
}
