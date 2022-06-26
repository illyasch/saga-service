#!/usr/bin/env sh
set -e

echo "Creating SQS queues..."
waitforit -timeout 60 -address ${SQS_ENDPOINT_URL}
sleep 3
aws --endpoint-url=${SQS_ENDPOINT_URL} \
    sqs create-queue \
        --queue-name commands1 \
        --region ${AWS_REGION}

sleep 3
aws --endpoint-url=${SQS_ENDPOINT_URL} \
    sqs create-queue \
        --queue-name commands2 \
        --region ${AWS_REGION}

sleep 3
aws --endpoint-url=${SQS_ENDPOINT_URL} \
    sqs create-queue \
        --queue-name commands3 \
        --region ${AWS_REGION}

sleep 3
aws --endpoint-url=${SQS_ENDPOINT_URL} \
    sqs create-queue \
        --queue-name responses \
        --region ${AWS_REGION}
