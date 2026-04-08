CREATE TABLE announcement_reads (
    announcement_id INT NOT NULL REFERENCES announcements(id),
    user_id         INT NOT NULL REFERENCES users(id),
    read_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (announcement_id, user_id)
);
CREATE INDEX idx_announcement_reads_user ON announcement_reads(user_id);
