package tax

// Skatteregler for inntektsaar 2025 (ENK, tjenestebasert, ingen ansatte,
// ingen varelager). Kilder, se Sources nederst.
//
// VIKTIG: Skatteavtalen Norge-Brasil traadte i kraft 30. desember 2024 og
// gjelder fra inntektsaar 2025. Dobbeltbeskatning avhjelpes med kreditmetoden
// (avtalens art. 25). Maksimalt kreditfradrag begrenses fortsatt av norsk
// skatt paa samme inntekt (jf. § 16-21). Ubenyttet kredit fremfoeres inntil
// 5 aar (§ 16-22). Prop. 13 S (2022-2023).

func init() {
	register(Rules{
		Year: 2025,

		Deductions: deductions2025(),

		HjemmekontorSjablong:    2192,  // kr/aar
		KmRate:                  3.50,  // kr/km, egen bil (skattefri sats)
		KmPassengerAddon:        1.00,  // kr/km/passasjer
		SmaaanskaffelseLimit:    30000, // straksfradrag under denne grensen
		RepresentasjonPerPerson: 579,   // kr/person, enkel bevertning
		EkPrivateAddback:        4392,  // kr/aar privat bruk EK-tjeneste

		AlminneligInntektsskattPct: 22.0,
		TrygdeavgiftNaeringPct:     10.9,
		TrygdeavgiftNedreGrense:    99650,
		TrygdeavgiftOpptrappingPct: 25.0,
		TrinnskattBrackets: []TrinnskattBracket{
			{Threshold: 217400, Rate: 1.7},
			{Threshold: 306050, Rate: 4.0},
			{Threshold: 697150, Rate: 13.7},
			{Threshold: 942400, Rate: 16.7},
			{Threshold: 1410750, Rate: 17.7},
		},

		Sources: map[string]string{
			"trinnskatt":      "https://www.skatteetaten.no/satser/trinnskatt/",
			"trygdeavgift":    "https://www.skatteetaten.no/en/rates/national-insurance-contributions/",
			"hjemmekontor":    "https://www.skatteetaten.no/en/rates/home-office-standard-deduction/",
			"bilgodtgjorelse": "https://www.skatteetaten.no/satser/bilgodtgjorelse-kilometergodtgjorelse/",
			"avskrivning":     "https://www.skatteetaten.no/en/rates/depreciation-rates/",
			"stortingsvedtak": "https://lovdata.no/dokument/STV/forskrift/2024-12-13-3203",
			"satsforskrift":   "https://lovdata.no/dokument/SF/forskrift/2024-11-27-2889",
			"skatteavtale_br": "https://lovdata.no/dokument/TRAKTAT/traktat/2022-11-04-19",
		},
	})
}

func deductions2025() []Deduction {
	return []Deduction{
		{Key: "tjenesteinntekt_kostnad", Name: "Driftskostnad (generell)", Description: "Generelle fradragsberettigede driftskostnader.", DefaultPct: 100},
		{Key: "hjemmekontor", Name: "Kostnader hjemmekontor (standardfradrag)", Description: "Sjablongfradrag for hjemmekontor. Krever minst 50% egen bruk. Alternativt faktiske kostnader.", DefaultPct: 100, SjablongAmount: 2192, Note: "Krever >=50% egen bruk av boligen i minst halve aaret."},
		{Key: "kontorrekvisita", Name: "Kontorkostnad / rekvisita", Description: "Kontorrekvisita og forbruksmateriell.", DefaultPct: 100},
		{Key: "telefon_internett", Name: "Elektronisk kommunikasjon (telefon/bredband)", Description: "Telefon og internett til bruk i naeringen.", DefaultPct: 100, Note: "Ved privat bruk legges sjablongtillegg kr 4 392 til personinntekt."},
		{Key: "reise", Name: "Reise-, diett- og oppholdskostnader", Description: "Yrkesreiser. Diett/overnatting etter Skatteetatens skattefrie satser.", DefaultPct: 100},
		{Key: "bil_km", Name: "Bilgodtgjorelse / kjoregodtgjorelse yrkeskjoring", Description: "Yrkeskjoring med egen bil. Skattefri sats kr 3,50/km.", DefaultPct: 100, Note: "kr 3,50/km skattefritt (statens sats kr 5,00/km, differansen er skattepliktig)."},
		{Key: "kurs_faglitteratur", Name: "Kurs og faglitteratur", Description: "Vedlikehold av eksisterende kompetanse.", DefaultPct: 100, Note: "Kurs som gir NY kompetanse er normalt ikke fradragsberettiget."},
		{Key: "forsikring", Name: "Forsikringspremie (yrkes-/ansvarsforsikring)", Description: "Forsikringer knyttet til naeringen.", DefaultPct: 100, Note: "Personlige forsikringer er normalt ikke fradragsberettiget."},
		{Key: "regnskapsprogram", Name: "Programvare / regnskapssystem", Description: "Abonnement og programvare til drift.", DefaultPct: 100, Note: "Evigvarende lisens >= kr 30 000 aktiveres."},
		{Key: "smaa_driftsmidler", Name: "Smaaanskaffelser / driftsmidler (straksfradrag)", Description: "Driftsmidler under kr 30 000 eller med levetid under 3 aar.", DefaultPct: 100, MaxAmount: 30000, Note: "Over kr 30 000 og levetid >=3 aar: aktiveres og saldoavskrives."},
		{Key: "representasjon", Name: "Representasjon (bevertning)", Description: "Enkel bevertning av kunder/forbindelser.", DefaultPct: 100, MaxAmount: 579, Note: "Maks kr 579/person (2025), ingen brennevin. Overskrides belopet er hele kostnaden ikke fradragsberettiget."},
	}
}
