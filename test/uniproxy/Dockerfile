FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o uniproxy ./main.go

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/uniproxy /usr/local/bin/uniproxy
EXPOSE 8080
ENTRYPOINT ["uniproxy"]
CMD ["-config", "/etc/uniproxy/config.yaml"]
