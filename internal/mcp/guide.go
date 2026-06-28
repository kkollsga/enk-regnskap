package mcp

import _ "embed"

// guideDoc er den fulle bruksanvisningen som serveres av guide-verktoyet, så en
// agent alltid får komplett, versjons-matchende dokumentasjon fra appen selv –
// ikke en (potensielt avkortet) kopi i en slash-kommando.
//
//go:embed guide.md
var guideDoc string
