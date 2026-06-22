-- Seed-data for land: Norge og Brasil.
-- Idempotent via INSERT OR IGNORE mot UNIQUE-constraints.
-- Tallverdier (satser) oppdateres fra research/Skatteetaten; se notes.

-- ---------------------------------------------------------------------------
-- country_tax_rules
-- ---------------------------------------------------------------------------

-- Norge (hjemstat - ingen skatteavtale med seg selv)
INSERT OR IGNORE INTO country_tax_rules
  (country_code, country_name, effective_from, effective_to, has_tax_treaty,
   treaty_method, treaty_reference, standard_withholding_pct, notes)
VALUES
  ('NO', 'Norge', 2015, NULL, 0, NULL, NULL, NULL,
   'Hjemstat. Inntekt skattlegges etter intern norsk rett. Utenlandsk skatt gir kreditfradrag etter sktl. § 16-20 flg. eller skatteavtale.');

-- Brasil, periode FOR skatteavtalen (inntektsår t.o.m. 2024): intern rett
INSERT OR IGNORE INTO country_tax_rules
  (country_code, country_name, effective_from, effective_to, has_tax_treaty,
   treaty_method, treaty_reference, standard_withholding_pct, notes)
VALUES
  ('BR', 'Brasil', 2015, 2024, 0, NULL, NULL, 15.0,
   'Inntektsår 2024 og tidligere: ingen skatteavtale i kraft. Kreditfradrag for brasiliansk skatt etter intern norsk rett (sktl. § 16-20 til § 16-28). Maksimalt kreditfradrag beregnes per land (§ 16-21), overskytende fremfores i inntil 10 år (§ 16-22).');

-- Brasil, periode ETTER skatteavtalen (inntektsår f.o.m. 2025): kreditmetoden
INSERT OR IGNORE INTO country_tax_rules
  (country_code, country_name, effective_from, effective_to, has_tax_treaty,
   treaty_in_force_date, treaty_method, treaty_reference, treaty_source_url,
   standard_withholding_pct, notes)
VALUES
  ('BR', 'Brasil', 2025, NULL, 1, '2024-12-30', 'credit',
   'Prop. 13 S (2022-2023)',
   'https://lovdata.no/dokument/TRAKTAT/traktat/2022-11-04-19',
   15.0,
   'Skatteavtale Norge-Brasil undertegnet 4. november 2022, i kraft 30. desember 2024, gjelder fra inntektsår 2025. Dobbeltbeskatning avhjelpes med kreditmetoden.');

-- ---------------------------------------------------------------------------
-- country_tax_types - Brasil
-- ---------------------------------------------------------------------------

INSERT INTO country_tax_types
  (country_code, tax_type_code, tax_type_name, description, applies_to,
   is_creditable_in_norway, basis, typical_rate_pct, effective_from)
VALUES
  ('BR', 'IRRF', 'Imposto de Renda Retido na Fonte',
   'Brasiliansk kildeskatt på inntekt. Trekkes ved kilden på tjenester og honorar. Dette er en inntektsskatt og gir kreditfradrag i Norge.',
   'tjenester', 1, 'brutto', 15.0, 2015),
  ('BR', 'ISS', 'Imposto Sobre Serviços',
   'Kommunal tjenesteskatt (indirekte, ikke en inntektsskatt). Gir normalt ikke kreditfradrag i Norge.',
   'tjenester', 0, 'brutto', 5.0, 2015),
  ('BR', 'CSLL', 'Contribuição Social sobre o Lucro Líquido',
   'Sosial bidragsskatt på selskapsoverskudd. Normalt ikke relevant eller krediterbar for et norsk ENK.',
   'selskap', 0, 'netto', 9.0, 2015),
  ('BR', 'PIS', 'Programa de Integração Social',
   'Bidragsskatt på omsetning. Ikke en inntektsskatt – normalt ikke krediterbar i Norge.',
   'omsetning', 0, 'brutto', 0.65, 2015),
  ('BR', 'COFINS', 'Contribuição para o Financiamento da Seguridade Social',
   'Bidragsskatt på omsetning til finansiering av sosial trygghet. Normalt ikke krediterbar i Norge.',
   'omsetning', 0, 'brutto', 3.0, 2015)
ON CONFLICT(country_code, tax_type_code, effective_from) DO UPDATE SET
  tax_type_name = excluded.tax_type_name,
  description = excluded.description,
  is_creditable_in_norway = excluded.is_creditable_in_norway,
  basis = excluded.basis,
  typical_rate_pct = excluded.typical_rate_pct;

-- ---------------------------------------------------------------------------
-- country_tax_types - Norge
-- ---------------------------------------------------------------------------

INSERT OR IGNORE INTO country_tax_types
  (country_code, tax_type_code, tax_type_name, description, applies_to,
   is_creditable_in_norway, basis, typical_rate_pct, effective_from)
VALUES
  ('NO', 'INNTEKTSSKATT', 'Alminnelig inntektsskatt',
   'Skatt på alminnelig inntekt (næringsresultat etter fradrag).',
   'alminnelig inntekt', 0, 'netto', 22.0, 2015),
  ('NO', 'TRINNSKATT', 'Trinnskatt på personinntekt',
   'Progressiv skatt på personinntekt over fastsatte terskler.',
   'personinntekt', 0, 'brutto', NULL, 2015),
  ('NO', 'TRYGDEAVGIFT', 'Trygdeavgift (selvstendig næringsdrivende)',
   'Trygdeavgift på personinntekt fra næring. Høyere sats enn for lønnstakere.',
   'personinntekt', 0, 'brutto', 11.0, 2015);
