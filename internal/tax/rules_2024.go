package tax

// Skatteregler for inntektsaar 2024 (ENK, tjenestebasert, ingen ansatte,
// ingen varelager). Kilder, se Sources nederst.
//
// VIKTIG: For inntektsaar 2024 og tidligere finnes ingen skatteavtale i kraft
// mellom Norge og Brasil. Kreditfradrag for utenlandsk skatt foelger intern
// norsk rett (sktl. § 16-20 flg.). Maksimalt kreditfradrag per land/kategori
// (§ 16-21). Ubenyttet kreditfradrag fremfoeres i inntil 5 aar (§ 16-22).
// Krav maa fremmes senest 6 mnd etter at endelig skatt er fastsatt i utlandet
// (§ 16-30).

func init() {
	register(Rules{
		Year: 2024,

		Deductions: deductions2024(),

		HjemmekontorSjablong:    2128,  // kr/aar
		KmRate:                  3.50,  // kr/km, egen bil (skattefri sats)
		KmPassengerAddon:        1.00,  // kr/km/passasjer
		SmaaanskaffelseLimit:    30000, // straksfradrag under denne grensen
		RepresentasjonPerPerson: 562,   // kr/person, enkel bevertning
		EkPrivateAddback:        4392,  // kr/aar privat bruk EK-tjeneste

		AlminneligInntektsskattPct: 22.0,
		TrygdeavgiftNaeringPct:     11.0,
		TrygdeavgiftNedreGrense:    69650,
		TrygdeavgiftOpptrappingPct: 25.0,
		TrinnskattBrackets: []TrinnskattBracket{
			{Threshold: 208050, Rate: 1.7},
			{Threshold: 292850, Rate: 4.0},
			{Threshold: 670000, Rate: 13.6},
			{Threshold: 937900, Rate: 16.6},
			{Threshold: 1350000, Rate: 17.6},
		},

		Sources: map[string]string{
			"trinnskatt":      "https://www.skatteetaten.no/satser/trinnskatt/",
			"trygdeavgift":    "https://www.skatteetaten.no/en/rates/national-insurance-contributions/",
			"hjemmekontor":    "https://www.skatteetaten.no/en/rates/home-office-standard-deduction/",
			"bilgodtgjorelse": "https://www.skatteetaten.no/satser/bilgodtgjorelse-kilometergodtgjorelse/",
			"avskrivning":     "https://www.skatteetaten.no/en/rates/depreciation-rates/",
			"stortingsvedtak": "https://lovdata.no/dokument/STV/forskrift/2023-12-14-2071",
		},
	})
}

func deductions2024() []Deduction {
	return []Deduction{
		{Key: "tjenesteinntekt_kostnad", Name: "Driftskostnad (generell)", Description: "Generelle fradragsberettigede driftskostnader.", DefaultPct: 100},
		{Key: "hjemmekontor", Name: "Kostnader hjemmekontor (standardfradrag)", Description: "Sjablongfradrag for hjemmekontor. Krever minst 50% egen bruk. Alternativt faktiske kostnader.", DefaultPct: 100, SjablongAmount: 2128, Note: "Krever >=50% egen bruk av boligen i minst halve aaret."},
		{Key: "kontorrekvisita", Name: "Kontorkostnad / rekvisita", Description: "Kontorrekvisita og forbruksmateriell.", DefaultPct: 100},
		{Key: "telefon_internett", Name: "Elektronisk kommunikasjon (telefon/bredband)", Description: "Telefon og internett til bruk i naeringen.", DefaultPct: 100, Note: "Ved privat bruk legges sjablongtillegg kr 4 392 til personinntekt."},
		{Key: "reise", Name: "Reise-, diett- og oppholdskostnader", Description: "Yrkesreiser. Diett/overnatting etter Skatteetatens skattefrie satser.", DefaultPct: 100},
		{Key: "bil_km", Name: "Bilgodtgjorelse / kjoregodtgjorelse yrkeskjoring", Description: "Yrkeskjoring med egen bil. Skattefri sats kr 3,50/km.", DefaultPct: 100, Note: "kr 3,50/km i 2024 (+1,00/km per passasjer)."},
		{Key: "kurs_faglitteratur", Name: "Kurs og faglitteratur", Description: "Vedlikehold av eksisterende kompetanse.", DefaultPct: 100, Note: "Kurs som gir NY kompetanse er normalt ikke fradragsberettiget."},
		{Key: "forsikring", Name: "Forsikringspremie (yrkes-/ansvarsforsikring)", Description: "Forsikringer knyttet til naeringen.", DefaultPct: 100, Note: "Personlige forsikringer er normalt ikke fradragsberettiget."},
		{Key: "regnskapsprogram", Name: "Programvare / regnskapssystem", Description: "Abonnement og programvare til drift.", DefaultPct: 100, Note: "Evigvarende lisens >= kr 30 000 aktiveres."},
		{Key: "smaa_driftsmidler", Name: "Smaaanskaffelser / driftsmidler (straksfradrag)", Description: "Driftsmidler under kr 30 000 eller med levetid under 3 aar.", DefaultPct: 100, MaxAmount: 30000, Note: "Over kr 30 000 og levetid >=3 aar: aktiveres og saldoavskrives."},
		{Key: "representasjon", Name: "Representasjon (bevertning)", Description: "Enkel bevertning av kunder/forbindelser.", DefaultPct: 100, MaxAmount: 562, Note: "Maks kr 562/person (2024), ingen brennevin. Overskrides belopet er hele kostnaden ikke fradragsberettiget."},
	}
}
