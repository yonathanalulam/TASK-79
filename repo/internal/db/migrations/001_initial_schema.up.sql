-- FleetCommerce Operations Hub - Full Schema

-- ============================================================
-- RBAC: users, roles, permissions
-- ============================================================

CREATE TABLE roles (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE permissions (
    id          SERIAL PRIMARY KEY,
    code        VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE role_permissions (
    role_id       INT NOT NULL REFERENCES roles(id),
    permission_id INT NOT NULL REFERENCES permissions(id),
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE users (
    id             SERIAL PRIMARY KEY,
    username       VARCHAR(100) UNIQUE NOT NULL,
    password_hash  TEXT NOT NULL,
    full_name      VARCHAR(255) NOT NULL,
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at  TIMESTAMPTZ
);

CREATE TABLE user_roles (
    user_id INT NOT NULL REFERENCES users(id),
    role_id INT NOT NULL REFERENCES roles(id),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE sessions (
    id         VARCHAR(64) PRIMARY KEY,
    user_id    INT NOT NULL REFERENCES users(id),
    data       JSONB,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- ============================================================
-- CATALOG: brands, series, vehicle models, versions, media
-- ============================================================

CREATE TABLE brands (
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE series (
    id         SERIAL PRIMARY KEY,
    brand_id   INT NOT NULL REFERENCES brands(id),
    name       VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(brand_id, name)
);

CREATE TABLE vehicle_models (
    id                  SERIAL PRIMARY KEY,
    brand_id            INT NOT NULL REFERENCES brands(id),
    series_id           INT NOT NULL REFERENCES series(id),
    model_code          VARCHAR(50) UNIQUE NOT NULL,
    model_name          VARCHAR(255) NOT NULL,
    year                INT NOT NULL CHECK (year >= 1900 AND year <= 2100),
    description         TEXT,
    publication_status  VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (publication_status IN ('draft', 'published', 'unpublished')),
    stock_quantity      INT NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),
    expiry_date         DATE,
    discontinued_at     TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vehicle_models_brand ON vehicle_models(brand_id);
CREATE INDEX idx_vehicle_models_series ON vehicle_models(series_id);
CREATE INDEX idx_vehicle_models_status ON vehicle_models(publication_status);

CREATE TABLE vehicle_model_versions (
    id                  SERIAL PRIMARY KEY,
    vehicle_model_id    INT NOT NULL REFERENCES vehicle_models(id),
    version_number      INT NOT NULL,
    model_name          VARCHAR(255) NOT NULL,
    year                INT NOT NULL,
    description         TEXT,
    stock_quantity      INT NOT NULL DEFAULT 0,
    expiry_date         DATE,
    status              VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    is_current_draft    BOOLEAN NOT NULL DEFAULT FALSE,
    is_current_pub      BOOLEAN NOT NULL DEFAULT FALSE,
    created_by          INT REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(vehicle_model_id, version_number)
);

CREATE TABLE vehicle_media (
    id                  SERIAL PRIMARY KEY,
    vehicle_model_id    INT NOT NULL REFERENCES vehicle_models(id),
    kind                VARCHAR(10) NOT NULL CHECK (kind IN ('image', 'video')),
    original_filename   VARCHAR(500) NOT NULL,
    stored_path         VARCHAR(1000) NOT NULL,
    mime_type           VARCHAR(100) NOT NULL,
    size_bytes          BIGINT NOT NULL,
    sha256_fingerprint  VARCHAR(64) NOT NULL,
    uploaded_by         INT REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vehicle_media_model ON vehicle_media(vehicle_model_id);

-- ============================================================
-- CSV IMPORTS
-- ============================================================

CREATE TABLE csv_import_jobs (
    id            SERIAL PRIMARY KEY,
    filename      VARCHAR(500) NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'validated', 'committed', 'failed')),
    total_rows    INT NOT NULL DEFAULT 0,
    valid_rows    INT NOT NULL DEFAULT 0,
    invalid_rows  INT NOT NULL DEFAULT 0,
    committed_rows INT NOT NULL DEFAULT 0,
    uploaded_by   INT REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ
);

CREATE TABLE csv_import_rows (
    id            SERIAL PRIMARY KEY,
    job_id        INT NOT NULL REFERENCES csv_import_jobs(id),
    row_number    INT NOT NULL,
    raw_data      JSONB NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'valid', 'invalid', 'committed')),
    errors        JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_csv_import_rows_job ON csv_import_rows(job_id);

-- ============================================================
-- CUSTOMERS & CARTS
-- ============================================================

CREATE TABLE customer_accounts (
    id                       SERIAL PRIMARY KEY,
    account_code             VARCHAR(50) UNIQUE NOT NULL,
    account_name             VARCHAR(255) NOT NULL,
    contact_phone_encrypted  BYTEA,
    contact_phone_nonce      BYTEA,
    contact_phone_masked     VARCHAR(20),
    location                 VARCHAR(255),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE carts (
    id                  SERIAL PRIMARY KEY,
    customer_account_id INT REFERENCES customer_accounts(id),
    status              VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'submitted', 'converted', 'abandoned')),
    created_by          INT REFERENCES users(id),
    updated_by          INT REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_carts_customer ON carts(customer_account_id);
CREATE INDEX idx_carts_status ON carts(status);

CREATE TABLE cart_items (
    id                  SERIAL PRIMARY KEY,
    cart_id             INT NOT NULL REFERENCES carts(id),
    vehicle_model_id    INT NOT NULL REFERENCES vehicle_models(id),
    quantity            INT NOT NULL CHECK (quantity > 0),
    unit_price_snapshot NUMERIC(12,2),
    validity_status     VARCHAR(20) NOT NULL DEFAULT 'valid' CHECK (validity_status IN ('valid', 'discontinued', 'unpublished', 'out_of_stock')),
    validation_message  TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cart_items_cart ON cart_items(cart_id);

CREATE TABLE cart_events (
    id          SERIAL PRIMARY KEY,
    cart_id     INT NOT NULL REFERENCES carts(id),
    event_type  VARCHAR(50) NOT NULL,
    details     JSONB,
    actor_id    INT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- ORDERS
-- ============================================================

CREATE TABLE orders (
    id                    SERIAL PRIMARY KEY,
    order_number          VARCHAR(50) UNIQUE NOT NULL,
    customer_account_id   INT REFERENCES customer_accounts(id),
    source_cart_id        INT REFERENCES carts(id),
    status                VARCHAR(30) NOT NULL DEFAULT 'created' CHECK (status IN (
        'created', 'payment_recorded', 'cutoff', 'picking', 'arrival',
        'pickup', 'delivery', 'completed', 'cancelled', 'partially_backordered', 'split'
    )),
    promised_date         DATE,
    location              VARCHAR(255),
    created_by            INT REFERENCES users(id),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cutoff_at             TIMESTAMPTZ,
    payment_recorded_at   TIMESTAMPTZ,
    split_parent_order_id INT REFERENCES orders(id)
);

CREATE INDEX idx_orders_customer ON orders(customer_account_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_number ON orders(order_number);

CREATE TABLE order_lines (
    id                     SERIAL PRIMARY KEY,
    order_id               INT NOT NULL REFERENCES orders(id),
    vehicle_model_id       INT NOT NULL REFERENCES vehicle_models(id),
    quantity_requested     INT NOT NULL CHECK (quantity_requested > 0),
    quantity_allocated     INT NOT NULL DEFAULT 0,
    quantity_backordered   INT NOT NULL DEFAULT 0,
    line_status            VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (line_status IN (
        'pending', 'allocated', 'backordered', 'fulfilled', 'cancelled'
    )),
    stock_snapshot         INT,
    publication_snapshot   VARCHAR(20),
    discontinued_snapshot  BOOLEAN DEFAULT FALSE,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_lines_order ON order_lines(order_id);

CREATE TABLE order_notes (
    id         SERIAL PRIMARY KEY,
    order_id   INT NOT NULL REFERENCES orders(id),
    note_type  VARCHAR(20) NOT NULL CHECK (note_type IN ('internal', 'picking', 'delivery')),
    content    TEXT NOT NULL CHECK (LENGTH(content) <= 2000),
    author_id  INT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_notes_order ON order_notes(order_id);

CREATE TABLE order_state_history (
    id             SERIAL PRIMARY KEY,
    order_id       INT NOT NULL REFERENCES orders(id),
    from_status    VARCHAR(30),
    to_status      VARCHAR(30) NOT NULL,
    actor_id       INT REFERENCES users(id),
    actor_type     VARCHAR(20) NOT NULL DEFAULT 'user' CHECK (actor_type IN ('user', 'system')),
    reason         TEXT,
    metadata       JSONB,
    transitioned_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_state_history_order ON order_state_history(order_id);

CREATE TABLE payment_records (
    id          SERIAL PRIMARY KEY,
    order_id    INT NOT NULL REFERENCES orders(id),
    amount      NUMERIC(12,2),
    method      VARCHAR(50),
    recorded_by INT REFERENCES users(id),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE order_events (
    id          SERIAL PRIMARY KEY,
    order_id    INT NOT NULL REFERENCES orders(id),
    event_type  VARCHAR(50) NOT NULL,
    details     JSONB,
    actor_id    INT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- NOTIFICATIONS
-- ============================================================

CREATE TABLE notification_templates (
    id          SERIAL PRIMARY KEY,
    code        VARCHAR(100) UNIQUE NOT NULL,
    name        VARCHAR(255) NOT NULL,
    subject     TEXT NOT NULL,
    body        TEXT NOT NULL,
    variables   JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notifications (
    id            SERIAL PRIMARY KEY,
    type          VARCHAR(50) NOT NULL,
    title         VARCHAR(500) NOT NULL,
    body          TEXT,
    entity_type   VARCHAR(50),
    entity_id     INT,
    created_by    INT REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notification_recipients (
    id              SERIAL PRIMARY KEY,
    notification_id INT NOT NULL REFERENCES notifications(id),
    user_id         INT NOT NULL REFERENCES users(id),
    is_read         BOOLEAN NOT NULL DEFAULT FALSE,
    read_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_recipients_user ON notification_recipients(user_id);
CREATE INDEX idx_notification_recipients_unread ON notification_recipients(user_id, is_read) WHERE is_read = FALSE;

CREATE TABLE announcements (
    id          SERIAL PRIMARY KEY,
    title       VARCHAR(500) NOT NULL,
    body        TEXT NOT NULL,
    priority    VARCHAR(20) NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'critical')),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  INT REFERENCES users(id),
    starts_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notification_preferences (
    id          SERIAL PRIMARY KEY,
    user_id     INT NOT NULL REFERENCES users(id),
    channel     VARCHAR(20) NOT NULL CHECK (channel IN ('in_app', 'email', 'sms', 'webhook')),
    event_type  VARCHAR(50) NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE(user_id, channel, event_type)
);

CREATE TABLE export_queue_items (
    id              SERIAL PRIMARY KEY,
    channel         VARCHAR(20) NOT NULL CHECK (channel IN ('email', 'sms', 'webhook')),
    recipient       VARCHAR(500) NOT NULL,
    subject         VARCHAR(500),
    body            TEXT NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'exported', 'retrying', 'failed', 'cancelled')),
    attempts        INT NOT NULL DEFAULT 0,
    max_attempts    INT NOT NULL DEFAULT 3,
    last_attempt_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_export_queue_status ON export_queue_items(status);

CREATE TABLE export_attempt_logs (
    id              SERIAL PRIMARY KEY,
    queue_item_id   INT NOT NULL REFERENCES export_queue_items(id),
    attempt_number  INT NOT NULL,
    status          VARCHAR(20) NOT NULL,
    error_message   TEXT,
    exported_path   VARCHAR(1000),
    attempted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- ALERTS
-- ============================================================

CREATE TABLE alert_rules (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    rule_type   VARCHAR(50) NOT NULL,
    condition   JSONB NOT NULL,
    severity    VARCHAR(20) NOT NULL DEFAULT 'warning' CHECK (severity IN ('info', 'warning', 'critical')),
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE alerts (
    id               SERIAL PRIMARY KEY,
    alert_rule_id    INT NOT NULL REFERENCES alert_rules(id),
    entity_type      VARCHAR(50) NOT NULL,
    entity_id        INT NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'claimed', 'processing', 'closed')),
    severity         VARCHAR(20) NOT NULL DEFAULT 'warning',
    title            VARCHAR(500) NOT NULL,
    details          JSONB,
    claimed_by       INT REFERENCES users(id),
    claimed_at       TIMESTAMPTZ,
    resolution_notes TEXT,
    closed_at        TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(alert_rule_id, entity_type, entity_id, status)
);

CREATE INDEX idx_alerts_status ON alerts(status);
CREATE INDEX idx_alerts_entity ON alerts(entity_type, entity_id);

CREATE TABLE alert_events (
    id          SERIAL PRIMARY KEY,
    alert_id    INT NOT NULL REFERENCES alerts(id),
    event_type  VARCHAR(50) NOT NULL,
    from_status VARCHAR(20),
    to_status   VARCHAR(20),
    actor_id    INT REFERENCES users(id),
    details     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- METRIC FRAMEWORK
-- ============================================================

CREATE TABLE metric_definitions (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    status      VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'pending_review', 'active', 'deprecated')),
    owner_id    INT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE metric_definition_versions (
    id                  SERIAL PRIMARY KEY,
    metric_id           INT NOT NULL REFERENCES metric_definitions(id),
    version_number      INT NOT NULL,
    sql_expression      TEXT,
    semantic_formula    TEXT,
    time_grain          VARCHAR(20),
    description         TEXT,
    is_derived          BOOLEAN NOT NULL DEFAULT FALSE,
    window_calculation  TEXT,
    status              VARCHAR(20) NOT NULL DEFAULT 'draft',
    created_by          INT REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(metric_id, version_number)
);

CREATE TABLE metric_dimensions (
    id          SERIAL PRIMARY KEY,
    metric_id   INT NOT NULL REFERENCES metric_definitions(id),
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    UNIQUE(metric_id, name)
);

CREATE TABLE metric_filters (
    id          SERIAL PRIMARY KEY,
    metric_id   INT NOT NULL REFERENCES metric_definitions(id),
    name        VARCHAR(100) NOT NULL,
    expression  TEXT NOT NULL
);

CREATE TABLE metric_dependencies (
    id                SERIAL PRIMARY KEY,
    metric_id         INT NOT NULL REFERENCES metric_definitions(id),
    depends_on_metric INT NOT NULL REFERENCES metric_definitions(id),
    CHECK (metric_id != depends_on_metric)
);

CREATE TABLE metric_lineage_edges (
    id          SERIAL PRIMARY KEY,
    source_type VARCHAR(20) NOT NULL CHECK (source_type IN ('metric', 'chart')),
    source_id   INT NOT NULL,
    target_type VARCHAR(20) NOT NULL CHECK (target_type IN ('metric', 'chart')),
    target_id   INT NOT NULL
);

CREATE TABLE metric_activation_reviews (
    id              SERIAL PRIMARY KEY,
    metric_id       INT NOT NULL REFERENCES metric_definitions(id),
    version_id      INT NOT NULL REFERENCES metric_definition_versions(id),
    impact_summary  JSONB,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by     INT REFERENCES users(id),
    reviewed_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE metric_permissions (
    id          SERIAL PRIMARY KEY,
    metric_id   INT NOT NULL REFERENCES metric_definitions(id),
    user_id     INT REFERENCES users(id),
    role_id     INT REFERENCES roles(id),
    can_view    BOOLEAN NOT NULL DEFAULT TRUE,
    can_activate BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE(metric_id, user_id, role_id)
);

CREATE TABLE charts (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    chart_type  VARCHAR(50),
    config      JSONB,
    created_by  INT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE chart_metric_dependencies (
    id        SERIAL PRIMARY KEY,
    chart_id  INT NOT NULL REFERENCES charts(id),
    metric_id INT NOT NULL REFERENCES metric_definitions(id),
    UNIQUE(chart_id, metric_id)
);

-- ============================================================
-- AUDIT LOG
-- ============================================================

CREATE TABLE audit_log (
    id              SERIAL PRIMARY KEY,
    entity_type     VARCHAR(50) NOT NULL,
    entity_id       INT,
    action          VARCHAR(50) NOT NULL,
    actor_user_id   INT REFERENCES users(id),
    actor_role      VARCHAR(50),
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    before_json     JSONB,
    after_json      JSONB,
    metadata_json   JSONB,
    request_id      VARCHAR(100),
    ip_address      VARCHAR(45)
);

CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_log_occurred ON audit_log(occurred_at);
CREATE INDEX idx_audit_log_actor ON audit_log(actor_user_id);
