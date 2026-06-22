package scenarios

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/server"
)

// workspaceServer starter en server med flerprosjekt-stotte for testing.
func workspaceServer(t *testing.T) (*httptest.Server, *core.Workspace, *http.Client) {
	t.Helper()
	ws, err := core.NewWorkspace(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := server.NewWithWorkspace(ws)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	t.Cleanup(func() { ts.Close(); ws.Close() })
	return ts, ws, client
}

func TestNoProjectRedirectsToPicker(t *testing.T) {
	ts, _, client := workspaceServer(t)
	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := readBody(t, resp)
	if !strings.Contains(body, "Velg foretak") {
		t.Error("uten prosjekt skal forsiden vise prosjektvelgeren")
	}
	// Demo-knappen skal vaere synlig her (i motsetning til toppmenyen).
	if !strings.Contains(body, "/projects/demo") {
		t.Error("prosjektvelgeren mangler demo-foretak-knappen")
	}
}

func TestGenerateDemoProjectFromPicker(t *testing.T) {
	ts, ws, client := workspaceServer(t)

	resp, err := client.PostForm(ts.URL+"/projects/demo", url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Et demo-foretak skal nå finnes og vaere valgbart.
	projects, _ := ws.Projects()
	if len(projects) != 1 {
		t.Fatalf("forventet 1 demo-foretak, fikk %d", len(projects))
	}
	if !strings.HasPrefix(projects[0].Folder, "Demo-foretak") {
		t.Errorf("uventet demo-navn: %q", projects[0].Folder)
	}
	// Det skal vaere fylt med testdata (12 inntekter for 2025).
	app := ws.Current()
	if app == nil {
		t.Fatal("demo-foretaket ble ikke aktivt")
	}
	rows, _ := app.ListIncome(context.Background(), 2025)
	if len(rows) != 12 {
		t.Errorf("demo-foretak har %d inntekter, forventet 12", len(rows))
	}

	// Et nytt demo-foretak skal faa et unikt navn.
	resp2, _ := client.PostForm(ts.URL+"/projects/demo", url.Values{})
	resp2.Body.Close()
	projects, _ = ws.Projects()
	if len(projects) != 2 {
		t.Errorf("forventet 2 demo-foretak etter to genereringer, fikk %d", len(projects))
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := make([]byte, 0, 8192)
	tmp := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return string(buf)
}
