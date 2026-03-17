-- Clear edges first (they'll be rebuilt by the fetch-waypoints job)
TRUNCATE waypoint_edges;

-- Drop FK constraints on waypoint_edges that reference waypoints(name)
-- (PostgreSQL requires the referenced column to have a UNIQUE constraint)
ALTER TABLE waypoint_edges DROP CONSTRAINT waypoint_edges_from_name_fkey;
ALTER TABLE waypoint_edges DROP CONSTRAINT waypoint_edges_to_name_fkey;

-- Drop the single-name uniqueness
ALTER TABLE waypoints DROP CONSTRAINT waypoints_name_key;

-- Add expression-based unique index: one row per (name, location) rounded to ~11m precision
-- This becomes the new ON CONFLICT target for upserts
CREATE UNIQUE INDEX waypoints_name_loc_unique
    ON waypoints (name, ROUND(latitude::numeric, 4), ROUND(longitude::numeric, 4));
