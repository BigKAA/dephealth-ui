# Stage 1 — frontend build
ARG REGISTRY=docker.io
FROM ${REGISTRY}/node:22-alpine AS frontend
WORKDIR /frontend
COPY frontend/ ./
RUN if [ -f package-lock.json ]; then npm ci; elif [ -f package.json ]; then npm install; fi
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
RUN apk --no-cache add ca-certificates graphviz && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup
COPY --from=builder /dephealth-ui /dephealth-ui
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/dephealth-ui"]
