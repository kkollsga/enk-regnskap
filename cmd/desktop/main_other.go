//go:build !darwin

package main

import "log"

func main() {
	log.Fatal("Den frittstående desktop-appen støttes kun på macOS. " +
		"Bruk 'enk-regnskap' (cmd/server) i nettleser på andre plattformer.")
}
