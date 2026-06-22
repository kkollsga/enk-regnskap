// Command server starter ENK Regnskap: en lokal HTTP-server som åpner i
// nettleseren. All data lagres lokalt i en valgt data-mappe (gjerne i OneDrive).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/mcp"
	"github.com/kkollsga/enk-regnskap/internal/server"
)

func main() {
	var (
		home    = flag.String("home", core.DefaultBaseDir(), "basismappe for prosjekter (~/ENK-Regnskap). Hvert foretak er en undermappe.")
		dataDir = flag.String("data", "", "valgfritt: bruk en enkelt prosjektmappe direkte (overstyrer -home)")
		port    = flag.Int("port", 7331, "HTTP-port")
		noOpen  = flag.Bool("no-open", false, "ikke åpne nettleseren automatisk")
		mcpMode = flag.Bool("mcp", false, "kjor som MCP-server over stdio (for AI-agenter)")
	)
	flag.Parse()

	srv, label, closer, mcpApp, err := build(*home, *dataDir)
	if err != nil {
		log.Fatalf("kunne ikke starte: %v", err)
	}
	defer closer()

	// MCP stdio-modus: ingen web-server, snakk JSON-RPC over stdin/stdout.
	if *mcpMode {
		log.SetOutput(os.Stderr)
		if mcpApp == nil {
			log.Fatalf("ingen aktivt prosjekt for MCP. Opprett et prosjekt i appen først, eller bruk -data <mappe>.")
		}
		if err := mcp.New(mcpApp).ServeStdio(context.Background(), os.Stdin, os.Stdout); err != nil {
			log.Fatalf("mcp stdio: %v", err)
		}
		return
	}

	ln, addr, err := listen(*port)
	if err != nil {
		log.Fatalf("kunne ikke binde port: %v", err)
	}
	url := "http://" + addr

	httpSrv := &http.Server{Handler: srv}
	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server stoppet: %v", err)
		}
	}()

	log.Printf("ENK Regnskap kjorer på %s (%s)", url, label)
	if !*noOpen {
		waitUntilReady(url, 3*time.Second)
		openBrowser(url)
	}

	// Blokker til prosessen avsluttes.
	select {}
}

// build setter opp serveren i enten enkeltprosjekt- (-data) eller
// flerprosjekt-modus (-home). Returnerer også en eventuell App for MCP-stdio.
func build(home, dataDir string) (*server.Server, string, func(), *core.App, error) {
	if dataDir != "" {
		app, err := core.New(dataDir, nil)
		if err != nil {
			return nil, "", nil, nil, err
		}
		srv, err := server.New(app)
		if err != nil {
			app.Close()
			return nil, "", nil, nil, err
		}
		return srv, "data: " + dataDir, func() { app.Close() }, app, nil
	}
	ws, err := core.NewWorkspace(home, nil)
	if err != nil {
		return nil, "", nil, nil, err
	}
	srv, err := server.NewWithWorkspace(ws)
	if err != nil {
		ws.Close()
		return nil, "", nil, nil, err
	}
	return srv, "prosjektmappe: " + home, func() { ws.Close() }, ws.Current(), nil
}

// listen prover oppgitt port, deretter de neste 10, og returnerer en lytter.
func listen(preferred int) (net.Listener, string, error) {
	for p := preferred; p < preferred+10; p++ {
		addr := fmt.Sprintf("127.0.0.1:%d", p)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, addr, nil
		}
	}
	return nil, "", fmt.Errorf("ingen ledig port i %d-%d", preferred, preferred+10)
}

// waitUntilReady venter til serveren svarer på /health, eller timeout.
func waitUntilReady(url string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url+"/health", nil)
		resp, err := http.DefaultClient.Do(req)
		cancel()
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// openBrowser åpner url i standard nettleser på tvers av plattformer.
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd, args = "open", []string{url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	if err := exec.Command(cmd, args...).Start(); err != nil {
		log.Printf("kunne ikke åpne nettleser automatisk: %v (åpne %s manuelt)", err, url)
	}
}
