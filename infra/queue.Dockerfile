FROM alpine:3.16.0

ARG SQS_ENDPOINT_URL
ARG AWS_REGION
ARG AWS_ACCESS_KEY_ID
ARG AWS_SECRET_ACCESS_KEY

ENV SQS_ENDPOINT_URL=${SQS_ENDPOINT_URL}
ENV AWS_REGION=${AWS_REGION}
ENV AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
ENV AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}

# Install common tools.
RUN apk add --no-cache \
    ca-certificates \
    curl \
    git \
    make \
    aws-cli

# Install waitforit. See https://github.com/maxcnunes/waitforit.
ARG WAITFORIT_VERSION
RUN set -ex ;\
    curl -LSs \
        -o /usr/local/bin/waitforit \
        "https://github.com/maxcnunes/waitforit/releases/download/v2.4.1/waitforit-linux_amd64" ; \
    chmod +x /usr/local/bin/waitforit

COPY infra/init-queue.sh /usr/local/bin/init-queue
RUN chmod +x /usr/local/bin/init-queue
