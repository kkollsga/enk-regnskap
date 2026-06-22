package currency

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// NorgesBank henter dagskurser fra Norges Banks åpne API (SDMX-JSON).
type NorgesBank struct {
	client  *http.Client
	baseURL string
}

// NewNorgesBank lager en klient med fornuftig timeout.
func NewNorgesBank() *NorgesBank {
	return &NorgesBank{
		client:  &http.Client{Timeout: 15 * time.Second},
		baseURL: "https://data.norges-bank.no/api/data/EXR",
	}
}

// FetchRate henter kursen for currency som gjelder for date. Den henter et
// vindu (date-10 dager .. date) og velger siste observasjon t.o.m. date, slik
// at helg/helligdag faller tilbake til nærmeste foregående bankdag.
func (n *NorgesBank) FetchRate(ctx context.Context, currency, date string) (Rate, error) {
	currency = NormalizeCurrency(currency)
	if currency == "NOK" {
		return Rate{Currency: "NOK", Date: date, RateNOK: 1, Source: "fixed"}, nil
	}
	end, err := parseDate(date)
	if err != nil {
		return Rate{}, fmt.Errorf("ugyldig dato %q: %w", date, err)
	}
	start := end.AddDate(0, 0, -10)

	url := fmt.Sprintf("%s/B.%s.NOK.SP?startPeriod=%s&endPeriod=%s&format=sdmx-json",
		n.baseURL, currency, start.Format(dateLayout), end.Format(dateLayout))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Rate{}, err
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return Rate{}, fmt.Errorf("kall Norges Bank: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Rate{}, fmt.Errorf("Norges Bank svarte %d: %s", resp.StatusCode, body)
	}

	obs, err := parseSDMX(resp.Body)
	if err != nil {
		return Rate{}, err
	}
	if len(obs) == 0 {
		return Rate{}, fmt.Errorf("ingen kurs funnet for %s frem til %s", currency, date)
	}
	// Velg siste observasjon t.o.m. ønsket dato.
	sort.Slice(obs, func(i, j int) bool { return obs[i].date < obs[j].date })
	chosen := obs[0]
	for _, o := range obs {
		if o.date <= date {
			chosen = o
		}
	}
	return Rate{
		Currency: currency,
		Date:     chosen.date,
		RateNOK:  chosen.rate,
		Source:   "norges-bank",
	}, nil
}

type observation struct {
	date string
	rate float64
}

// sdmxResponse modellerer de delene av SDMX-JSON vi trenger.
type sdmxResponse struct {
	Data struct {
		DataSets []struct {
			Series map[string]struct {
				Observations map[string][]json.RawMessage `json:"observations"`
			} `json:"series"`
		} `json:"dataSets"`
		Structure struct {
			Dimensions struct {
				Observation []struct {
					ID     string `json:"id"`
					Values []struct {
						ID string `json:"id"`
					} `json:"values"`
				} `json:"observation"`
			} `json:"dimensions"`
		} `json:"structure"`
	} `json:"data"`
}

// parseSDMX trekker ut (dato, kurs)-par fra Norges Banks SDMX-JSON.
func parseSDMX(r io.Reader) ([]observation, error) {
	var doc sdmxResponse
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("tolk SDMX-JSON: %w", err)
	}
	// Finn TIME_PERIOD-dimensjonen (observasjonsindeks -> dato).
	var periods []string
	for _, d := range doc.Data.Structure.Dimensions.Observation {
		if d.ID == "TIME_PERIOD" {
			for _, v := range d.Values {
				periods = append(periods, v.ID)
			}
		}
	}
	if len(periods) == 0 {
		return nil, nil
	}
	var out []observation
	for _, ds := range doc.Data.DataSets {
		for _, series := range ds.Series {
			for idxStr, vals := range series.Observations {
				idx, err := strconv.Atoi(idxStr)
				if err != nil || idx < 0 || idx >= len(periods) || len(vals) == 0 {
					continue
				}
				rate, ok := rawToFloat(vals[0])
				if !ok {
					continue
				}
				out = append(out, observation{date: periods[idx], rate: rate})
			}
		}
	}
	return out, nil
}

// rawToFloat tolker en SDMX-verdi som kan være streng eller tall.
func rawToFloat(raw json.RawMessage) (float64, bool) {
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
