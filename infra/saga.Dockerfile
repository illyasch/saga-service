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

# Build the saga binary. We are doing this last since this will be different
# every time we run through this process.
WORKDIR /saga/cmd/saga-service
RUN go build -ldflags "-X main.build=${BUILD_REF}"

# Run the Go Binary in Alpine.
FROM alpine:3.15
ARG BUILD_DATE
ARG BUILD_REF
RUN addgroup -g 1000 -S saga && \
    adduser -u 1000 -h /saga -G saga -S saga
COPY --from=build_saga --chown=saga:saga /saga/cmd/saga-service/saga-service /saga/saga-service
WORKDIR /saga
USER saga
CMD ["./saga-service"]

LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.title="url-saga" \
      org.opencontainers.image.authors="Ilya Scheblanov <ilya.scheblanov@gmail.com>" \
      org.opencontainers.image.source="https://github.com/illyasch/saga-service/cmd/saga-service" \
      org.opencontainers.image.revision="${BUILD_REF}"
