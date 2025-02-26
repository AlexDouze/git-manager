# Build stage
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/bin/gitm -ldflags="-w -s" .

# Final stage
FROM scratch

# Copy the binary from the builder stage
COPY --from=builder /app/bin/gitm /gitm

# Set the entrypoint
ENTRYPOINT ["/gitm"]
