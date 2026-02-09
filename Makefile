BINARY := dashboard-generator
PKG := github.com/wcatz/dashboard-generator
CMD := ./cmd/dashboard-generator

.PHONY: build test lint clean run-dry

build:
	go build -o $(BINARY) $(CMD)

test:
	go test ./... -v

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
	rm -f gen-*.json

run-dry: build
	./$(BINARY) generate --config example-config.yaml --dry-run --verbose
