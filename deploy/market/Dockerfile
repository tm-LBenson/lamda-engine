FROM golang:1.25.0-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/lamda-market-server ./cmd/lamda-market-server
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -g 10001 market && adduser -D -u 10001 -G market market && mkdir /data && chown market:market /data
COPY --from=build /out/lamda-market-server /usr/local/bin/lamda-market-server
COPY --from=build /src/THIRD-PARTY-NOTICES.txt /usr/share/lamdaui/THIRD-PARTY-NOTICES.txt
USER 10001:10001
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/lamda-market-server"]
