// Command server starter ENK Regnskap: en lokal HTTP-server som apner i
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
	"path/filepath"
	"runtime"
	"time"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/server"
)

func main() {
	var (
		dataDir = flag.String("data", defaultDataDir(), "mappe for database og kvitteringer (synkroniseres gjerne via OneDrive)")
		port    = flag.Int("port", 7331, "HTTP-port")
		noOpen  = flag.Bool("no-open", false, "ikke apne nettleseren automatisk")
	)
	flag.Parse()

	app, err := core.New(*dataDir, nil)
	if err != nil {
		log.Fatalf("kunne ikke starte: %v", err)
	}
	defer app.Close()

	srv, err := server.New(app)
	if err != nil {
		log.Fatalf("kunne ikke bygge server: %v", err)
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

	log.Printf("ENK Regnskap kjorer paa %s (data: %s)", url, *dataDir)
	if !*noOpen {
		waitUntilReady(url, 3*time.Second)
		openBrowser(url)
	}

	// Blokker til prosessen avsluttes.
	select {}
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

// defaultDataDir velger en fornuftig standardmappe ved siden av binaeren / hjemme.
func defaultDataDir() string {
	if v := os.Getenv("ENK_DATA_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "data"
	}
	return filepath.Join(home, "ENK-Regnskap", "data")
}

// waitUntilReady venter til serveren svarer paa /health, eller timeout.
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

// openBrowser apner url i standard nettleser paa tvers av plattformer.
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
		log.Printf("kunne ikke apne nettleser automatisk: %v (apne %s manuelt)", err, url)
	}
}
