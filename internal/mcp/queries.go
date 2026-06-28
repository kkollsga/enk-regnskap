package mcp

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/kkollsga/enk-regnskap/internal/core"
	"github.com/kkollsga/enk-regnskap/internal/db"
)

// filterIncome/filterExpenses begrenser radene til det agenten faktisk spør om
// (land, kategori, måned, antall) så svaret holder seg lite.
func filterIncome(rows []db.Income, a Args) []db.Income {
	country := strings.ToUpper(a.str("country_code"))
	category := a.str("category")
	month := a.str("month")
	limit := a.intval("limit")
	var out []db.Income
	for _, r := range rows {
		if country != "" && strings.ToUpper(r.CountryCode) != country {
			continue
		}
		if category != "" && r.Category != category {
			continue
		}
		if month != "" && !strings.HasPrefix(r.Date, month) {
			continue
		}
		out = append(out, r)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func filterExpenses(rows []db.Expense, a Args) []db.Expense {
	country := strings.ToUpper(a.str("country_code"))
	category := a.str("category")
	month := a.str("month")
	incomeID := int64(a.intval("income_id"))
	limit := a.intval("limit")
	var out []db.Expense
	for _, r := range rows {
		if country != "" && strings.ToUpper(r.CountryCode) != country {
			continue
		}
		if category != "" && r.Category != category {
			continue
		}
		if month != "" && !strings.HasPrefix(r.Date, month) {
			continue
		}
		if incomeID > 0 && (!r.IncomeID.Valid || r.IncomeID.Int64 != incomeID) {
			continue
		}
		out = append(out, r)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

type aggBucket struct {
	Group         string  `json:"group"`
	Count         int     `json:"count"`
	SumNok        float64 `json:"sum_nok"`
	AvgNok        float64 `json:"avg_nok"`
	DeductibleNok float64 `json:"deductible_nok,omitempty"`
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

// aggregateRun summerer/snitter uten å sende rader tilbake.
func aggregateRun(ctx context.Context, app *core.App, a Args) (string, error) {
	kind := strings.ToLower(a.str("kind"))
	year := a.intval("year")
	groupBy := strings.ToLower(a.str("group_by"))
	if groupBy == "" {
		groupBy = "total"
	}
	groupKey := func(date, country, category string) string {
		switch groupBy {
		case "category":
			return category
		case "country":
			return country
		case "month":
			if len(date) >= 7 {
				return date[:7]
			}
			return date
		default:
			return "total"
		}
	}

	buckets := map[string]*aggBucket{}
	order := []string{}
	add := func(key string, amt, deductible float64) {
		b := buckets[key]
		if b == nil {
			b = &aggBucket{Group: key}
			buckets[key] = b
			order = append(order, key)
		}
		b.Count++
		b.SumNok += amt
		b.DeductibleNok += deductible
	}

	switch kind {
	case "income":
		rows, err := app.ListIncome(ctx, year)
		if err != nil {
			return "", err
		}
		for _, r := range filterIncome(rows, a) {
			add(groupKey(r.Date, r.CountryCode, r.Category), r.AmountNok, 0)
		}
	case "expenses", "expense":
		rows, err := app.ListExpenses(ctx, year)
		if err != nil {
			return "", err
		}
		for _, r := range filterExpenses(rows, a) {
			add(groupKey(r.Date, r.CountryCode, r.Category), r.AmountNok, r.DeductibleNok)
		}
	default:
		return "", fmt.Errorf("kind må være 'income' eller 'expenses'")
	}

	out := make([]aggBucket, 0, len(order))
	for _, k := range order {
		b := buckets[k]
		b.AvgNok = 0
		if b.Count > 0 {
			b.AvgNok = round2(b.SumNok / float64(b.Count))
		}
		b.SumNok = round2(b.SumNok)
		b.DeductibleNok = round2(b.DeductibleNok)
		out = append(out, *b)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SumNok > out[j].SumNok })
	return toJSON(out), nil
}
