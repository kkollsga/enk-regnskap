//go:build darwin

// Command desktop kjorer ENK Regnskap som en frittstående macOS-app: en native
// WKWebView i et NSWindow (med macOS-tittellinje) som viser appen, mens
// HTTP-serveren kjorer in-process. Bygges som EnkRegnskap.app via make mac-app.
package main

import (
	"html"
	"log"
	"net"
	"net/http"

	webview "github.com/webview/webview_go"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/server"
)

func main() {
	// Åpne vinduet FØR oppstart, slik at en feil kan vises pent i stedet for
	// at appen «forsvinner» uten beskjed.
	w := webview.New(false)
	defer w.Destroy()
	gWebview = w
	w.SetTitle("ENK Regnskap")
	w.SetSize(1280, 860, webview.HintNone)
	installAppMenu() // native menylinje: Foretak, Språk, Rediger, Avslutt

	url, err := startServer()
	if err != nil {
		log.Printf("oppstart feilet: %v", err)
		w.SetHtml(errorPage(err))
	} else {
		w.Navigate(url)
	}
	// Aktiver fil-input-velgeren (NSOpenPanel) – uten en WKUIDelegate gjør
	// <input type="file"> ingenting i WKWebView. Gir også iPhone-import.
	enableFileOpenPanel(w.Window())
	w.Run()
	core.RemoveMCPEndpoint(core.DefaultBaseDir())
}

// startServer setter opp workspace + HTTP-server og returnerer URL-en.
func startServer() (string, error) {
	ws, err := core.NewWorkspace(core.DefaultBaseDir(), nil)
	if err != nil {
		return "", err
	}
	srv, err := server.NewWithWorkspace(ws)
	if err != nil {
		return "", err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	go func() {
		if err := http.Serve(ln, srv); err != nil {
			log.Printf("server stoppet: %v", err)
		}
	}()
	url := "http://" + ln.Addr().String()
	// Gjor adressen synlig for en lokal AI-agent (MCP over POST /mcp). Uten
	// dette er den tilfeldige porten umulig å finne.
	if err := core.WriteMCPEndpoint(core.DefaultBaseDir(), url); err != nil {
		log.Printf("kunne ikke skrive MCP-endepunkt: %v", err)
	}
	return url, nil
}

// errorPage gir en pen, lesbar feilside i stedet for en hard krasj.
func errorPage(err error) string {
	msg := html.EscapeString(err.Error())
	return `<!doctype html><html lang="nb"><head><meta charset="utf-8">
<style>
  body{font:15px/1.5 -apple-system,Helvetica,Arial,sans-serif;color:#1a1a1a;
       margin:0;display:flex;align-items:center;justify-content:center;height:100vh;background:#fff}
  .box{max-width:560px;padding:40px;text-align:center}
  h1{color:#1D3557;font-size:22px;margin:0 0 8px}
  .err{background:#fdf1f1;border-left:3px solid #D62828;border-radius:8px;
       padding:12px 16px;margin:18px 0;text-align:left;font-size:13px;color:#7a1f1f;word-break:break-word}
  p{color:#6b7280}
</style></head><body><div class="box">
  <h1>Kunne ikke starte ENK Regnskap</h1>
  <p>Det oppstod en feil under oppstart. Dataene dine er ikke endret.</p>
  <div class="err">` + msg + `</div>
  <p>Prøv å starte appen på nytt. Hvis problemet vedvarer, sjekk at
     mappen <strong>~/ENK-Regnskap</strong> er tilgjengelig, eller ta kontakt for hjelp.</p>
</div></body></html>`
}
