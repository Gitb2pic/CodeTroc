# syntax=docker/dockerfile:1
# Image unique contenant les binaires api et migrate.

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# Distroless : ni shell ni gestionnaire de paquets, utilisateur non root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /out/migrate /usr/local/bin/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
