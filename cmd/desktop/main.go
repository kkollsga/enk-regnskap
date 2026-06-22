//go:build darwin

// Command desktop kjorer ENK Regnskap som en frittstaaende macOS-app: en native
// WKWebView i et NSWindow (med macOS-tittellinje) som viser appen, mens
// HTTP-serveren kjorer in-process. Bygges som EnkRegnskap.app via make mac-app.
package main

import (
	"log"
	"net"
	"net/http"

	webview "github.com/webview/webview_go"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/server"
)

func main() {
	// Flerprosjekt-workspace under ~/ENK-Regnskap/.
	ws, err := core.NewWorkspace(core.DefaultBaseDir(), nil)
	if err != nil {
		log.Fatalf("kunne ikke aapne prosjektmappe: %v", err)
	}
	defer ws.Close()

	srv, err := server.NewWithWorkspace(ws)
	if err != nil {
		log.Fatalf("kunne ikke bygge server: %v", err)
	}

	// Lytt paa en tilfeldig ledig localhost-port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("kunne ikke binde port: %v", err)
	}
	url := "http://" + ln.Addr().String()
	go func() {
		if err := http.Serve(ln, srv); err != nil {
			log.Printf("server stoppet: %v", err)
		}
	}()

	// Native vindu med macOS-tittellinje.
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle("ENK Regnskap")
	w.SetSize(1280, 860, webview.HintNone)
	w.Navigate(url)
	w.Run()
}
