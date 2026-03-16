CREATE TABLE waypoint_edges (
    from_name  VARCHAR(10)      NOT NULL REFERENCES waypoints(name),
    to_name    VARCHAR(10)      NOT NULL REFERENCES waypoints(name),
    distance_m DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (from_name, to_name)
);

CREATE INDEX waypoint_edges_from_idx ON waypoint_edges (from_name);
