# Stage 1: Build the Go application
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./ 
RUN go mod download

# Copy all code and build as a binary named main
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/api/main.go

# Stage 2: Create a minimal runtime image
FROM alpine:latest

# Install ffmpeg, ca-certificates, and tzdata for timezone support (Important for proper time handling)
RUN apk update && apk add --no-cache ffmpeg ca-certificates tzdata

WORKDIR /root/

# Copy the built binary from the builder stage
COPY --from=builder /app/main .

# Create directory for temporary file storage as configured
RUN mkdir -p /root/tmp/downloads

EXPOSE 8080
CMD ["./main"]