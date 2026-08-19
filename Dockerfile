FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-w -s -X main.version=${VERSION}" -o aws-health-exporter .

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=builder /app/aws-health-exporter /bin/aws-health-exporter

USER nonroot:nonroot

ENTRYPOINT ["/bin/aws-health-exporter"]
