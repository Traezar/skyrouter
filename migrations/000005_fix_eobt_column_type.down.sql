ALTER TABLE flights
    ALTER COLUMN estimated_off_block_time TYPE TIME USING estimated_off_block_time::TIME;
