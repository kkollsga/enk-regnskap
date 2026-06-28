package mcp

import (
	"context"
	"fmt"

	"github.com/kkollsga/enk-regnskap/internal/core"
)

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
			Description: "Bytt aktivt foretak til prosjektmappen 'folder' (se list_companies).",
			InputSchema: obj(map[string]any{"folder": prop("string", "Prosjektmappenavn fra list_companies")}, "folder"),
			Run: func(ctx context.Context, a Args) (string, error) {
				folder := a.str("folder")
				if _, err := ws.Open(folder); err != nil {
					return "", err
				}
				return fmt.Sprintf("Aktivt foretak: %s", folder), nil
			},
		},
	}
}
