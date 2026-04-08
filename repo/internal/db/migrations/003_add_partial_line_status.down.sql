ALTER TABLE order_lines DROP CONSTRAINT IF EXISTS order_lines_line_status_check;
ALTER TABLE order_lines ADD CONSTRAINT order_lines_line_status_check
    CHECK (line_status IN ('pending', 'allocated', 'backordered', 'fulfilled', 'cancelled'));

DROP RULE IF EXISTS audit_log_no_delete ON audit_log;
DROP RULE IF EXISTS audit_log_no_update ON audit_log;
