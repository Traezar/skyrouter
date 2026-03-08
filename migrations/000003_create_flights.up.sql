CREATE TABLE flights (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id                TEXT        NOT NULL UNIQUE,
    message_type             TEXT        NOT NULL,
    callsign                 TEXT        NOT NULL,
    flight_type              TEXT,
    operator                 TEXT,
    src                      TEXT,
    remark                   TEXT,

    -- aircraft
    aircraft_type            TEXT,
    wake_turbulence          TEXT,
    aircraft_registration    TEXT,
    aircraft_address         TEXT,
    aircraft_capabilities    JSONB,

    -- departure
    departure_aerodrome      TEXT        NOT NULL,
    date_of_flight           DATE        NOT NULL,
    estimated_off_block_time TIME,
    scheduled_departure_at   TIMESTAMPTZ,

    -- arrival
    destination_aerodrome    TEXT        NOT NULL,
    alternate_aerodromes     TEXT[],
    scheduled_arrival_at     TIMESTAMPTZ,

    -- enroute
    alternate_enroute        TEXT,
    mode_a_code              TEXT,

    -- gufi
    gufi                     TEXT,
    gufi_originator          TEXT,

    -- meta
    reception_time           TIMESTAMPTZ,
    last_updated_at          TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON flights (callsign);
CREATE INDEX ON flights (departure_aerodrome);
CREATE INDEX ON flights (destination_aerodrome);
CREATE INDEX ON flights (date_of_flight);

-- Each row is a complete snapshot of the route at a point in time.
-- Inserting a new version preserves the previous one as history.
CREATE TABLE flight_route_versions (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    flight_id          UUID        NOT NULL REFERENCES flights (id) ON DELETE CASCADE,
    version            INT         NOT NULL,
    flight_rules       TEXT,
    cruising_speed     TEXT,
    cruising_level     TEXT,
    route_text         TEXT,
    total_elapsed_time INTERVAL,
    fir_estimates      TEXT[],
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (flight_id, version)
);

CREATE INDEX ON flight_route_versions (flight_id);

CREATE TABLE flight_route_elements (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_version_id UUID NOT NULL REFERENCES flight_route_versions (id) ON DELETE CASCADE,
    seq_num          INT  NOT NULL,
    waypoint_name    TEXT NOT NULL,
    airway           TEXT,
    airway_type      TEXT,
    change_speed     TEXT,
    change_level     TEXT,
    UNIQUE (route_version_id, seq_num)
);

CREATE INDEX ON flight_route_elements (route_version_id);
