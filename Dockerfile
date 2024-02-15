# Use the official Golang image to create a build artifact.
FROM golang:1.22 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the command inside the container.
# The binary will be named 'app'
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

# Use alpine:latest for a lean production container.
FROM alpine:latest

# Add ca-certificates in case need them to access a TLS endpoint
RUN apk --no-cache add ca-certificates

# Set the working directory to /root/
WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/app .

# Expose port 9999 to the outside world
EXPOSE 9999

# Command to run the executable
CMD ["./app"]