-- Add 'partial' to order line status enum
-- Drop any existing check constraint on line_status (auto-named or explicit)
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT conname FROM pg_constraint
        WHERE conrelid = 'order_lines'::regclass
          AND pg_get_constraintdef(oid) LIKE '%line_status%'
    LOOP
        EXECUTE 'ALTER TABLE order_lines DROP CONSTRAINT ' || quote_ident(r.conname);
    END LOOP;
END$$;

ALTER TABLE order_lines ADD CONSTRAINT order_lines_line_status_check
    CHECK (line_status IN ('pending', 'allocated', 'backordered', 'partial', 'fulfilled', 'cancelled'));

-- Enforce append-only on audit_log via rules
CREATE OR REPLACE RULE audit_log_no_delete AS ON DELETE TO audit_log DO INSTEAD NOTHING;
CREATE OR REPLACE RULE audit_log_no_update AS ON UPDATE TO audit_log DO INSTEAD NOTHING;
