package mcp

import (
	"context"
	"fmt"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

// isCompanyMgmt er sann for verktoy som ikke skal trigge per-kall foretaksbytte
// via 'company'-argumentet (create_company sin 'company' er det nye navnet).
func isCompanyMgmt(name string) bool {
	switch name {
	case "create_company", "open_company", "list_companies":
		return true
	}
	return false
}

// broadcastNavigateHome ber et eventuelt åpent vindu navigere til forsiden, slik
// at det viser det nye aktive foretaket. Må kalles FØR foretaket byttes (mens
// klienten enda abonnerer på det gjeldende prosjektet).
func broadcastNavigateHome(ws *core.Workspace) {
	if cur := ws.Current(); cur != nil {
		cur.Events.Broadcast(core.Event{Type: "navigate", Path: "/"})
	}
}

// companyTools er kun tilgjengelige i flerprosjekt-modus (s.ws != nil). De lar
// en agent liste, opprette og åpne foretak uten å gå via nettgrensesnittet –
// så agenten kan starte fra null.
func (s *Server) companyTools() []Tool {
	ws := s.ws
	return []Tool{
		{
			Name:        "list_companies",
			Description: "List foretak (prosjekter) i arbeidsmappen, med hvilket som er aktivt.",
			InputSchema: obj(map[string]any{}),
			Run: func(ctx context.Context, a Args) (string, error) {
				projects, err := ws.Projects()
				if err != nil {
					return "", err
				}
				active := ws.CurrentName()
				type compOut struct {
					Folder  string `json:"folder"`
					Company string `json:"company"`
					OrgNr   string `json:"org_nr,omitempty"`
					Active  bool   `json:"active,omitempty"`
				}
				out := make([]compOut, 0, len(projects))
				for _, p := range projects {
					out = append(out, compOut{
						Folder: p.Folder, Company: p.Company, OrgNr: p.OrgNr,
						Active: p.Folder == active,
					})
				}
				return toJSON(out), nil
			},
		},
		{
			Name:        "create_company",
			Description: "Opprett et nytt foretak (ENK) og gjor det aktivt. Fullforer onboarding automatisk. Etterpå virker alle inntekts-/utgiftsverktoy mot dette foretaket.",
			InputSchema: obj(map[string]any{
				"company":  prop("string", "Firmanavn"),
				"org_nr":   prop("string", "Organisasjonsnummer (valgfritt)"),
				"language": prop("string", "Språk: nb (default), nn eller en"),
			}, "company"),
			Run: func(ctx context.Context, a Args) (string, error) {
				company := a.str("company")
				orgnr := a.str("org_nr")
				if core.ProjectFolderName(company, orgnr) == "" {
					return "", fmt.Errorf("oppgi firmanavn og/eller organisasjonsnummer")
				}
				broadcastNavigateHome(ws) // be vinduet følge med til det nye foretaket
				proj, err := ws.CreateProject(company, orgnr)
				if err != nil {
					return "", err
				}
				app := ws.Current()
				if app == nil {
					return "", fmt.Errorf("kunne ikke åpne det nye foretaket")
				}
				if err := app.CompleteOnboarding(ctx, core.OnboardInput{
					BusinessName: proj.Company, OrgNr: proj.OrgNr, Language: a.str("language"),
				}); err != nil {
					return "", err
				}
				return toJSON(map[string]any{
					"folder": proj.Folder, "company": proj.Company, "org_nr": proj.OrgNr, "active": true,
				}), nil
			},
		},
		{
			Name:        "open_company",
			Description: "Bytt aktivt foretak. 'company' kan være mappenavn, org.nr eller firmanavn (eksakt eller delvis). Appens vindu navigerer til det nye foretaket.",
			InputSchema: obj(map[string]any{
				"company": prop("string", "Mappenavn, org.nr eller firmanavn"),
				"folder":  prop("string", "Alias for company (mappenavn)"),
			}, "company"),
			Run: func(ctx context.Context, a Args) (string, error) {
				ident := a.str("company")
				if ident == "" {
					ident = a.str("folder")
				}
				folder, err := resolveCompanyFolder(ws, ident)
				if err != nil {
					return "", err
				}
				broadcastNavigateHome(ws) // før byttet, mens klienten enda abonnerer
				if _, err := ws.Open(folder); err != nil {
					return "", err
				}
				return fmt.Sprintf("Aktivt foretak: %s", folder), nil
			},
		},
	}
}
