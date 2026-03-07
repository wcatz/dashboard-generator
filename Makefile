BINARY := dashboard-generator
PKG := github.com/wcatz/dashboard-generator
CMD := ./cmd/dashboard-generator
IMAGE := wcatz/dashboard-generator

# version info
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w \
  -X main.version=$(VERSION) \
  -X main.commit=$(COMMIT) \
  -X main.buildDate=$(BUILD_DATE)

.PHONY: build test lint clean run-dry docker-build docker-run helm-lint

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

test:
	go test ./... -v

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
	rm -f gen-*.json

run-dry: build
	./$(BINARY) generate --config example-config.yaml --dry-run --verbose

docker-build:
	docker build -t $(IMAGE):latest .

docker-run:
	docker run --rm -p 8080:8080 \
		-v $(PWD)/example-config.yaml:/data/config.yaml:ro \
		-v $(PWD)/output:/data/output \
		$(IMAGE):latest

helm-lint:
	helm lint helm-chart/
