# Stage 1 — frontend build
ARG REGISTRY=docker.io
FROM ${REGISTRY}/node:22-alpine AS frontend
WORKDIR /frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2 — Go build
FROM ${REGISTRY}/golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=frontend /frontend/dist internal/server/static
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /dephealth-ui ./cmd/dephealth-ui

# Stage 3 — runtime
FROM ${REGISTRY}/alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /dephealth-ui /dephealth-ui
EXPOSE 8080
ENTRYPOINT ["/dephealth-ui"]
