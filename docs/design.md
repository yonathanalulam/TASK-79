# FleetCommerce Operations Hub - Design Document

## 1. System Overview

FleetCommerce Operations Hub is an offline-first dealership parts and vehicle catalog management system built as a monolithic Go web application. It provides a unified platform for managing vehicle content, building carts, placing orders, and staying informed through messaging and alerting—all without relying on external network services.

### Core Principles
- **Offline-First**: All data, files, and notification queues are local. No external API calls, cloud services, or third-party integrations at runtime.
- **Transactional Integrity**: PostgreSQL is the single system of record; all mutations occur within transactions.
- **Auditability**: Every state change writes an immutable audit trail with actor, timestamp, and before/after snapshots.
- **Role-Based Access**: Four predefined roles with 25 granular permissions enforced on every route.

---

## 2. Architecture

### 2.1 High-Level Architecture

```
┌──────────────────────────────────────────────────────────┐
│                      Web Browser                         │
│   (Templ HTML + HTMX + Static CSS/JS)                   │
└──────────────┬───────────────────────────────────────────┘
               │ HTTP (port 8080)
┌──────────────▼───────────────────────────────────────────┐
│                    Gin HTTP Router                        │
│  ┌─────────┐ ┌──────────┐ ┌───────────────────────┐     │
│  │  CSRF   │ │  Auth    │ │  Permission Middleware │     │
│  │Middleware│ │Middleware│ │  (per-route)           │     │
│  └─────────┘ └──────────┘ └───────────────────────┘     │
├──────────────────────────────────────────────────────────┤
│                    Handlers Layer                         │
│  (Page rendering + API JSON endpoints)                   │
├──────────────────────────────────────────────────────────┤
│                   Service Layer                           │
│  ┌────────┐ ┌───────┐ ┌──────┐ ┌───────┐ ┌──────────┐  │
│  │Catalog │ │ Cart  │ │Orders│ │Notifs │ │  Alerts  │  │
│  │Service │ │Service│ │Svc   │ │Service│ │  Service │  │
│  └────────┘ └───────┘ └──────┘ └───────┘ └──────────┘  │
│  ┌────────┐ ┌───────┐ ┌──────┐ ┌───────┐               │
│  │Metrics │ │Import │ │Audit │ │ RBAC  │               │
│  │Service │ │Service│ │Svc   │ │Service│               │
│  └────────┘ └───────┘ └──────┘ └───────┘               │
├──────────────────────────────────────────────────────────┤
│                  Repository Layer                         │
│  (SQL queries via pgx/v5 connection pool)                │
├──────────────────────────────────────────────────────────┤
│                    PostgreSQL                             │
│  (Transactional store, sessions, audit log)              │
├──────────────────────────────────────────────────────────┤
│               Local File System                          │
│  ./web/uploads (media)    ./web/exports (notification    │
│                            export queue files)           │
├──────────────────────────────────────────────────────────┤
│                   Scheduler                               │
│  ┌──────────────┐ ┌────────────┐ ┌────────────────┐     │
│  │Order Cutoff  │ │Alert Eval  │ │Export Retry    │     │
│  │(60s default) │ │(900s/15min)│ │(300s/5min)     │     │
│  └──────────────┘ └────────────┘ └────────────────┘     │
└──────────────────────────────────────────────────────────┘
```

### 2.2 Project Structure

```
repo/
├── cmd/
│   ├── seed/main.go              # Standalone seed utility
│   └── server/main.go            # Application entry point
├── internal/
│   ├── alerts/                   # Inventory alert engine & lifecycle
│   ├── app/config.go             # Environment-based configuration
│   ├── audit/service.go          # Immutable append-only audit log
│   ├── auth/service.go           # Session-based authentication
│   ├── cart/                     # Cart CRUD, validation, merge, checkout
│   ├── catalog/                  # Vehicle models, brands, series, versioning
│   ├── crypto/                   # Argon2id hashing + AES-GCM encryption
│   ├── db/                       # Connection pool, migrations, seeds
│   │   └── migrations/           # Sequential SQL migration files
│   ├── http/
│   │   ├── handlers/             # Gin handler functions (pages + API)
│   │   ├── middleware/           # Auth, CSRF, permission middleware
│   │   └── views/                # Templ components + HTML templates
│   ├── imports/                  # CSV import/validate/commit pipeline
│   ├── masking/                  # PII masking utilities
│   ├── metrics/                  # Metric definitions, versioning, lineage
│   ├── notifications/            # Notifications, announcements, export queue
│   ├── orders/                   # Order lifecycle, state machine, split
│   ├── rbac/                     # Roles, permissions, permission matrix
│   ├── scheduler/                # Background job scheduler
│   └── testutil/                 # Test database helpers
├── testdata/                     # Sample CSV files for import testing
├── web/
│   ├── exports/                  # Notification export output files
│   ├── static/                   # CSS, JS (HTMX)
│   └── uploads/                  # User-uploaded media files
├── docker-compose.yml
├── Dockerfile
├── go.mod
└── go.sum
```

---

## 3. Technology Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Language | Go | 1.25 |
| HTTP Framework | Gin | 1.12 |
| Template Engine | Templ (a-h/templ) | 0.3.1001 |
| Fallback Templates | Go html/template | stdlib |
| Client Interactivity | HTMX | bundled |
| Database | PostgreSQL | 15+ |
| Database Driver | pgx/v5 | 5.9.1 |
| Migrations | golang-migrate/v4 | 4.19.1 |
| Password Hashing | Argon2id (x/crypto) | - |
| Field Encryption | AES-256-GCM (stdlib) | - |
| UUID Generation | google/uuid | 1.6.0 |
| Containerization | Docker + Docker Compose | - |

---

## 4. Data Model Design

### 4.1 Core Entity Relationships

```
roles ──< role_permissions >── permissions
  │
  └──< user_roles >── users ──< sessions
                         │
                         ├──< carts ──< cart_items ──> vehicle_models
                         │      └──< cart_events
                         │
                         ├──< orders ──< order_lines ──> vehicle_models
                         │      ├──< order_notes
                         │      ├──< order_state_history
                         │      ├──< order_events
                         │      └──< payment_records
                         │
                         └──< notification_recipients ──> notifications
                                                    
brands ──< series ──< vehicle_models ──< vehicle_model_versions
                              └──< vehicle_media

customer_accounts ──< carts
                 └──< orders

alert_rules ──< alerts ──< alert_events

metric_definitions ──< metric_definition_versions
       ├──< metric_dimensions
       ├──< metric_filters
       ├──< metric_dependencies
       ├──< metric_permissions
       ├──< metric_activation_reviews
       └──< chart_metric_dependencies ──> charts

metric_lineage_edges (source_type/id -> target_type/id)

notification_templates
announcements ──< announcement_reads
notification_preferences (user × channel × event_type)
export_queue_items ──< export_attempt_logs

audit_log (append-only, spans all entities)
csv_import_jobs ──< csv_import_rows
```

### 4.2 Key Schema Design Decisions

- **Append-only audit**: `audit_log` has no UPDATE/DELETE operations exposed. Captures `before_json` and `after_json` for full change tracking.
- **Encrypted PII**: Customer phone numbers use separate columns for encrypted data (`BYTEA`), nonce (`BYTEA`), and masked display (`VARCHAR`).
- **Versioned entities**: Both `vehicle_models` and `metric_definitions` track version history with `is_current_draft` / `is_current_pub` flags.
- **State constraints**: Order status and alert status use CHECK constraints with explicit enum values.
- **Deduplication**: Alert unique constraint on `(alert_rule_id, entity_type, entity_id, status)` prevents duplicate open alerts.

---

## 5. Authentication & Authorization

### 5.1 Authentication Flow

1. User submits username/password to `POST /login`
2. Server looks up user, verifies password with Argon2id constant-time comparison
3. Checks `is_active` flag (disabled accounts are rejected)
4. Generates a cryptographically random 64-character hex session ID
5. Stores session in PostgreSQL `sessions` table with 24-hour expiry
6. Sets session ID as an HTTP cookie
7. Redirects to dashboard

### 5.2 Session Validation (Every Request)

1. `AuthMiddleware` extracts session ID from cookie
2. Queries `sessions` table, checks expiry
3. Loads user record, verifies `is_active`
4. Loads user's permissions and roles via RBAC service
5. Injects user, permissions, and roles into request context

### 5.3 Authorization Model

- **25 permission codes** across 10 domains (catalog, cart, order, notification, alert, audit, metric, dashboard, media, system)
- **Per-route enforcement**: Every route handler is wrapped with `middleware.RequirePermission(permCode)`
- **Object-level scoping**: Cart and order handlers implement additional scope checks (Sales Associates see only their own carts/orders; Admins and Auditors see all)
- **Role-permission matrix**: Defined both in code (`rbac.RolePermissions`) and seeded into the database for runtime resolution

---

## 6. Order State Machine

### 6.1 State Diagram

```
                    ┌─────────────┐
                    │   created   │
                    └──────┬──────┘
                  ┌────────┼────────────┐
                  ▼        ▼            ▼
         ┌──────────┐ ┌────────┐ ┌───────────┐
         │ payment  │ │ cutoff │ │ cancelled │
         │ recorded │ │(auto)  │ └───────────┘
         └────┬─────┘ └───┬────┘
              │            │
              └──────┬─────┘
                     ▼
              ┌──────────┐
              │ picking  │
              └────┬─────┘
           ┌───────┼──────────────┐
           ▼       ▼              ▼
    ┌─────────┐ ┌──────────────┐ ┌───────────┐
    │ arrival │ │ partially    │ │ cancelled │
    │         │ │ backordered  │ └───────────┘
    └────┬────┘ └──────┬───────┘
    ┌────┴────┐        │
    ▼         ▼        ▼
┌────────┐┌────────┐┌───────┐
│ pickup ││delivery││ split │
└───┬────┘└───┬────┘└───────┘
    │         │       (terminal)
    └────┬────┘
         ▼
   ┌───────────┐
   │ completed │
   └───────────┘
     (terminal)
```

### 6.2 Automatic Cutoff

Orders in `created` status that have not received a `payment_recorded` transition within 30 minutes are automatically moved to `cutoff` by the scheduler. This is recorded with `actor_type = 'system'` in the state history.

### 6.3 Partial Fulfillment & Split

During checkout, stock is allocated line-by-line with row-level locking (`SELECT ... FOR UPDATE`). If insufficient stock exists:
- Available units are allocated; the remainder becomes a backorder quantity
- Line status is set to `partial` or `backordered`
- The order can transition to `partially_backordered`
- A split operation creates a child order for backordered lines, linked via `split_parent_order_id`

---

## 7. Inventory Alert Engine

### 7.1 Alert Rules

| Rule | Type | Condition | Severity |
|------|------|-----------|----------|
| Low Stock | `low_stock` | stock < 5 (published models) | Warning |
| Overstock | `overstock` | stock > 250 (published models) | Info |
| Near Expiry | `near_expiry` | expiry within 14 days (published models) | Critical |

### 7.2 Evaluation

The scheduler runs `EvaluateAlerts` every 15 minutes (configurable). For each active rule, it queries published vehicle models matching the condition and creates alerts with deduplication via the unique constraint.

### 7.3 Closed-Loop Lifecycle

```
Open → Claimed → Processing → Closed (with mandatory resolution notes)
```

Each transition requires the `alert.manage` permission and writes both an `alert_events` record and an `audit_log` entry.

---

## 8. Metric Framework

### 8.1 Metric Store

Metric definitions support:
- **Versioned definitions**: Each edit creates a new version with SQL expression, semantic formula, time grain, and window calculation
- **Dimensions**: Named dimensions associated with a metric (e.g., "region", "brand")
- **Filters**: Named filter expressions applied to metric calculations
- **Dependencies**: Directed graph of metric-to-metric dependencies (self-references prevented by CHECK constraint)
- **Derived metrics**: Flag indicating a metric is computed from other metrics

### 8.2 Activation Flow

1. Create/edit metric definition (status: `draft`)
2. Run impact analysis → counts dependent metrics, dependent charts, missing dependencies
3. Review stored in `metric_activation_reviews`
4. If approved and no missing dependencies → activate (status: `active`)
5. Lineage tracked via `metric_lineage_edges` for cross-entity dependency graphs

### 8.3 Metric Permissions

Per-metric ACLs via `metric_permissions` table with `can_view` and `can_activate` flags, assignable by user or role.

---

## 9. Notification System

### 9.1 In-App Notifications

- Targeted per-user notifications with read/unread tracking
- Bulk mark-as-read support
- Linked to entities (e.g., order, alert) for navigation

### 9.2 System Announcements

- Global broadcasts with priority levels (low, normal, high, critical)
- Active/inactive toggle with optional date range
- Per-user read tracking via `announcement_reads` table

### 9.3 Export Queues

- Channels: `email`, `sms`, `webhook` (local file output only)
- Retry logic: up to 3 attempts with logging
- Messages rendered from templates with variable substitution
- All export attempts logged locally in `export_attempt_logs`

### 9.4 User Preferences

Per-user, per-channel, per-event-type subscription preferences stored in `notification_preferences`.

---

## 10. Security Design

### 10.1 Authentication Security
- Argon2id password hashing (64 MB memory, 3 iterations, 2-lane parallelism)
- Constant-time password comparison to prevent timing attacks
- Cryptographically random 64-character hex session tokens
- 24-hour session expiry with server-side invalidation

### 10.2 Data Protection
- AES-256-GCM encryption for sensitive fields (customer phone numbers)
- Separate nonce storage per encrypted value
- Pre-computed masked values for UI display without decryption
- Phone masking format: `***-***-XXXX` (last 4 digits visible)

### 10.3 Request Security
- CSRF double-submit cookie protection on all state-changing requests
- Per-route permission enforcement via middleware
- Object-level authorization scoping in handlers
- File upload size limits (configurable, default 25 MB)
- SHA-256 file fingerprints for integrity verification

### 10.4 Local-Only Principle
- No external network calls at runtime
- Notification exports written to local filesystem
- All session, audit, and retry data stored in PostgreSQL
- File uploads stored on local disk

---

## 11. Frontend Design

### 11.1 Rendering Strategy

The UI uses a hybrid server-side rendering approach:
- **Templ components** (`.templ` files): Layout shell, dashboard, catalog list, cart, orders, notifications, login
- **Go HTML templates** (`.html` files): Alerts, audit log, metrics, catalog detail/edit, order detail, cart detail
- **HTMX**: Progressive enhancement for in-page interactions (modals, form submissions, live updates)

### 11.2 Page Structure

All pages share a common layout with:
- Navigation sidebar with role-based link visibility
- Unread notification count badge
- Flash message banners (success/error) via cookie-based flash messages
- CSRF token injection in all forms

### 11.3 Key Views

| View | Template Type | Description |
|------|--------------|-------------|
| Login | Templ | Username/password form |
| Dashboard | Templ + HTML | Open carts, active orders, unread count |
| Catalog List | Templ | Browse by brand/series, filter by status |
| Catalog Detail | HTML | Vehicle model details, media, versions |
| Catalog Edit | HTML | Draft editing form with publish controls |
| Cart List | Templ | Open carts with customer info |
| Cart Detail | HTML | Items, validation status, merge/checkout actions |
| Orders List | Templ | Orders with status filters |
| Order Detail | HTML | Timeline, lines, notes, transition controls |
| Notifications | Templ + HTML | Unified inbox with announcements |
| Alerts | HTML | Alert list with claim/process/close actions |
| Metrics | HTML | Metric definitions with impact analysis |
| Audit Log | HTML | Searchable audit trail |

---

## 12. Deployment

### 12.1 Docker Compose

The application ships with `docker-compose.yml` and `Dockerfile` for single-command deployment:
- Go application container
- PostgreSQL container
- Shared volume for uploads and exports

### 12.2 Configuration

All configuration via environment variables with sensible defaults:

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | `postgres://fleet:fleet@localhost:5432/fleetcommerce?sslmode=disable` | PostgreSQL connection |
| `SESSION_SECRET` | (development default) | Session signing key |
| `ENCRYPTION_KEY` | (development default) | AES-256 key (hex) |
| `UPLOADS_DIR` | `./web/uploads` | Media upload path |
| `EXPORTS_DIR` | `./web/exports` | Notification export path |
| `MAX_UPLOAD_BYTES` | `26214400` (25 MB) | Upload size limit |
| `SCHEDULER_ENABLED` | `true` | Enable background jobs |
| `CUTOFF_INTERVAL_SEC` | `60` | Order cutoff check interval |
| `ALERT_INTERVAL_SEC` | `900` | Alert evaluation interval |
| `EXPORT_RETRY_INTERVAL_SEC` | `300` | Export retry interval |

### 12.3 Startup Sequence

1. Connect to PostgreSQL (with retry loop, up to 30 attempts)
2. Run database migrations (embedded SQL files)
3. Run seed data (idempotent, safe on every boot)
4. Initialize all services with dependency injection
5. Configure Gin router with middleware and routes
6. Start scheduler (if enabled)
7. Start HTTP server
8. Wait for shutdown signal (SIGINT/SIGTERM)
9. Graceful shutdown with 10-second timeout
