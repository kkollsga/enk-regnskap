package core

import (
	"context"
	"path/filepath"
	"testing"
)

func TestProjectFolderName(t *testing.T) {
	cases := map[string][2]string{
		"Acme AS - 999888777": {"Acme AS", "999888777"},
		"Acme/Bad:Name - 111": {"Acme/Bad:Name", "111"},
		"Foretak":             {"Foretak", ""},
	}
	for want, in := range cases {
		// sanitering fjerner ulovlige tegn
		got := ProjectFolderName(in[0], in[1])
		if in[0] == "Acme/Bad:Name" {
			want = "Acme Bad Name - 111"
		}
		if got != want {
			t.Errorf("ProjectFolderName(%q,%q) = %q, forventet %q", in[0], in[1], got, want)
		}
	}
}

func TestWorkspaceCreateListOpen(t *testing.T) {
	base := t.TempDir()
	ws, err := NewWorkspace(base, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	if ws.Current() != nil {
		t.Error("nytt tomt workspace skal ikke ha aktivt prosjekt")
	}

	// Opprett to prosjekter.
	p1, err := ws.CreateProject("Acme AS", "999888777")
	if err != nil {
		t.Fatal(err)
	}
	if ws.Current() == nil || ws.CurrentName() != p1.Folder {
		t.Errorf("etter opprettelse skal prosjektet være aktivt (%s)", p1.Folder)
	}
	// Data skal ligge under base/<folder>/.
	if _, err := ws.Current().Q.GetConfig(context.Background(), "business_name"); err != nil {
		// business_name settes ved onboarding, ikke create; ok om mangler
		_ = err
	}
	if filepath.Dir(p1.Path) != base {
		t.Errorf("prosjektsti %q ligger ikke under base %q", p1.Path, base)
	}

	if _, err := ws.CreateProject("Fjord Design AS", "111222333"); err != nil {
		t.Fatal(err)
	}

	projects, err := ws.Projects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("forventet 2 prosjekter, fikk %d", len(projects))
	}

	// Bytt tilbake til første prosjekt.
	if _, err := ws.Open(p1.Folder); err != nil {
		t.Fatal(err)
	}
	if ws.CurrentName() != p1.Folder {
		t.Errorf("aktivt prosjekt = %q, forventet %q", ws.CurrentName(), p1.Folder)
	}
	if got := parseFolderCompany(projects, "999888777"); got != "Acme AS" {
		t.Errorf("parset firmanavn = %q, forventet Acme AS", got)
	}
}

func TestWorkspaceReopensActive(t *testing.T) {
	base := t.TempDir()
	ws, _ := NewWorkspace(base, nil)
	p, _ := ws.CreateProject("Test AS", "123456789")
	ws.Close()

	// Nytt workspace skal åpne sist aktive prosjekt automatisk.
	ws2, err := NewWorkspace(base, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws2.Close()
	if ws2.CurrentName() != p.Folder {
		t.Errorf("gjenåpnet aktivt prosjekt = %q, forventet %q", ws2.CurrentName(), p.Folder)
	}
}

func parseFolderCompany(projects []Project, orgnr string) string {
	for _, p := range projects {
		if p.OrgNr == orgnr {
			return p.Company
		}
	}
	return ""
}
