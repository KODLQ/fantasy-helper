FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/fantasy-helper ./cmd/server

FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/fantasy-helper /app/fantasy-helper
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --retries=15 CMD wget -q -O - http://localhost:8080/healthz >/dev/null || exit 1
ENTRYPOINT ["/app/fantasy-helper"]
