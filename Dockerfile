# arthik — two-stage build: tests + static binary, then a tiny runtime image.
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go vet ./... && go test ./... \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/arthik ./cmd/arthik

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/arthik /usr/local/bin/arthik
# Data folder (bind-mounted from the host); the container runs as PUID:PGID from .env.
VOLUME ["/data/finance"]
ENV ARTHIK_DATA=/data/finance ARTHIK_LISTEN=:8080
EXPOSE 8080
HEALTHCHECK --interval=60s --timeout=5s --start-period=10s CMD ["arthik", "health"]
ENTRYPOINT ["arthik"]
CMD ["serve"]
