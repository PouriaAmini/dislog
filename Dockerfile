# Use the official Golang 1.19-alpine image as the base image
FROM golang:1.19-alpine AS build

# Set the working directory to /go/src/dislog
WORKDIR /go/src/dislog

# Copy the current directory into the container at /go/src/dislog
COPY . .

# Build the Go binary with CGO disabled and output it to /go/bin/dislog
RUN CGO_ENABLED=0 go build -o /go/bin/dislog ./cmd/dislog

# Use a scratch image as the base image to create a minimal container image
FROM scratch

# Copy the binary from the build image to the new image
COPY --from=build /go/bin/dislog /bin/dislog

# Set the entry point of the container to the binary
ENTRYPOINT ["/bin/dislog"]