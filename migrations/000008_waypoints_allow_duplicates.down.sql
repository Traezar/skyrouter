TRUNCATE waypoint_edges;

DROP INDEX waypoints_name_loc_unique;

-- Remove duplicate rows, keeping the most recently updated per name
DELETE FROM waypoints
WHERE id NOT IN (
    SELECT DISTINCT ON (name) id FROM waypoints ORDER BY name, updated_at DESC
);

ALTER TABLE waypoints ADD CONSTRAINT waypoints_name_key UNIQUE (name);

ALTER TABLE waypoint_edges
    ADD CONSTRAINT waypoint_edges_from_name_fkey FOREIGN KEY (from_name) REFERENCES waypoints(name),
    ADD CONSTRAINT waypoint_edges_to_name_fkey   FOREIGN KEY (to_name)   REFERENCES waypoints(name);
