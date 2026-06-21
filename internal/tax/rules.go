// Package tax inneholder norske skatteregler for enkeltpersonforetak (ENK),
// med en egen fil per inntektsaar (rules_AAAA.go). Reglene registreres
// automatisk via init() og lastes med Load(year).
//
// Tallene er hentet fra Skatteetaten og Lovdata (se Sources-feltet og
// kommentarer i hver aarsfil). Appen skal aldri garantere et endelig
// skatteoppgjor - dette er stotteberegninger og informasjon.
package tax

import (
	"fmt"
	"math"
	"sort"
)

// Deduction beskriver en fradragskategori for et tjenestebasert ENK.
type Deduction struct {
	Key            string  // maskinnokkel brukt i databasen
	Name           string  // offisiell norsk postbetegnelse
	Description    string  // forklaring til brukeren
	PostReference  string  // referanse i naeringsspesifikasjonen (kan vaere tom)
	DefaultPct     float64 // standard fradragsprosent (0-100)
	SjablongAmount float64 // fast aarlig sjablongbelop i NOK (0 = ingen)
	MaxAmount      float64 // maksbelop i NOK (0 = ingen)
	Note           string  // saerregler / vilkaar
}

// TrinnskattBracket er ett trinn i trinnskatten. Threshold er nedre
// innslagspunkt (kr); Rate er prosentsatsen som gjelder for inntekt over
// dette punktet og opp til neste trinn.
type TrinnskattBracket struct {
	Threshold float64
	Rate      float64
}

// Rules samler alle satser og kategorier for ett inntektsaar.
type Rules struct {
	Year int

	Deductions []Deduction

	// Sjablong- og standardsatser (NOK / prosent)
	HjemmekontorSjablong    float64 // fast aarlig hjemmekontorfradrag
	KmRate                  float64 // skattefri sats per km, egen bil
	KmPassengerAddon        float64 // tillegg per km per passasjer
	SmaaanskaffelseLimit    float64 // grense for straksfradrag (ellers saldoavskriving)
	RepresentasjonPerPerson float64 // maks bevertning per person
	EkPrivateAddback        float64 // sjablongtillegg for privat bruk av EK-tjeneste

	// Personinntekt
	AlminneligInntektsskattPct float64 // flat sats paa alminnelig inntekt
	TrygdeavgiftNaeringPct     float64 // trygdeavgift paa naeringsinntekt
	TrygdeavgiftNedreGrense    float64 // nedre grense for trygdeavgift
	TrygdeavgiftOpptrappingPct float64 // opptrappingssats over nedre grense
	TrinnskattBrackets         []TrinnskattBracket

	Sources map[string]string // kilde-URL-er for satsene
}

// DeductionByKey finner en fradragskategori paa maskinnokkel.
func (r Rules) DeductionByKey(key string) (Deduction, bool) {
	for _, d := range r.Deductions {
		if d.Key == key {
			return d, true
		}
	}
	return Deduction{}, false
}

// Trygdeavgift beregner trygdeavgift av beregnet personinntekt.
// Under nedre grense betales ingen avgift. Over grensen er avgiften
// begrenset ("opptrappet") slik at den ikke overstiger opptrappingssatsen
// av inntekten som overstiger grensen.
func (r Rules) Trygdeavgift(personinntekt float64) float64 {
	if personinntekt <= r.TrygdeavgiftNedreGrense {
		return 0
	}
	full := personinntekt * r.TrygdeavgiftNaeringPct / 100.0
	capped := (personinntekt - r.TrygdeavgiftNedreGrense) * r.TrygdeavgiftOpptrappingPct / 100.0
	return Round2(math.Min(full, capped))
}

// Trinnskatt beregner progressiv trinnskatt av personinntekt.
func (r Rules) Trinnskatt(personinntekt float64) float64 {
	brackets := append([]TrinnskattBracket(nil), r.TrinnskattBrackets...)
	sort.Slice(brackets, func(i, j int) bool {
		return brackets[i].Threshold < brackets[j].Threshold
	})
	var total float64
	for i, b := range brackets {
		if personinntekt <= b.Threshold {
			break
		}
		upper := math.Inf(1)
		if i+1 < len(brackets) {
			upper = brackets[i+1].Threshold
		}
		slice := math.Min(personinntekt, upper) - b.Threshold
		if slice > 0 {
			total += slice * b.Rate / 100.0
		}
	}
	return Round2(total)
}

// TaxEstimate er et grovt estimat paa skatt for et ENK.
type TaxEstimate struct {
	Year                    int
	AlminneligInntekt       float64
	Personinntekt           float64
	AlminneligInntektsskatt float64
	Trygdeavgift            float64
	Trinnskatt              float64
	SumSkatt                float64
}

// Estimate gir et forenklet skatteestimat. For et lite ENK uten lonn
// settes ofte personinntekt ~ alminnelig inntekt (naeringsresultat), men
// vi tar begge som parametre for fleksibilitet. Dette er KUN et estimat.
func (r Rules) Estimate(alminneligInntekt, personinntekt float64) TaxEstimate {
	if alminneligInntekt < 0 {
		alminneligInntekt = 0
	}
	if personinntekt < 0 {
		personinntekt = 0
	}
	ais := Round2(alminneligInntekt * r.AlminneligInntektsskattPct / 100.0)
	ta := r.Trygdeavgift(personinntekt)
	ts := r.Trinnskatt(personinntekt)
	return TaxEstimate{
		Year:                    r.Year,
		AlminneligInntekt:       alminneligInntekt,
		Personinntekt:           personinntekt,
		AlminneligInntektsskatt: ais,
		Trygdeavgift:            ta,
		Trinnskatt:              ts,
		SumSkatt:                Round2(ais + ta + ts),
	}
}

// Round2 runder til to desimaler (ore).
func Round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// --- Register / loader ---

var registry = map[int]Rules{}

// register kalles fra init() i hver aarsfil.
func register(r Rules) {
	registry[r.Year] = r
}

// Load returnerer reglene for et inntektsaar. Finnes ikke aaret eksakt,
// brukes naermeste tidligere aar som er registrert (regler gjelder til de
// erstattes). Feiler bare hvis ingen aar er registrert.
func Load(year int) (Rules, error) {
	if r, ok := registry[year]; ok {
		return r, nil
	}
	best := -1
	for y := range registry {
		if y <= year && y > best {
			best = y
		}
	}
	if best == -1 {
		return Rules{}, fmt.Errorf("ingen skatteregler registrert for inntektsaar %d eller tidligere", year)
	}
	return registry[best], nil
}

// AvailableYears returnerer registrerte inntektsaar i stigende rekkefolge.
func AvailableYears() []int {
	years := make([]int, 0, len(registry))
	for y := range registry {
		years = append(years, y)
	}
	sort.Ints(years)
	return years
}
