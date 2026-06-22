package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/kkollsga/enk-regnskap/internal/db"
)

// Aktorer som kan utfore endringer.
const (
	ActorWeb    = "web"
	ActorMCP    = "mcp"
	ActorSystem = "system"
)

// auditableEntities er tabellene som kan endres og rulles tilbake. Hvitliste
// for aa hindre at dynamisk SQL bygges fra ukjente tabellnavn.
var auditableEntities = map[string]bool{
	"income":              true,
	"expenses":            true,
	"receipts":            true,
	"foreign_tax_credits": true,
}

// snapshotRow leser hele raden som et generisk kart (kolonne -> verdi). Brukes
// til for/etter-tilstand i endringsloggen og til rollback. Returnerer nil hvis
// raden ikke finnes.
func (a *App) snapshotRow(ctx context.Context, entity string, id int64) (map[string]any, error) {
	if !auditableEntities[entity] {
		return nil, fmt.Errorf("ukjent entitet %q", entity)
	}
	rows, err := a.DB.QueryContext(ctx, "SELECT * FROM "+entity+" WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		return nil, nil
	}
	raw := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	m := make(map[string]any, len(cols))
	for i, c := range cols {
		v := raw[i]
		if b, ok := v.([]byte); ok {
			v = string(b)
		}
		m[c] = v
	}
	return m, nil
}

// genericDelete sletter en rad.
func (a *App) genericDelete(ctx context.Context, entity string, id int64) error {
	if !auditableEntities[entity] {
		return fmt.Errorf("ukjent entitet %q", entity)
	}
	_, err := a.DB.ExecContext(ctx, "DELETE FROM "+entity+" WHERE id = ?", id)
	return err
}

// genericInsert setter inn en rad fra et kolonnekart (inkl. id).
func (a *App) genericInsert(ctx context.Context, entity string, row map[string]any) error {
	if !auditableEntities[entity] {
		return fmt.Errorf("ukjent entitet %q", entity)
	}
	cols := sortedKeys(row)
	placeholders := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, c := range cols {
		placeholders[i] = "?"
		args[i] = row[c]
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		entity, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	_, err := a.DB.ExecContext(ctx, q, args...)
	return err
}

// genericUpdate gjenoppretter en rad fra et kolonnekart (alle kolonner, WHERE id).
func (a *App) genericUpdate(ctx context.Context, entity string, row map[string]any) error {
	if !auditableEntities[entity] {
		return fmt.Errorf("ukjent entitet %q", entity)
	}
	id, ok := row["id"]
	if !ok {
		return fmt.Errorf("rad mangler id for oppdatering")
	}
	cols := sortedKeys(row)
	sets := make([]string, 0, len(cols))
	args := make([]any, 0, len(cols)+1)
	for _, c := range cols {
		if c == "id" {
			continue
		}
		sets = append(sets, c+" = ?")
		args = append(args, row[c])
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE id = ?", entity, strings.Join(sets, ", "))
	_, err := a.DB.ExecContext(ctx, q, args...)
	return err
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// logChange skriver en endringslogg-rad og kringkaster en live-hendelse.
func (a *App) logChange(ctx context.Context, actor, op, entity string, id int64, before, after map[string]any, year int, desc string) error {
	bj, err := jsonOrEmpty(before)
	if err != nil {
		return err
	}
	aj, err := jsonOrEmpty(after)
	if err != nil {
		return err
	}
	_, err = a.Q.CreateChangeLog(ctx, db.CreateChangeLogParams{
		Actor:       actor,
		Operation:   op,
		Entity:      entity,
		EntityID:    sql.NullInt64{Int64: id, Valid: id != 0},
		BeforeJson:  nullString(bj),
		AfterJson:   nullString(aj),
		Description: desc,
		RollbackOf:  sql.NullInt64{},
	})
	if err != nil {
		return err
	}
	a.Events.Broadcast(Event{Type: entity, Action: op, Entity: entity, ID: id, Year: year})
	a.syncMirrorBestEffort(ctx)
	return nil
}

// Rollback reverserer en tidligere endring og logger selve tilbakerullingen.
func (a *App) Rollback(ctx context.Context, actor string, changeID int64) error {
	cl, err := a.Q.GetChangeLog(ctx, changeID)
	if err != nil {
		return fmt.Errorf("finn endring %d: %w", changeID, err)
	}
	if cl.RolledBack != 0 {
		return fmt.Errorf("endring %d er allerede rullet tilbake", changeID)
	}
	if cl.Operation == "rollback" {
		return fmt.Errorf("kan ikke rulle tilbake en tilbakerulling")
	}
	entity := cl.Entity
	id := cl.EntityID.Int64

	switch cl.Operation {
	case "insert":
		// Reverser ved aa slette den innsatte raden.
		if err := a.genericDelete(ctx, entity, id); err != nil {
			return err
		}
	case "delete":
		before, err := decodeRow(cl.BeforeJson)
		if err != nil {
			return err
		}
		if err := a.genericInsert(ctx, entity, before); err != nil {
			return err
		}
	case "update":
		before, err := decodeRow(cl.BeforeJson)
		if err != nil {
			return err
		}
		if err := a.genericUpdate(ctx, entity, before); err != nil {
			return err
		}
	default:
		return fmt.Errorf("ukjent operasjon %q", cl.Operation)
	}

	if err := a.Q.MarkChangeRolledBack(ctx, changeID); err != nil {
		return err
	}
	// Loggfor selve tilbakerullingen (uten aa kringkaste dobbelt).
	_, err = a.Q.CreateChangeLog(ctx, db.CreateChangeLogParams{
		Actor:       actor,
		Operation:   "rollback",
		Entity:      entity,
		EntityID:    sql.NullInt64{Int64: id, Valid: id != 0},
		BeforeJson:  cl.AfterJson,
		AfterJson:   cl.BeforeJson,
		Description: fmt.Sprintf("Rullet tilbake endring #%d (%s %s)", changeID, cl.Operation, entity),
		RollbackOf:  sql.NullInt64{Int64: changeID, Valid: true},
	})
	if err != nil {
		return err
	}
	a.Events.Broadcast(Event{Type: "rollback", Action: "rollback", Entity: entity, ID: id})
	a.syncMirrorBestEffort(ctx)
	return nil
}

func jsonOrEmpty(m map[string]any) (string, error) {
	if m == nil {
		return "", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeRow(j sql.NullString) (map[string]any, error) {
	if !j.Valid || j.String == "" {
		return nil, fmt.Errorf("mangler tilstand for rollback")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(j.String), &m); err != nil {
		return nil, err
	}
	// JSON gjor heltall om til float64; SQLite tar imot float for INTEGER-kolonner.
	return m, nil
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
