# saga-service

The service orchestrates work of other services using saga pattern.
```
Pattern: Saga

Maintain data consistency across services using a sequence of local transactions that are coordinated using asynchronous messaging.

```

## Overview

Sagas are a mechanism to maintain data consistency in a microservice architecture. 
You define a saga for each system command that needs to update data in multiple services. 
A saga is a sequence of local transactions. Each local transaction updates data within a single service.
The system operation initiates the first step of the saga. The completion of a local transaction triggers the execution of the next local transaction.

An important benefit of asynchronous messaging is that it ensures the all the steps of a saga are executed even if one or more of the saga’s participants is temporarily unavailable.

## HTTP handlers

The entrypoint to the code is in cmd/saga-service/saga-service.go. The service has the following HTTP handlers:

- _/start_ - use POST method and x-www-form-urlencoded parameter saga_id with a new saga ID in UUID format. 
Returns base62 code of the URL. 
- _/readiness_ - check if the database is ready and if not will return a 500 status if it's not.
- _/liveness_ - return simple status info if the service is alive.

## Prerequisites

- [Docker](https://www.docker.com/) and [docker-compose](https://docs.docker.com/compose/install/)
- [Git](https://git-scm.com/)
- [GNU Make](https://www.gnu.org/software/make/)
- [Go](https://golang.org/) (if you want to compile and run the service without docker)

## Installation

1. Clone this repository in the current directory:

   ```
   git clone https://github.com/illyasch/saga-service
   ```

2. Build Docker images:

   ```bash
   make image
   ```

3. Migrate and seed the database and queues (uses Docker):

   ```
   make init
   ```

3. Start the local development environment (uses Docker):

   ```
   make up
   ```

   At this point you should have the saga-service service running. To confirm the state of the running Docker container, run

   ```
   $ docker ps
   ```

## How to

### Run unit tests

from the docker container

```
make test
```

### Run manual tests

   Start a new saga
   ```
   $ curl -i --data-urlencode "saga_id=72639776-a13f-4c1b-b0c3-5feb2d525e4e" http://localhost:3000/start
   HTTP/1.1 200 OK
   Content-Type: application/json
   Date: Sun, 26 Jun 2022 10:28:02 GMT
   Content-Length: 4
   ```
