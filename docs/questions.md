# Project Clarification Questions

## Business Logic Questions Log

### 1. Backend Framework and Language Choice

**Question:** What exact backend framework and language should be used for the REST-style API server?
**My Understanding:** The prompt specifies "Gin to expose REST-style endpoints" but does not specify the exact Go version or additional middleware libraries needed for a production-grade system with CSRF protection, session management, and static file serving.
**Solution:** Implement the backend as a Go 1.25 application using Gin v1.12 as the HTTP framework, with custom middleware for CSRF protection (double-submit cookie pattern), session-based authentication via PostgreSQL-backed sessions, and `gin.Static` for serving uploaded files and frontend assets.

---

### 2. Frontend Rendering Approach

**Question:** What rendering strategy should the Templ-rendered web UI use—server-side only, or a hybrid with client-side interactivity?
**My Understanding:** The prompt says "Templ-rendered web UI" and mentions "no page reload surprises," but does not specify whether to use a full SPA framework or server-rendered HTML with progressive enhancement.
**Solution:** Implement a dual rendering system: Templ (Go template engine v0.3) generates server-side HTML components (layout, dashboard, catalog, cart, orders, notifications), supplemented with Go `html/template` for additional views (alerts, audit, metrics, catalog detail/edit). HTMX is included for in-page interactions without full page reloads. Flash messages via cookies provide success/error banners.

---

### 3. Password Hashing Algorithm

**Question:** What specific password hashing algorithm should be used for "salted and hashed" passwords?
**My Understanding:** The prompt requires passwords to be "salted and hashed" but does not name a specific algorithm. The choice affects security strength and performance.
**Solution:** Implement Argon2id password hashing (via `golang.org/x/crypto/argon2`) with parameters: 64 MB memory, 3 iterations, 2 parallelism lanes, 16-byte salt, 32-byte key length. Passwords are stored in the format `$argon2id$v=19$m=65536,t=3,p=2$<salt_hex>$<hash_hex>`. Verification uses constant-time byte comparison to prevent timing attacks.

---

### 4. Encryption Algorithm for Sensitive Fields

**Question:** What encryption algorithm should be used for encrypting sensitive fields like customer phone numbers at rest?
**My Understanding:** The prompt requires "sensitive fields such as customer phone numbers are masked in the UI and encrypted at rest" but does not specify the cipher.
**Solution:** Implement AES-256-GCM encryption using the `crypto/aes` and `crypto/cipher` standard library packages. The encryption key is a 32-byte hex string provided via the `ENCRYPTION_KEY` environment variable. Each encrypted field stores both the ciphertext (`BYTEA`) and nonce (`BYTEA`) in separate database columns, alongside a pre-computed masked representation (e.g., `***-***-1234`) for UI display without decryption.

---

### 5. Session Management Strategy

**Question:** How should user sessions be managed—JWT tokens, server-side sessions, or cookie-based sessions?
**My Understanding:** The prompt requires "local username and password" sign-in but does not specify the session mechanism.
**Solution:** Implement server-side sessions stored in PostgreSQL. On login, a cryptographically random 64-character hex session ID is generated, stored in the `sessions` table with a 24-hour expiry, and sent to the client as a cookie. Session validation queries the database on each authenticated request. Logout destroys the session record. Expired sessions are cleaned up by the `CleanExpiredSessions` method.

---

### 6. Order State Machine Transitions and Terminal States

**Question:** What exact state transitions should the order state machine support, and which states are terminal?
**My Understanding:** The prompt lists states "creation to payment-recorded, cutoff, picking, arrival, pickup or delivery, and completion" and mentions cancellation and partial backorder handling, but does not define every valid transition path.
**Solution:** Implement a strict state machine with the following transitions:
- `created` -> `payment_recorded`, `cutoff`, `cancelled`
- `payment_recorded` -> `picking`, `cancelled`
- `cutoff` -> `picking`, `cancelled`
- `picking` -> `arrival`, `partially_backordered`, `cancelled`
- `arrival` -> `pickup`, `delivery`, `cancelled`
- `pickup` -> `completed`
- `delivery` -> `completed`
- `partially_backordered` -> `split`, `picking`, `cancelled`
- `split`, `completed`, `cancelled` are terminal states (no further transitions)

Invalid transitions return `ErrInvalidTransition` and are rejected server-side.

---

### 7. Cutoff Mechanism Details

**Question:** How exactly should the automatic cutoff work—what triggers it, and what happens to the order?
**My Understanding:** The prompt says "cutoff occurs automatically 30 minutes after creation unless payment-recorded is posted by an authorized role" but does not detail the implementation mechanism.
**Solution:** Implement a scheduled job (via the `Scheduler` component) that runs at a configurable interval (default 60 seconds). The `ProcessCutoffs` method queries for all orders in `created` status where `created_at + 30 minutes < NOW()` and no `payment_recorded_at` has been set. These orders are automatically transitioned to `cutoff` status with the actor recorded as `system`. Each transition writes to `order_state_history` with `actor_type = 'system'`.

---

### 8. Partial Out-of-Stock and Order Split Behavior

**Question:** How should partial stock allocation and order splitting work during checkout and fulfillment?
**My Understanding:** The prompt mentions "partial out-of-stock handling that allows backorder lines and optional order split for immediate fulfillment" but does not define the allocation algorithm or split mechanics.
**Solution:** During order creation from cart checkout, each line item is evaluated against current stock (with `FOR UPDATE` row locking). If stock is sufficient, the full quantity is allocated and stock is deducted. If stock is insufficient, available stock is allocated and the remainder is marked as backordered. Line statuses are set to `allocated`, `backordered`, or `partial`. If any line has a backorder, the order status can transition to `partially_backordered`. The split operation creates a new child order (linked via `split_parent_order_id`) containing only the backordered quantities, while the original order retains allocated quantities and transitions to `split` status. Order lines track `quantity_requested`, `quantity_allocated`, `quantity_backordered`, and snapshot the stock level and publication status at time of creation.

---

### 9. RBAC Permission Model and Role Definitions

**Question:** What specific permissions should each role have, and how granular should the permission system be?
**My Understanding:** The prompt names four roles (Administrator, Inventory Manager, Sales Associate, Auditor) but does not enumerate their exact permission sets.
**Solution:** Implement a database-backed RBAC system with 25 granular permissions across 10 domains. Role assignments:
- **Administrator**: All 25 permissions (full system access)
- **Inventory Manager**: `catalog.read`, `catalog.write`, `catalog.publish`, `catalog.import`, `media.upload`, `order.read`, `order.transition`, `notification.read`, `alert.read`, `alert.manage`, `dashboard.read`
- **Sales Associate**: `catalog.read`, `cart.read`, `cart.write`, `cart.merge`, `order.read`, `order.create`, `order.notes`, `notification.read`, `dashboard.read`
- **Auditor**: `catalog.read`, `cart.read`, `order.read`, `notification.read`, `alert.read`, `audit.read`, `metric.read`, `dashboard.read`

Every API and page route is guarded by `middleware.RequirePermission()` with the specific permission code.

---

### 10. Cart Merge Rules

**Question:** What are the exact rules for merging carts, and what validation should be applied?
**My Understanding:** The prompt says "merging carts by customer account" but does not specify what happens with duplicate items, whether carts must belong to the same customer, or what states are eligible for merging.
**Solution:** Cart merging enforces the following rules:
- Both carts must be in `open` status (`ErrCartNotOpen`)
- A cart cannot merge with itself (`ErrMergeSameCart`)
- Both carts must belong to the same customer account (`ErrMergeDiffCustomer`)
- Items from the source cart are transferred to the target cart
- The merge operation is recorded in the audit log with before/after state
- Cart events track the merge action with actor information

---

### 11. Cart Item Validation and Flagging

**Question:** What specific conditions should flag a cart item as invalid?
**My Understanding:** The prompt says "automatically flagging invalid items (discontinued, not published, or stock at zero)" but does not specify how flagging interacts with checkout or whether flagged items block the order.
**Solution:** Cart validation (`ValidateCart`) checks each item against current vehicle model state:
- `discontinued`: item's vehicle model has a non-null `discontinued_at` timestamp
- `unpublished`: item's vehicle model `publication_status` is not `published`
- `out_of_stock`: item's vehicle model `stock_quantity` is 0
- `valid`: none of the above conditions apply

Each cart item stores a `validity_status` and `validation_message`. On checkout, if all items are invalid, the checkout is rejected with `ErrAllItemsInvalid`. Additionally, a cart must have a customer account assigned (`ErrNoCustomer`) to proceed to checkout.

---

### 12. Inventory Alert Thresholds and Lifecycle

**Question:** How should inventory alert rules be configured, and what exactly is the closed-loop lifecycle?
**My Understanding:** The prompt specifies thresholds (low stock <5, overstock >250, near-expiry <14 days) and a lifecycle of "claim, process, and close with mandatory resolution notes" but does not detail the evaluation engine or deduplication.
**Solution:** Alert rules are stored in the `alert_rules` table with JSON conditions (e.g., `{"operator":"lt","threshold":5}`). The `EvaluateAlerts` method runs on a schedule (default every 15 minutes / 900 seconds) and queries published vehicle models against each active rule. Alerts are deduplicated by a unique constraint on `(alert_rule_id, entity_type, entity_id, status)` so the same condition doesn't create duplicate open alerts. The lifecycle is:
- **Open**: Alert created by the evaluation engine
- **Claimed**: A user with `alert.manage` permission claims the alert (records `claimed_by` and `claimed_at`)
- **Processing**: The claimer marks it as being worked on
- **Closed**: Requires mandatory `resolution_notes` (`ErrResolutionRequired` if empty); records `closed_at`

Each lifecycle transition writes an `alert_events` record and an audit log entry.

---

### 13. Notification Export Queue Channels

**Question:** How should the "email/SMS/webhook" export queues be implemented given the offline-first, local-only constraint?
**My Understanding:** The prompt says these channels are "represented as optional local export queues for later processing" but does not define the export format or retry strategy.
**Solution:** Export queue items are stored in the `export_queue_items` table with fields for channel (`email`, `sms`, `webhook`), recipient, subject, body, status (`pending`, `exported`, `retrying`, `failed`, `cancelled`), attempt count, and max attempts (default 3). Rather than sending externally, the `ProcessExportRetries` method writes message content to local files in the configured exports directory (`./web/exports`). The scheduler runs export retry processing at a configurable interval (default 300 seconds). Each attempt is logged in `export_attempt_logs` with the exported file path, status, and any error message.

---

### 14. Notification Template Variable Rendering

**Question:** How should templated notification messages render variables like order number, promised date, and location?
**My Understanding:** The prompt mentions "templated messages that render variables" but does not specify the template syntax or rendering engine.
**Solution:** Notification templates are stored in the `notification_templates` table with a `code` identifier, `subject` and `body` fields using `{{variable_name}}` syntax, and a JSON array of expected variable names. Template rendering uses Go's `strings.Replacer` to substitute `{{order_number}}`, `{{promised_date}}`, `{{customer_account_name}}`, `{{location}}`, `{{alert_type}}`, and other variables. Templates are seeded with predefined codes: `order_created`, `order_transition`, `alert_opened`, and `payment_reminder`.

---

### 15. File Upload Validation Details

**Question:** What specific file type validations and storage conventions should be applied beyond the 25 MB size limit?
**My Understanding:** The prompt requires "file uploads validated by type and size (for example max 25 MB) and stored on local disk with SHA-256 fingerprints" but does not enumerate allowed types or the storage path structure.
**Solution:** File uploads for vehicle media are validated as follows:
- **Size**: Configurable maximum via `MAX_UPLOAD_BYTES` environment variable (default 25 MB / 26,214,400 bytes)
- **Type**: The uploaded file's MIME type is checked; the `kind` field accepts `image` or `video`
- **Storage**: Files are saved to the configured uploads directory (`./web/uploads`) with a UUID-based filename to prevent collisions, preserving the original file extension
- **Fingerprint**: A SHA-256 hash is computed over the file contents and stored as a 64-character hex string in the `sha256_fingerprint` column of `vehicle_media`
- **Metadata**: Original filename, stored path, MIME type, size in bytes, and uploader user ID are all recorded

---

### 16. Metric Framework Impact Analysis and Activation Flow

**Question:** What does "impact analysis before activation" mean concretely for the metric framework?
**My Understanding:** The prompt requires "definition changes require impact analysis before activation, track lineage to dependent charts, and enforce metric-level permissions" but does not define what the analysis checks or how activation is gated.
**Solution:** The metric activation flow works as follows:
1. **Impact Analysis** (`ImpactAnalysisAPI`): Before activating a metric version, the system computes: count of dependent metrics (via `metric_dependencies`), count of dependent charts (via `chart_metric_dependencies`), and count of unresolved dependencies. The result is stored in `metric_activation_reviews` with status `pending`.
2. **Activation Gate**: Activation is blocked with `ErrImpactAnalysisRequired` unless a completed impact analysis review exists. `ErrMissingDependencies` blocks activation if dependencies are unresolved.
3. **Lineage**: The `metric_lineage_edges` table tracks directed relationships between metrics and charts (`source_type`/`target_type` of `metric` or `chart`), enabling lineage traversal via the `MetricLineageAPI` endpoint.
4. **Metric Permissions**: The `metric_permissions` table supports per-metric ACLs by user or role, with `can_view` and `can_activate` flags.

---

### 17. Audit Trail Scope and Immutability

**Question:** What entity types should be audited, and how is immutability enforced?
**My Understanding:** The prompt requires "every state change writes an immutable operation audit trail including actor, timestamp, and before/after values" but does not specify which entities beyond orders are audited or the immutability mechanism.
**Solution:** The audit log is a centralized, append-only table (`audit_log`) that records events across all major entities: orders, carts, catalog models, alerts, metrics, imports, and notifications. Each entry captures: `entity_type`, `entity_id`, `action`, `actor_user_id`, `actor_role`, `occurred_at`, `before_json`, `after_json`, `metadata_json`, `request_id`, and `ip_address`. Immutability is enforced by design: the service only exposes `Log` (insert) and `List` (read) methods—there is no update or delete functionality. The `LogTx` method supports writing audit entries within database transactions to ensure atomic consistency between state changes and their audit records.

---

### 18. Dashboard Summary Content

**Question:** What specific data should the dashboard show beyond "open carts, active orders, and unread notifications"?
**My Understanding:** The prompt says users "land on a dashboard showing open carts, active orders, and unread notifications" but does not define the full summary data shape.
**Solution:** The dashboard provides a `DashboardSummaryAPI` endpoint returning aggregated counts of: open carts (scoped by user role—Sales Associates see only their own, Administrators and Auditors see all), active orders (non-terminal status, with similar scoping), and unread notification count. The dashboard page renders via both Templ components and HTML templates, with the active navigation state set to highlight the dashboard link.

---

### 19. CSV Import Preview and Commit Workflow

**Question:** How should the two-phase CSV import (preview with row-level errors, then commit) be implemented?
**My Understanding:** The prompt says "bulk import/export is offered as local CSV files with preview and row-level error marking before commit" but does not define required columns, validation rules, or the commit process.
**Solution:** CSV import follows a two-phase workflow:
1. **Upload and Validate** (`ImportCatalogCSV`): The CSV must contain headers: `model_code`, `model_name`, `brand`, `series`, `year`. Each row is parsed and validated (e.g., year must be numeric). Valid rows are stored in `csv_import_rows` with status `valid`; invalid rows get status `invalid` with a JSON errors array describing each issue. The parent `csv_import_jobs` record tracks `total_rows`, `valid_rows`, and `invalid_rows`. Job status is set to `validated`.
2. **Preview** (`GetImportJob`): Returns the job summary and all rows with their validation status, allowing the user to review errors before committing.
3. **Commit** (`CommitImportJob`): Processes only `valid` rows, creating or updating vehicle models in the catalog. Committed rows are marked `committed` and the job status becomes `committed`. The entire operation is audit-logged.

---

### 20. Announcement System vs. Per-User Notifications

**Question:** How should system-wide announcements differ from targeted per-user notifications?
**My Understanding:** The prompt mentions "in-app messages and system announcements" but does not distinguish how they are stored, displayed, or managed differently.
**Solution:** The system implements two distinct notification types:
- **Notifications**: Targeted per-user messages stored in `notifications` with recipient records in `notification_recipients` (per-user `is_read` tracking). Created programmatically when events occur (e.g., order state changes).
- **Announcements**: System-wide broadcasts stored in `announcements` with priority levels (`low`, `normal`, `high`, `critical`), active/inactive toggle, and optional date range (`starts_at`, `expires_at`). Read state is tracked per user via the `announcement_reads` table (added in migration 004). Announcements are listed separately via `ListAnnouncementsAPI` and marked read individually via `MarkAnnouncementReadAPI`.

Both types appear in the unified Notification Center view with bulk mark-as-read support for notifications.

---

### 21. User Notification Subscription Preferences

**Question:** How should users configure which notifications they receive and through which channels?
**My Understanding:** The prompt mentions "user subscription preferences" but does not detail the preference model or available event types.
**Solution:** Notification preferences are stored per user in the `notification_preferences` table with a unique constraint on `(user_id, channel, event_type)`. Available channels are `in_app`, `email`, `sms`, and `webhook`. Users can enable or disable specific event types per channel via the `GetPreferencesAPI` and `UpdatePreferencesAPI` endpoints. This allows granular control—for example, a user can receive order alerts in-app but disable SMS for the same event type.

---

### 22. CSRF Protection Mechanism

**Question:** How should CSRF protection be implemented for the form-based Templ UI?
**My Understanding:** The prompt does not explicitly mention CSRF protection, but it is a critical security requirement for any session-based web application with form submissions.
**Solution:** Implement a double-submit cookie CSRF middleware (`middleware.CSRFMiddleware`). A CSRF token is generated and set as a cookie; every state-changing request (POST, PUT, DELETE) must include the token in either a form field or request header. The middleware validates the token matches the cookie value. The token is injected into all Templ templates and HTML forms via the `CSRFToken` field in page data, and included in HTMX requests via headers.

---

### 23. Catalog Versioning and Draft/Publish Workflow

**Question:** How should the draft editing and publish/unpublish workflow handle version history for vehicle models?
**My Understanding:** The prompt mentions "draft edits, and publish/unpublish controls" but does not define whether versions are tracked or how drafts relate to published state.
**Solution:** Vehicle models use a versioning system via the `vehicle_model_versions` table. Each version records a snapshot of `model_name`, `year`, `description`, `stock_quantity`, `expiry_date`, with flags `is_current_draft` and `is_current_pub`. The workflow:
- **Create**: New models start in `draft` status with version 1
- **Draft Edit** (`UpdateDraftAPI`): Creates a new version with `is_current_draft = true`, incrementing the version number
- **Publish** (`PublishModelAPI`): Sets `publication_status = 'published'` on the model and marks the current draft version as `is_current_pub = true`
- **Unpublish** (`UnpublishModelAPI`): Reverts `publication_status` to `unpublished`

All version changes are audit-logged with before/after state.

---

### 24. Order Notes Categories

**Question:** What types of order notes should be supported for picking and delivery workflows?
**My Understanding:** The prompt mentions "capturing order notes for picking and delivery" but does not define specific note categories or constraints.
**Solution:** Order notes are stored in `order_notes` with three types enforced by a CHECK constraint:
- `internal`: Internal notes visible only to staff
- `picking`: Notes specific to the warehouse picking process
- `delivery`: Notes for the delivery or pickup process

Notes have a maximum length of 2,000 characters (enforced at the database level), track the author via `author_id`, and are immutable once created (no update/delete operations exposed).

---

### 25. Scheduled Job Configuration

**Question:** How should the scheduled background jobs (cutoff, alert evaluation, export retry) be configured and managed?
**My Understanding:** The prompt mentions alerts "evaluated on a schedule (for example every 15 minutes)" and automatic cutoff, but does not detail the scheduling infrastructure.
**Solution:** Implement a lightweight in-process scheduler (`internal/scheduler`) with three independent goroutine-based ticker jobs:
- **Order Cutoff**: Runs every `CUTOFF_INTERVAL_SEC` seconds (default 60), processes 30-minute cutoff window
- **Alert Evaluation**: Runs every `ALERT_INTERVAL_SEC` seconds (default 900 / 15 minutes), evaluates all active alert rules
- **Export Retry**: Runs every `EXPORT_RETRY_INTERVAL_SEC` seconds (default 300 / 5 minutes), retries pending export queue items

The scheduler is enabled/disabled via the `SCHEDULER_ENABLED` environment variable (default true). Each job runs with a 30-second context timeout. The scheduler supports graceful shutdown via `Stop()` which closes a channel and waits for all goroutines via `sync.WaitGroup`.
