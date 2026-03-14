ALTER TABLE flights
    ALTER COLUMN estimated_off_block_time TYPE TIMESTAMPTZ USING NULL;
