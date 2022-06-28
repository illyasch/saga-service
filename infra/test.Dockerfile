# Build the Go Binary.
FROM golang:1.18 as build_saga
ENV CGO_ENABLED 0
ARG BUILD_REF

# Create the saga directory and the copy the module files first and then
# download the dependencies.
RUN mkdir /saga
COPY go.* /saga/
WORKDIR /saga
RUN go mod download

# Copy the source code into the container.
COPY . /saga

# Run tests.
WORKDIR /saga
CMD CGO_ENABLED=1 go test -count=1 -v ./...
