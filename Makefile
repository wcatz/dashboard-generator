BINARY := dashboard-generator
PKG := github.com/wcatz/dashboard-generator
CMD := ./cmd/dashboard-generator
IMAGE := wcatz/dashboard-generator

.PHONY: build test lint clean run-dry docker-build docker-run helm-lint

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

docker-build:
	docker build -t $(IMAGE):latest .

docker-run:
	docker run --rm -p 8080:8080 \
		-v $(PWD)/example-config.yaml:/data/config.yaml:ro \
		-v $(PWD)/output:/data/output \
		$(IMAGE):latest

helm-lint:
	helm lint helm-chart/
