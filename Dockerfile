FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/qrcode ./cmd/server

FROM alpine:3.22
RUN addgroup -S qrcode && adduser -S -G qrcode qrcode
COPY --from=build /out/qrcode /qrcode
USER qrcode:qrcode
EXPOSE 8080
ENV QR_ADDR=:8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz || exit 1
ENTRYPOINT ["/qrcode"]
