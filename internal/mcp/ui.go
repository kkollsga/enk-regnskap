package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// pagePaths kobler en sidenøkkel (norsk eller engelsk) til en URL i appen.
var pagePaths = map[string]string{
	"dashboard": "/", "oversikt": "/", "home": "/", "forside": "/",
	"income": "/income", "inntekter": "/income", "inntekt": "/income",
	"new-income": "/income/new", "new_income": "/income/new", "ny-inntekt": "/income/new",
	"expenses": "/expenses", "utgifter": "/expenses", "utgift": "/expenses",
	"new-expense": "/expenses/new", "new_expense": "/expenses/new", "ny-utgift": "/expenses/new",
	"foreign-tax": "/foreign-tax", "foreign_tax": "/foreign-tax", "utenlandsk-skatt": "/foreign-tax", "utenlandsk": "/foreign-tax",
	"tax-info": "/tax-info", "tax_info": "/tax-info", "skatteinfo": "/tax-info",
	"selvangivelse": "/selvangivelse", "tax-return": "/selvangivelse",
	"reports": "/reports", "rapporter": "/reports",
	"companies": "/projects", "foretak": "/projects", "projects": "/projects",
}

// resolvePage gir URL-en for en sidenøkkel, eller en sti som starter med "/".
func resolvePage(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if p, ok := pagePaths[s]; ok {
		return p
	}
	if strings.HasPrefix(s, "/") {
		return s
	}
	return ""
}

// resolveCompanyFolder finner prosjektmappen for en identifikator (mappenavn,
// org.nr eller firmanavn – eksakt eller delvis).
func resolveCompanyFolder(ws *core.Workspace, ident string) (string, error) {
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return "", fmt.Errorf("oppgi foretak (navn, org.nr eller mappe)")
	}
	projects, err := ws.Projects()
	if err != nil {
		return "", err
	}
	for _, p := range projects {
		if p.Folder == ident || p.OrgNr == ident || strings.EqualFold(p.Company, ident) {
			return p.Folder, nil
		}
	}
	low := strings.ToLower(ident)
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Company), low) || strings.Contains(strings.ToLower(p.Folder), low) {
			return p.Folder, nil
		}
	}
	return "", fmt.Errorf("fant ikke foretak: %s", ident)
}

// toolErr pakker en feil som et MCP-innholdssvar med isError (ikke protokollfeil).
func toolErr(err error) any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": "Feil: " + err.Error()}},
		"isError": true,
	}
}

// uiTools lar en agent styre hva som vises i appens vindu (live via SSE):
// bytte side, bytte språk og slå utvidbare seksjoner av/på.
func (s *Server) uiTools() []Tool {
	app := s.app
	return []Tool{
		{
			Name:        "navigate",
			Description: "Bytt siden som vises i appens vindu (live). page: dashboard|income|expenses|foreign-tax|tax-info|selvangivelse|reports|new-income|new-expense, eller en sti som starter med '/'.",
			InputSchema: obj(map[string]any{"page": prop("string", "Sidenøkkel eller sti")}, "page"),
			Run: func(ctx context.Context, a Args) (string, error) {
				path := resolvePage(a.str("page"))
				if path == "" {
					return "", fmt.Errorf("ukjent side: %s", a.str("page"))
				}
				app.Events.Broadcast(core.Event{Type: "navigate", Path: path})
				return "Viser " + path, nil
			},
		},
		{
			Name:        "set_language",
			Description: "Bytt språk i appen (live): nb (norsk), pt (portugisisk) eller en (engelsk).",
			InputSchema: obj(map[string]any{"lang": prop("string", "nb | pt | en")}, "lang"),
			Run: func(ctx context.Context, a Args) (string, error) {
				lang := strings.ToLower(strings.TrimSpace(a.str("lang")))
				if lang != "nb" && lang != "pt" && lang != "en" {
					return "", fmt.Errorf("lang må være nb, pt eller en")
				}
				app.Events.Broadcast(core.Event{Type: "navigate", Path: "/set-lang?lang=" + lang})
				return "Språk satt til " + lang, nil
			},
		},
		{
			Name:        "ui_toggle",
			Description: "Slå utvidbare seksjoner av/på i visningen (live). selector = CSS-selektor (f.eks. 'details.entry' for alle poster, '.estimate-details' for skatteberegningen). mode = toggle (default) | open | close.",
			InputSchema: obj(map[string]any{
				"selector": prop("string", "CSS-selektor for <details>-elementer"),
				"mode":     prop("string", "toggle | open | close (default toggle)"),
			}, "selector"),
			Run: func(ctx context.Context, a Args) (string, error) {
				mode := strings.ToLower(strings.TrimSpace(a.str("mode")))
				if mode != "open" && mode != "close" {
					mode = "toggle"
				}
				app.Events.Broadcast(core.Event{Type: "ui", Action: mode, Selector: a.str("selector")})
				return fmt.Sprintf("%s %s", mode, a.str("selector")), nil
			},
		},
	}
}
