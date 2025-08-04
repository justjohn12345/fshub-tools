# Build stage
# This stage builds the Go binary. It assumes the build environment
# has the same architecture as the target environment (e.g., x86).
FROM golang:1.23-alpine AS builder

# Install C build tools and SQLite development libraries for CGo
RUN apk add --no-cache build-base sqlite-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the Go application. CGo is enabled by default.
RUN go build -o main .

# Final stage
# This stage creates the final, minimal image.
FROM alpine:latest

# Install the runtime SQLite library required by the compiled binary
RUN apk add --no-cache sqlite-libs

WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /app/main .
# Copy the static web assets
COPY --from=builder /app/static ./static

EXPOSE 80
EXPOSE 443
ENTRYPOINT ["./main"]
