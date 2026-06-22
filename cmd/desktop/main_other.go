//go:build !darwin

package main

import "log"

func main() {
	log.Fatal("Den frittstaaende desktop-appen stottes kun paa macOS. " +
		"Bruk 'enk-regnskap' (cmd/server) i nettleser paa andre plattformer.")
}
