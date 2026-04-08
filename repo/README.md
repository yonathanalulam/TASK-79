# FleetCommerce Operations Hub

An offline-first dealership parts and vehicle catalog workflow system built with Go, Gin, PostgreSQL, and HTMX.

## Architecture

```
cmd/server/          - Main server entry point
cmd/seed/            - Database seed tool (generates proper password hashes)
internal/
  app/               - Configuration
  auth/              - Authentication (sessions, login/logout)
  rbac/              - Role-based access control (permission matrix)
  catalog/           - Vehicle catalog (brands, series, models, versions)
  media/             - Media upload handling
  cart/              - Shopping carts, items, validation, merge
  orders/            - Orders, state machine, notes, split/backorder
  notifications/     - Notification center, templates, export queues
  alerts/            - Inventory alerts, rules, lifecycle
  metrics/           - Metric framework, versioning, impact analysis, lineage
  audit/             - Immutable audit logging
  imports/           - CSV import/export with two-phase commit
  scheduler/         - Background jobs (cutoff, alerts, export retry)
  crypto/            - Argon2id hashing, AES-GCM encryption
  masking/           - Phone/email masking
  db/                - Database connection, embedded migrations
  http/
    handlers/        - HTTP handlers (REST + HTML pages)
    middleware/      - Auth, RBAC, CSRF middleware
    views/           - Template renderer + embedded HTML templates
web/
  static/css/        - Stylesheet
  uploads/           - Uploaded media files
  exports/           - Export queue output
migrations/          - (embedded in internal/db/migrations)
testdata/            - Sample CSV files
```

## Quick Start

```bash
docker compose up
```

That's it. This single command:
- Starts PostgreSQL 16
- Builds the Go application
- Runs database migrations
- Seeds demo data (users, roles, catalog, alert rules, notification templates)
- Hashes demo passwords with Argon2id
- Encrypts sample phone numbers with AES-GCM
- Starts the background scheduler (order cutoff, alert evaluation, export retry)
- Serves the application on **http://localhost:8080**

### Prerequisites

- Docker and Docker Compose

### Manual Setup (without Docker)

If you prefer running without Docker:

1. Install Go 1.22+ and PostgreSQL 14+
2. Create database:
   ```bash
   createdb fleetcommerce
   createuser fleet -P  # password: fleet
   psql -c "GRANT ALL PRIVILEGES ON DATABASE fleetcommerce TO fleet;"
   ```
3. Start the server (auto-migrates and seeds):
   ```bash
   go run ./cmd/server
   ```
4. Open http://localhost:8080

## Demo Credentials

| Username    | Password      | Role              |
|-------------|---------------|-------------------|
| admin       | password123   | Administrator     |
| inventory   | password123   | Inventory Manager |
| sales       | password123   | Sales Associate   |
| auditor     | password123   | Auditor           |

Password hashes and phone encryption are applied automatically on startup.

## Configuration

| Env Variable           | Default                     | Description                    |
|------------------------|-----------------------------|--------------------------------|
| APP_PORT               | 8080                        | Server port                    |
| DATABASE_URL           | postgres://fleet:fleet@...  | PostgreSQL connection string   |
| SESSION_SECRET         | change-me...                | Session cookie signing key     |
| ENCRYPTION_KEY         | 0123456789abcdef...         | AES-GCM key for encryption     |
| UPLOADS_DIR            | ./web/uploads               | File upload directory          |
| EXPORTS_DIR            | ./web/exports               | Export queue output directory   |
| MAX_UPLOAD_BYTES       | 26214400 (25MB)             | Max upload file size           |
| SCHEDULER_ENABLED      | true                        | Enable background scheduler    |
| CUTOFF_INTERVAL_SEC    | 60                          | Order cutoff check interval    |
| ALERT_INTERVAL_SEC     | 900                         | Alert evaluation interval      |
| EXPORT_RETRY_INTERVAL_SEC | 300                      | Export retry interval          |

## Key Features

### Vehicle Catalog
- Brand > Series > Model hierarchical browsing
- Draft/publish/unpublish workflow with versioned content
- Media upload with MIME validation, size limits, SHA-256 fingerprinting
- CSV import with two-phase preview/commit and row-level validation
- CSV export

### Cart Management
- Create carts per customer account
- Add/remove/update line items
- Automatic validation against catalog state (discontinued, unpublished, out of stock)
- Cart merge by customer account with deduplication
- Checkout converts cart to order

### Order Management
- Server-enforced state machine: created -> payment_recorded -> picking -> arrival -> pickup/delivery -> completed
- Automatic cutoff at 30 minutes if payment not recorded
- Order notes (internal, picking, delivery)
- Order split for backorder handling
- Visual timeline of state transitions
- Payment recording with permission gate

### Notification Center
- In-app notifications with read/unread state
- Bulk mark-as-read
- System announcements with priority levels
- User subscription preferences
- Template rendering with variable substitution
- Local export queues (email/SMS/webhook simulation)

### Inventory Alerts
- Configurable alert rules (low stock, overstock, near expiry)
- Scheduled evaluation every 15 minutes
- Claim/process/close lifecycle
- Mandatory resolution notes on close

### Metric Framework
- Versioned metric definitions
- Dependency tracking between metrics
- Impact analysis before activation
- Lineage visualization (metric -> metric, chart -> metric)
- Metric-level permissions

### Security
- Argon2id password hashing
- AES-GCM encryption at rest for sensitive fields
- Phone number masking in UI
- CSRF protection on forms
- Role-based permission matrix
- Session-based authentication

### Audit Trail
- Immutable append-only audit log
- Captures entity type, ID, action, actor, before/after JSON, metadata
- Covers: catalog mutations, cart operations, order transitions, alerts, metrics, imports

## Running Tests

```bash
go test ./...
```

## Known Simplifications

- "Email/SMS/webhook" exports are stored locally, not actually sent
- Metric framework stores definitions and governance metadata, not a full BI engine
- Template rendering uses simple string replacement, not full Go templates
- File uploads stored on local filesystem
- Single-server architecture (no distributed scheduling)
- Session cleanup runs as part of the scheduler, not a separate service
