BINARY := enk-regnskap
PKG := ./cmd/server
DIST := dist

.PHONY: build run dev test fmt sqlc dist clean

## build: kompiler binær for denne plattformen
build:
	go build -o $(BINARY) $(PKG)

## run: kjør appen lokalt
run:
	go run $(PKG)

## dev: kjør med hot reload (krever air: go install github.com/air-verse/air@latest)
dev:
	air

## test: kjør alle tester
test:
	go test ./...

## fmt: formatér koden
fmt:
	gofmt -w internal cmd web
	goimports -w internal cmd 2>/dev/null || true

## sqlc: regenerer typesikre queries
sqlc:
	sqlc generate

## dist: krysskompiler for Windows, macOS (arm64) og Linux
dist: clean
	mkdir -p $(DIST)
	GOOS=windows GOARCH=amd64 go build -o $(DIST)/$(BINARY)-windows.exe   $(PKG)
	GOOS=darwin  GOARCH=arm64 go build -o $(DIST)/$(BINARY)-mac-arm64     $(PKG)
	GOOS=darwin  GOARCH=amd64 go build -o $(DIST)/$(BINARY)-mac-intel     $(PKG)
	GOOS=linux   GOARCH=amd64 go build -o $(DIST)/$(BINARY)-linux         $(PKG)

## mac-app: bygg den frittstående macOS-appen EnkRegnskap.app (native vindu)
mac-app:
	bash assets/make-app.sh

## icon: regenerer app-ikonet (krever Chrome for rendering)
icon:
	bash assets/make-icon.sh

## clean: fjern byggeartefakter
clean:
	rm -rf $(DIST) $(BINARY)
