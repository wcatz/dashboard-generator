FROM golang:1.24-bookworm AS builder

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE=unknown

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags "-X main.version=${VERSION} -X main.commitSHA=${COMMIT_SHA} -X main.buildDate=${BUILD_DATE}" \
    -o dashboard-generator ./cmd/dashboard-generator

FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="dashboard-generator" \
      org.opencontainers.image.description="Config-driven Grafana dashboard generator with web UI" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="https://github.com/wcatz/dashboard-generator"

COPY --from=builder /app/dashboard-generator /dashboard-generator

EXPOSE 8080

ENTRYPOINT ["/dashboard-generator"]
CMD ["serve", "--config", "/data/config.yaml", "--port", "8080"]
