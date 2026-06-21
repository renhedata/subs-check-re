-- node_ids restricts a check to a subset of the subscription's nodes.
-- NULL or empty array = check all nodes (the default / scheduled behavior).
ALTER TABLE check_jobs ADD COLUMN node_ids JSONB;
