# syntax=docker/dockerfile:1
# Image unique contenant les binaires api, migrate et worker.

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN for cmd in api migrate worker; do \
      CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/$cmd ./cmd/$cmd || exit 1; \
    done

# Distroless : ni shell ni gestionnaire de paquets, utilisateur non root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /usr/local/bin/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
