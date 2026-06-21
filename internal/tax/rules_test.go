package tax

import "testing"

const eps = 0.005

func near(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

func TestLoadAndFallback(t *testing.T) {
	if _, err := Load(2024); err != nil {
		t.Fatalf("Load(2024): %v", err)
	}
	if _, err := Load(2025); err != nil {
		t.Fatalf("Load(2025): %v", err)
	}
	// 2026 finnes ikke -> faller tilbake til 2025
	r, err := Load(2026)
	if err != nil {
		t.Fatalf("Load(2026): %v", err)
	}
	if r.Year != 2025 {
		t.Errorf("Load(2026) = aar %d, vil ha fallback til 2025", r.Year)
	}
	// For tidlig aar finnes ingen regler
	if _, err := Load(2000); err == nil {
		t.Error("Load(2000) skulle gi feil, ingen regler saa langt tilbake")
	}
}

func TestAvailableYears(t *testing.T) {
	years := AvailableYears()
	if len(years) < 2 || years[0] != 2024 || years[len(years)-1] < 2025 {
		t.Errorf("AvailableYears() = %v, forventet minst [2024 ... 2025]", years)
	}
}

func TestTrinnskatt2025(t *testing.T) {
	r, _ := Load(2025)
	// personinntekt 700 000:
	// 88650*1.7% + 391100*4% + 2850*13.7% = 1507.05 + 15644 + 390.45 = 17541.50
	got := r.Trinnskatt(700000)
	if !near(got, 17541.50) {
		t.Errorf("Trinnskatt(700000) 2025 = %.2f, forventet 17541.50", got)
	}
	// Under forste innslagspunkt -> 0
	if got := r.Trinnskatt(200000); got != 0 {
		t.Errorf("Trinnskatt(200000) 2025 = %.2f, forventet 0", got)
	}
}

func TestTrinnskatt2024(t *testing.T) {
	r, _ := Load(2024)
	// personinntekt 300 000: 84800*1.7% + 7150*4% = 1441.60 + 286 = 1727.60
	got := r.Trinnskatt(300000)
	if !near(got, 1727.60) {
		t.Errorf("Trinnskatt(300000) 2024 = %.2f, forventet 1727.60", got)
	}
}

func TestTrygdeavgift2025(t *testing.T) {
	r, _ := Load(2025)
	// Under nedre grense (99 650) -> 0
	if got := r.Trygdeavgift(99650); got != 0 {
		t.Errorf("Trygdeavgift(99650) 2025 = %.2f, forventet 0", got)
	}
	// Like over grensen -> opptrapping (25%) gjelder, ikke full sats
	// 100 000: full=10900, opptrapping=(350)*25%=87.50 -> 87.50
	if got := r.Trygdeavgift(100000); !near(got, 87.50) {
		t.Errorf("Trygdeavgift(100000) 2025 = %.2f, forventet 87.50 (opptrapping)", got)
	}
	// Hoy inntekt -> full sats 10.9%
	// 700 000: full=76300, opptrapping=(600350)*25%=150087.50 -> 76300
	if got := r.Trygdeavgift(700000); !near(got, 76300) {
		t.Errorf("Trygdeavgift(700000) 2025 = %.2f, forventet 76300", got)
	}
}

func TestTrygdeavgift2024(t *testing.T) {
	r, _ := Load(2024)
	// 700 000: full = 11.0% = 77000; opptrapping mye hoyere -> 77000
	if got := r.Trygdeavgift(700000); !near(got, 77000) {
		t.Errorf("Trygdeavgift(700000) 2024 = %.2f, forventet 77000", got)
	}
}

func TestEstimate2025(t *testing.T) {
	r, _ := Load(2025)
	est := r.Estimate(700000, 700000)
	// alminnelig 22% = 154000; trygdeavgift 76300; trinnskatt 17541.50
	if !near(est.AlminneligInntektsskatt, 154000) {
		t.Errorf("AlminneligInntektsskatt = %.2f, forventet 154000", est.AlminneligInntektsskatt)
	}
	if !near(est.SumSkatt, 154000+76300+17541.50) {
		t.Errorf("SumSkatt = %.2f, forventet %.2f", est.SumSkatt, 154000+76300+17541.50)
	}
	// Negativt resultat gir 0 skatt
	if est := r.Estimate(-5000, -5000); est.SumSkatt != 0 {
		t.Errorf("Estimate(negativt) SumSkatt = %.2f, forventet 0", est.SumSkatt)
	}
}

func TestDeductions(t *testing.T) {
	r, _ := Load(2025)
	d, ok := r.DeductionByKey("hjemmekontor")
	if !ok {
		t.Fatal("hjemmekontor mangler i 2025")
	}
	if !near(d.SjablongAmount, 2192) {
		t.Errorf("hjemmekontor sjablong 2025 = %.0f, forventet 2192", d.SjablongAmount)
	}
	r24, _ := Load(2024)
	d24, _ := r24.DeductionByKey("hjemmekontor")
	if !near(d24.SjablongAmount, 2128) {
		t.Errorf("hjemmekontor sjablong 2024 = %.0f, forventet 2128", d24.SjablongAmount)
	}
}
