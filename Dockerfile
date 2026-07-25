# Stage 1: Build the binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Disable CGO for a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Stage 2: Run the binary
FROM alpine:latest
WORKDIR /root/

# Ensure the container can make secure HTTPS requests
RUN apk --no-cache add ca-certificates

# Bring in the compiled binary
COPY --from=builder /app/main .

# ENV must be in the final stage so the running app can see it
ENV GIN_MODE=release

EXPOSE 8080
CMD ["./main"]