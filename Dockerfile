# Dockerfile
# 1
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN if [ -f go.mod ]; then go mod download; fi
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sfpipeline .

# 2
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/sfpipeline .
RUN chown -R appuser:appgroup /app
USER appuser
ENTRYPOINT ["./sfpipeline"]