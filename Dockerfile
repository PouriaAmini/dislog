# Use the official Golang 1.19-alpine image as the base image
FROM golang:1.19-alpine AS build

# Set the working directory to /go/src/dislog
WORKDIR /go/src/dislog

# Copy the current directory into the container at /go/src/dislog
COPY . .

# Build the Go binary with CGO disabled and output it to /go/bin/dislog
RUN CGO_ENABLED=0 go build -o /go/bin/dislog ./cmd/dislog

# Install the grpc_health_probe executable in the image
RUN GRPC_HEALTH_PROBE_VERSION=v0.3.2 && \
    wget -qO/go/bin/grpc_health_probe \
    https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/\
    ${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /go/bin/grpc_health_probe

# Use a scratch image as the base image to create a minimal container image
FROM scratch

# Copy the binary from the build image to the new image
COPY --from=build /go/bin/dislog /bin/dislog

# Copy the binary from the build image for health prop to the new image
COPY --from=build /go/bin/grpc_health_probe /bin/grpc_health_probe

# Set the entry point of the container to the binary
ENTRYPOINT ["/bin/dislog"]