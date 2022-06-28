# Build the Go Binary.
FROM golang:1.18 as build_stub
ENV CGO_ENABLED 0
ARG BUILD_REF

# Create the stub directory and the copy the module files first and then
# download the dependencies.
RUN mkdir /stub
COPY go.* /stub/
WORKDIR /stub
RUN go mod download

# Copy the source code into the container.
COPY . /stub

# Build the stub binary. We are doing this last since this will be different
# every time we run through this process.
WORKDIR /stub/cmd/queue-stub
RUN go build -ldflags "-X main.build=${BUILD_REF}"

# Run the Go Binary in Alpine.
FROM alpine:3.15
ARG BUILD_DATE
ARG BUILD_REF
RUN addgroup -g 1000 -S stub && \
    adduser -u 1000 -h /stub -G stub -S stub
COPY --from=build_stub --chown=stub:stub /stub/cmd/queue-stub/queue-stub /stub/queue-stub
WORKDIR /stub
USER stub
CMD ["./queue-stub"]

LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.title="queue-stub" \
      org.opencontainers.image.authors="Ilya Scheblanov <ilya.scheblanov@gmail.com>" \
      org.opencontainers.image.source="https://github.com/illyasch/saga-service/cmd/queue-stub" \
      org.opencontainers.image.revision="${BUILD_REF}"
