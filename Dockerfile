FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

WORKDIR /app

COPY --from=build /out/server /app/server
RUN mkdir -p /app/history && chown -R app /app

USER app
EXPOSE 8080

# Secrets and the ISIN map stay outside the image, mount them at /app:
#   credentials.json, isin_map.json, and a volume for history/
ENTRYPOINT ["/app/server"]
