CREATE TABLE waypoints (
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(10)      NOT NULL UNIQUE,
    latitude   DOUBLE PRECISION NOT NULL,
    longitude  DOUBLE PRECISION NOT NULL,
    location   GEOGRAPHY(POINT, 4326),
    created_at TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);
