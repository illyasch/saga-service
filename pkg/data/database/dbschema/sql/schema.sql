-- Version: 1.1
-- Description: Create type SAGA_STATUS
CREATE TYPE SAGA_STATUS AS ENUM ('started', 'completed', 'error');

-- Version: 1.2
-- Description: Create table sagas
CREATE TABLE sagas (
    id UUID PRIMARY KEY,
    status SAGA_STATUS NOT NULL,
    service TEXT,
    date_created  TIMESTAMP
);
