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

-- Brasil: skatteavtale (country_tax_rules) + skattetyper (country_tax_types)
-- lastes deklarativt fra agreements/brazil_norway.json ved seeding.

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
