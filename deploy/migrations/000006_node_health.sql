-- M4: health-monitor expiry scan and repair lookups.
-- docs/03-data-model.md (node registry), docs/06-scheduler-and-repair.md.

CREATE INDEX IF NOT EXISTS nodes_status_last_seen
    ON nodes (status, last_seen_at)
    WHERE status IN ('online', 'suspect');

CREATE INDEX IF NOT EXISTS placements_lost_by_node
    ON fragment_placements (node_id, status)
    WHERE status = 'lost';
