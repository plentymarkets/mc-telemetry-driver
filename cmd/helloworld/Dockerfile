# Start from the latest golang base image
FROM golang:alpine as builder

# Add Maintainer Info
LABEL maintainer="Dein Name <email@plentymarkets.com>"

# Install Essentials
RUN apk update \
    && apk add -U --no-cache ca-certificates \
    && update-ca-certificates

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o helloworld ./cmd/helloworld/main.go

######## Start a new stage from latest alpine #######
FROM alpine:latest

# Copy the binary and ca certificate file from the previous stage
COPY --from=builder /app/helloworld .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
ENTRYPOINT ["./helloworld"]
