package core

// Category er et valg i en nedtrekksmeny (maskinnøkkel + visningsnavn).
type Category struct {
	Key  string
	Name string
}

// IncomeCategories er inntektskategoriene for et tjenestebasert ENK.
func IncomeCategories() []Category {
	return []Category{
		{Key: "tjenesteinntekt", Name: "Tjenesteinntekt"},
		{Key: "honorar", Name: "Honorar"},
		{Key: "konsulent", Name: "Konsulentinntekt"},
		{Key: "royalty", Name: "Royalty / lisens"},
		{Key: "annet", Name: "Annen næringsinntekt"},
	}
}

// SupportedCurrencies er valutaene appen støtter (NOK først).
func SupportedCurrencies() []string {
	return []string{"NOK", "USD", "EUR", "BRL", "GBP", "SEK", "DKK"}
}

// IsIncomeCategory sjekker om en nøkkel er en gyldig inntektskategori.
func IsIncomeCategory(key string) bool {
	for _, c := range IncomeCategories() {
		if c.Key == key {
			return true
		}
	}
	return false
}
