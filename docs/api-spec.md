# FleetCommerce Operations Hub - API Specification

## Base URL

```
http://localhost:8080
```

## Authentication

All API endpoints (under `/api/*`) and authenticated page routes require a valid session cookie. The session cookie is set upon successful login via `POST /login`.

## Common Response Format

### Success
```json
{
  "ok": true,
  "message": "Operation successful",
  "data": { ... }
}
```

### Error
```json
{
  "ok": false,
  "message": "Error description",
  "errors": [ ... ]
}
```

## CSRF Protection

All state-changing requests (POST, PUT, DELETE) require a valid CSRF token. The token must be included as:
- Form field: `csrf_token`
- Or HTTP header: `X-CSRF-Token`

The CSRF token is provided via cookie and embedded in all server-rendered pages.

---

## 1. Authentication Endpoints

### POST /login

Authenticate with username and password.

**Request Body** (form-encoded):
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `username` | string | Yes | User's login name |
| `password` | string | Yes | User's password |

**Success**: Redirect to `/` with session cookie set.
**Error**: Re-render login page with error message.

**Permission**: Public

---

### POST /logout

Destroy the current session.

**Success**: Redirect to `/login`.

**Permission**: Authenticated

---

## 2. Dashboard

### GET /api/dashboard/summary

Returns aggregated dashboard statistics.

**Response**:
```json
{
  "ok": true,
  "data": {
    "open_carts": 5,
    "active_orders": 12,
    "unread_notifications": 3
  }
}
```

**Permission**: `dashboard.read`

---

### GET /api/me

Returns the authenticated user's profile.

**Response**:
```json
{
  "ok": true,
  "data": {
    "id": 1,
    "username": "admin",
    "full_name": "System Administrator",
    "roles": ["administrator"],
    "permissions": ["catalog.read", "catalog.write", ...]
  }
}
```

**Permission**: Authenticated (any role)

---

## 3. Catalog Endpoints

### GET /api/catalog/brands

List all vehicle brands.

**Response**:
```json
{
  "ok": true,
  "data": [
    { "id": 1, "name": "Toyota" },
    { "id": 2, "name": "Honda" }
  ]
}
```

**Permission**: `catalog.read`

---

### GET /api/catalog/series

List series, optionally filtered by brand.

**Query Parameters**:
| Param | Type | Description |
|-------|------|-------------|
| `brand_id` | int | Filter by brand ID |

**Response**:
```json
{
  "ok": true,
  "data": [
    { "id": 1, "brand_id": 1, "name": "Sedan" },
    { "id": 2, "brand_id": 1, "name": "SUV" }
  ]
}
```

**Permission**: `catalog.read`

---

### GET /api/catalog/models

List vehicle models with pagination and filtering.

**Query Parameters**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | int | 1 | Page number |
| `per_page` | int | 20 | Items per page |
| `brand_id` | int | - | Filter by brand |
| `series_id` | int | - | Filter by series |
| `status` | string | - | Filter by publication status (`draft`, `published`, `unpublished`) |
| `q` | string | - | Search by model name or code |

**Response**:
```json
{
  "ok": true,
  "data": {
    "models": [
      {
        "id": 1,
        "brand_id": 1,
        "series_id": 1,
        "brand_name": "Toyota",
        "series_name": "Sedan",
        "model_code": "TOY-CAM-2025",
        "model_name": "Camry",
        "year": 2025,
        "description": "Mid-size sedan",
        "publication_status": "published",
        "stock_quantity": 45,
        "expiry_date": null,
        "discontinued_at": null,
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z"
      }
    ],
    "total": 10,
    "page": 1,
    "total_pages": 1
  }
}
```

**Permission**: `catalog.read`

---

### GET /api/catalog/models/:id

Get a single vehicle model with media and versions.

**Path Parameters**:
| Param | Type | Description |
|-------|------|-------------|
| `id` | int | Vehicle model ID |

**Response**:
```json
{
  "ok": true,
  "data": {
    "id": 1,
    "model_code": "TOY-CAM-2025",
    "model_name": "Camry",
    "year": 2025,
    "description": "Mid-size sedan",
    "publication_status": "published",
    "stock_quantity": 45,
    "brand_name": "Toyota",
    "series_name": "Sedan",
    "media": [
      {
        "id": 1,
        "kind": "image",
        "original_filename": "camry-front.jpg",
        "stored_path": "/uploads/abc123.jpg",
        "mime_type": "image/jpeg",
        "size_bytes": 2048576,
        "sha256_fingerprint": "a1b2c3..."
      }
    ],
    "versions": [
      {
        "version_number": 1,
        "model_name": "Camry",
        "year": 2025,
        "status": "published",
        "is_current_draft": false,
        "is_current_pub": true,
        "created_at": "2025-01-01T00:00:00Z"
      }
    ]
  }
}
```

**Permission**: `catalog.read`

---

### POST /api/catalog/models

Create a new vehicle model (starts in draft status).

**Request Body** (JSON):
```json
{
  "brand_id": 1,
  "series_id": 1,
  "model_code": "TOY-SUP-2025",
  "model_name": "Supra",
  "year": 2025,
  "description": "Sports car",
  "stock_quantity": 10,
  "expiry_date": "2025-12-31"
}
```

**Response**: `201 Created`
```json
{
  "ok": true,
  "message": "Model created",
  "data": { "id": 11 }
}
```

**Permission**: `catalog.write`

---

### PUT /api/catalog/models/:id/draft
### POST /api/catalog/models/:id/draft

Update the draft version of a vehicle model. Creates a new version record.

**Request Body** (JSON):
```json
{
  "model_name": "Supra Updated",
  "year": 2025,
  "description": "Updated sports car description",
  "stock_quantity": 15,
  "expiry_date": null
}
```

**Response**:
```json
{
  "ok": true,
  "message": "Draft updated"
}
```

**Permission**: `catalog.write`

---

### POST /api/catalog/models/:id/publish

Publish a vehicle model (sets `publication_status` to `published`).

**Response**:
```json
{
  "ok": true,
  "message": "Model published"
}
```

**Permission**: `catalog.publish`

---

### POST /api/catalog/models/:id/unpublish

Unpublish a vehicle model (sets `publication_status` to `unpublished`).

**Response**:
```json
{
  "ok": true,
  "message": "Model unpublished"
}
```

**Permission**: `catalog.publish`

---

### POST /api/catalog/models/:id/media

Upload an image or video attachment for a vehicle model.

**Request Body** (multipart/form-data):
| Field | Type | Description |
|-------|------|-------------|
| `file` | file | The media file (max 25 MB) |
| `kind` | string | `image` or `video` |

**Response**: `201 Created`
```json
{
  "ok": true,
  "message": "Media uploaded",
  "data": {
    "id": 1,
    "stored_path": "/uploads/abc123.jpg",
    "sha256_fingerprint": "a1b2c3d4..."
  }
}
```

**Permission**: `media.upload`

---

### GET /api/catalog/export.csv

Export the full vehicle catalog as a CSV file.

**Response**: `200 OK` with `Content-Type: text/csv`

CSV columns: `model_code`, `model_name`, `brand`, `series`, `year`, `description`, `status`, `stock_quantity`, `expiry_date`

**Permission**: `catalog.read`

---

### POST /api/catalog/imports

Upload a CSV file for bulk import with validation preview.

**Request Body** (multipart/form-data):
| Field | Type | Description |
|-------|------|-------------|
| `file` | file | CSV file with required headers |

Required CSV headers: `model_code`, `model_name`, `brand`, `series`, `year`

**Response**:
```json
{
  "ok": true,
  "message": "Import validated",
  "data": {
    "job_id": 1,
    "total_rows": 10,
    "valid_rows": 8,
    "invalid_rows": 2,
    "rows": [
      { "row_number": 1, "status": "valid", "raw_data": {...}, "errors": null },
      { "row_number": 2, "status": "invalid", "raw_data": {...}, "errors": ["year must be numeric"] }
    ]
  }
}
```

**Permission**: `catalog.import`

---

### GET /api/catalog/imports/:job_id

Get the status and rows of an import job (for preview).

**Response**: Same structure as POST response `data` field.

**Permission**: `catalog.import`

---

### POST /api/catalog/imports/:job_id/commit

Commit valid rows from an import job to the catalog.

**Response**:
```json
{
  "ok": true,
  "message": "Import committed",
  "data": { "committed_rows": 8 }
}
```

**Permission**: `catalog.import`

---

## 4. Cart Endpoints

### GET /api/carts

List carts (scoped by role: Sales Associates see own carts only).

**Response**:
```json
{
  "ok": true,
  "data": [
    {
      "id": 1,
      "customer_account_id": 1,
      "customer_account_name": "Acme Auto Dealers",
      "status": "open",
      "item_count": 3,
      "created_by": 3,
      "created_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

**Permission**: `cart.read`

---

### POST /api/carts

Create a new cart.

**Request Body** (JSON):
```json
{
  "customer_account_id": 1
}
```

**Response**: `201 Created`
```json
{
  "ok": true,
  "message": "Cart created",
  "data": { "id": 1 }
}
```

**Permission**: `cart.write`

---

### GET /api/carts/:id

Get cart details with items and validation status.

**Response**:
```json
{
  "ok": true,
  "data": {
    "id": 1,
    "status": "open",
    "customer_account_id": 1,
    "items": [
      {
        "id": 1,
        "vehicle_model_id": 1,
        "model_name": "Camry",
        "quantity": 2,
        "validity_status": "valid",
        "validation_message": null
      }
    ]
  }
}
```

**Permission**: `cart.read`

---

### POST /api/carts/:id/items

Add an item to a cart.

**Request Body** (JSON):
```json
{
  "vehicle_model_id": 1,
  "quantity": 2
}
```

**Response**: `201 Created`
```json
{
  "ok": true,
  "message": "Item added",
  "data": { "id": 1 }
}
```

**Permission**: `cart.write`

---

### PUT /api/carts/:id/items/:item_id

Update the quantity of a cart item.

**Request Body** (JSON):
```json
{
  "quantity": 5
}
```

**Response**:
```json
{
  "ok": true,
  "message": "Item updated"
}
```

**Permission**: `cart.write`

---

### DELETE /api/carts/:id/items/:item_id

Remove an item from a cart.

**Response**:
```json
{
  "ok": true,
  "message": "Item removed"
}
```

**Permission**: `cart.write`

---

### POST /api/carts/:id/merge

Merge another cart into this cart (both must belong to the same customer and be open).

**Request Body** (JSON):
```json
{
  "source_cart_id": 2
}
```

**Response**:
```json
{
  "ok": true,
  "message": "Carts merged"
}
```

**Validation Rules**:
- Both carts must be `open` status
- Cannot merge a cart with itself
- Both carts must belong to the same customer account

**Permission**: `cart.merge`

---

### POST /api/carts/:id/revalidate

Re-validate all cart items against current catalog and inventory state.

**Response**:
```json
{
  "ok": true,
  "message": "Cart revalidated",
  "data": {
    "items": [
      { "id": 1, "validity_status": "valid" },
      { "id": 2, "validity_status": "out_of_stock", "validation_message": "Stock is 0" }
    ]
  }
}
```

**Permission**: `cart.write`

---

### POST /api/carts/:id/checkout

Convert a cart to an order. Validates items and allocates stock.

**Request Body** (JSON):
```json
{
  "promised_date": "2025-02-15",
  "location": "New York, NY"
}
```

**Response**: `201 Created`
```json
{
  "ok": true,
  "message": "Order created",
  "data": {
    "order_id": 1,
    "order_number": "ORD-20250101-ABC123"
  }
}
```

**Validation Rules**:
- Cart must be `open` status
- Cart must have a customer account assigned
- At least one valid item must exist (not all items can be invalid)

**Permission**: `order.create`

---

## 5. Order Endpoints

### GET /api/orders

List orders with filtering (scoped by role).

**Query Parameters**:
| Param | Type | Description |
|-------|------|-------------|
| `page` | int | Page number |
| `per_page` | int | Items per page |
| `status` | string | Filter by order status |
| `customer_account_id` | int | Filter by customer |

**Response**:
```json
{
  "ok": true,
  "data": {
    "orders": [
      {
        "id": 1,
        "order_number": "ORD-20250101-ABC123",
        "status": "created",
        "customer_account_name": "Acme Auto Dealers",
        "location": "New York, NY",
        "promised_date": "2025-02-15",
        "created_at": "2025-01-01T00:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "total_pages": 1
  }
}
```

**Permission**: `order.read`

---

### GET /api/orders/:id

Get full order details including lines, notes, and state history.

**Response**:
```json
{
  "ok": true,
  "data": {
    "id": 1,
    "order_number": "ORD-20250101-ABC123",
    "status": "created",
    "customer_account_id": 1,
    "source_cart_id": 1,
    "promised_date": "2025-02-15",
    "location": "New York, NY",
    "lines": [
      {
        "id": 1,
        "vehicle_model_id": 1,
        "model_name": "Camry",
        "quantity_requested": 5,
        "quantity_allocated": 5,
        "quantity_backordered": 0,
        "line_status": "allocated",
        "stock_snapshot": 45
      }
    ],
    "notes": [
      {
        "id": 1,
        "note_type": "picking",
        "content": "Priority shipment",
        "author_name": "Sales Associate",
        "created_at": "2025-01-01T00:00:00Z"
      }
    ],
    "allowed_transitions": ["payment_recorded", "cutoff", "cancelled"]
  }
}
```

**Permission**: `order.read`

---

### POST /api/orders/:id/notes

Add a note to an order.

**Request Body** (JSON):
```json
{
  "note_type": "picking",
  "content": "Handle with care - fragile components"
}
```

**Validation**:
- `note_type` must be one of: `internal`, `picking`, `delivery`
- `content` max length: 2000 characters

**Response**:
```json
{
  "ok": true,
  "message": "Note added"
}
```

**Permission**: `order.notes`

---

### POST /api/orders/:id/payment-recorded

Record payment for an order (transitions from `created` to `payment_recorded`).

**Request Body** (JSON):
```json
{
  "amount": 50000.00,
  "method": "bank_transfer"
}
```

**Response**:
```json
{
  "ok": true,
  "message": "Payment recorded"
}
```

**Permission**: `order.payment`

---

### POST /api/orders/:id/transition

Transition an order to a new state.

**Request Body** (JSON):
```json
{
  "to_status": "picking",
  "reason": "Ready for warehouse processing"
}
```

**Validation**: The transition must be valid according to the state machine. Invalid transitions return `400 Bad Request`.

**Response**:
```json
{
  "ok": true,
  "message": "Order transitioned to picking"
}
```

**Permission**: `order.transition`

---

### POST /api/orders/:id/split

Split an order with backordered lines into two orders.

**Request Body** (JSON):
```json
{
  "backorder_line_ids": [2, 3]
}
```

**Response**:
```json
{
  "ok": true,
  "message": "Order split",
  "data": {
    "new_order_id": 2,
    "new_order_number": "ORD-20250101-DEF456"
  }
}
```

**Permission**: `order.split`

---

### GET /api/orders/:id/timeline

Get the full state transition history for an order.

**Response**:
```json
{
  "ok": true,
  "data": [
    {
      "from_status": null,
      "to_status": "created",
      "actor_type": "user",
      "actor_name": "Sales Associate",
      "reason": null,
      "transitioned_at": "2025-01-01T00:00:00Z"
    },
    {
      "from_status": "created",
      "to_status": "cutoff",
      "actor_type": "system",
      "reason": "Automatic cutoff after 30 minutes",
      "transitioned_at": "2025-01-01T00:30:00Z"
    }
  ]
}
```

**Permission**: `order.read`

---

## 6. Notification Endpoints

### GET /api/notifications

List notifications for the authenticated user.

**Query Parameters**:
| Param | Type | Description |
|-------|------|-------------|
| `unread_only` | bool | Show only unread notifications |

**Response**:
```json
{
  "ok": true,
  "data": [
    {
      "id": 1,
      "type": "order_created",
      "title": "Order ORD-20250101-ABC123 Created",
      "body": "A new order has been created...",
      "entity_type": "order",
      "entity_id": 1,
      "is_read": false,
      "created_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

**Permission**: `notification.read`

---

### POST /api/notifications/:id/read

Mark a single notification as read.

**Response**:
```json
{
  "ok": true,
  "message": "Notification marked as read"
}
```

**Permission**: `notification.read`

---

### POST /api/notifications/bulk-read

Mark multiple notifications as read.

**Request Body** (JSON):
```json
{
  "notification_ids": [1, 2, 3]
}
```

**Response**:
```json
{
  "ok": true,
  "message": "Notifications marked as read",
  "data": { "count": 3 }
}
```

**Permission**: `notification.read`

---

### GET /api/announcements

List active system announcements.

**Response**:
```json
{
  "ok": true,
  "data": [
    {
      "id": 1,
      "title": "System Maintenance",
      "body": "Scheduled maintenance window...",
      "priority": "high",
      "is_active": true,
      "is_read": false,
      "created_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

**Permission**: `notification.read`

---

### POST /api/announcements/:id/read

Mark an announcement as read for the authenticated user.

**Response**:
```json
{
  "ok": true,
  "message": "Announcement marked as read"
}
```

**Permission**: `notification.read`

---

### GET /api/notification-preferences

Get the authenticated user's notification subscription preferences.

**Response**:
```json
{
  "ok": true,
  "data": [
    { "channel": "in_app", "event_type": "order_created", "enabled": true },
    { "channel": "email", "event_type": "order_created", "enabled": false },
    { "channel": "sms", "event_type": "alert_opened", "enabled": true }
  ]
}
```

**Permission**: `notification.read`

---

### PUT /api/notification-preferences
### POST /api/notification-preferences

Update notification preferences.

**Request Body** (JSON):
```json
{
  "preferences": [
    { "channel": "email", "event_type": "order_created", "enabled": true },
    { "channel": "sms", "event_type": "order_created", "enabled": false }
  ]
}
```

**Response**:
```json
{
  "ok": true,
  "message": "Preferences updated"
}
```

**Permission**: `notification.read`

---

### GET /api/export-queue

List the notification export queue (local file-based).

**Response**:
```json
{
  "ok": true,
  "data": [
    {
      "id": 1,
      "channel": "email",
      "recipient": "dealer@example.com",
      "subject": "Order Confirmation",
      "status": "pending",
      "attempts": 0,
      "max_attempts": 3,
      "created_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

**Permission**: `notification.manage`

---

## 7. Alert Endpoints

### GET /api/alerts

List inventory alerts with optional status filter.

**Query Parameters**:
| Param | Type | Description |
|-------|------|-------------|
| `status` | string | Filter: `open`, `claimed`, `processing`, `closed` |

**Response**:
```json
{
  "ok": true,
  "data": [
    {
      "id": 1,
      "alert_rule_id": 1,
      "entity_type": "vehicle_model",
      "entity_id": 3,
      "status": "open",
      "severity": "warning",
      "title": "Low Stock: Tacoma (3 units)",
      "details": { "current_stock": 3, "threshold": 5 },
      "claimed_by": null,
      "created_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

**Permission**: `alert.read`

---

### POST /api/alerts/:id/claim

Claim an open alert for processing.

**Response**:
```json
{
  "ok": true,
  "message": "Alert claimed"
}
```

**Validation**: Alert must be in `open` status.

**Permission**: `alert.manage`

---

### POST /api/alerts/:id/process

Mark a claimed alert as being processed.

**Response**:
```json
{
  "ok": true,
  "message": "Alert marked as processing"
}
```

**Validation**: Alert must be in `claimed` status.

**Permission**: `alert.manage`

---

### POST /api/alerts/:id/close

Close an alert with mandatory resolution notes.

**Request Body** (JSON):
```json
{
  "resolution_notes": "Restock order placed with supplier. Expected delivery in 3 days."
}
```

**Validation**: 
- Alert must be in `processing` status
- `resolution_notes` is required (non-empty)

**Response**:
```json
{
  "ok": true,
  "message": "Alert closed"
}
```

**Permission**: `alert.manage`

---

### POST /api/alerts/evaluate

Manually trigger alert rule evaluation (normally runs on schedule).

**Response**:
```json
{
  "ok": true,
  "message": "Evaluation complete",
  "data": { "alerts_created": 2 }
}
```

**Permission**: `alert.manage`

---

## 8. Metric Endpoints

### GET /api/metrics

List all metric definitions.

**Response**:
```json
{
  "ok": true,
  "data": [
    {
      "id": 1,
      "name": "Total Orders",
      "description": "Count of all orders",
      "status": "active",
      "owner_name": "System Administrator",
      "latest_version": 1,
      "dependency_count": 0,
      "created_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

**Permission**: `metric.read`

---

### POST /api/metrics

Create a new metric definition with an initial version.

**Request Body** (JSON):
```json
{
  "name": "Revenue per Order",
  "description": "Average revenue per completed order",
  "sql_expression": "SELECT AVG(total) FROM order_totals WHERE status='completed'",
  "semantic_formula": "SUM(revenue) / COUNT(orders)",
  "time_grain": "daily",
  "is_derived": true,
  "window_calculation": "OVER (PARTITION BY date)",
  "depends_on_metrics": [1],
  "filters": [
    { "name": "completed_only", "expression": "status = 'completed'" }
  ]
}
```

**Response**: `201 Created`
```json
{
  "ok": true,
  "message": "Metric created",
  "data": { "id": 4 }
}
```

**Permission**: `metric.write`

---

### GET /api/metrics/:id

Get a metric definition with its latest version details.

**Permission**: `metric.read`

---

### PUT /api/metrics/:id

Update a metric definition (creates a new version).

**Request Body** (JSON): Same fields as create.

**Permission**: `metric.write`

---

### GET /api/metrics/:id/versions

List all versions of a metric definition.

**Response**:
```json
{
  "ok": true,
  "data": [
    {
      "id": 1,
      "version_number": 1,
      "sql_expression": "SELECT COUNT(*) FROM orders",
      "time_grain": "daily",
      "status": "active",
      "created_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

**Permission**: `metric.read`

---

### GET /api/metrics/:id/dimensions

List dimensions for a metric.

**Permission**: `metric.read`

---

### POST /api/metrics/:id/dimensions

Add a dimension to a metric.

**Request Body** (JSON):
```json
{
  "name": "brand",
  "description": "Vehicle brand dimension"
}
```

**Permission**: `metric.write`

---

### DELETE /api/metrics/:id/dimensions/:dim_id

Remove a dimension from a metric.

**Permission**: `metric.write`

---

### GET /api/metrics/:id/filters

List filters for a metric.

**Permission**: `metric.read`

---

### POST /api/metrics/:id/filters

Add a filter to a metric.

**Request Body** (JSON):
```json
{
  "name": "active_only",
  "expression": "status != 'cancelled'"
}
```

**Permission**: `metric.write`

---

### DELETE /api/metrics/:id/filters/:filter_id

Remove a filter from a metric.

**Permission**: `metric.write`

---

### GET /api/metrics/:id/dependencies

List metrics that this metric depends on.

**Permission**: `metric.read`

---

### POST /api/metrics/:id/dependencies

Add a dependency to a metric.

**Request Body** (JSON):
```json
{
  "depends_on_metric": 1
}
```

**Permission**: `metric.write`

---

### DELETE /api/metrics/:id/dependencies/:dep_id

Remove a dependency from a metric.

**Permission**: `metric.write`

---

### POST /api/metrics/:id/impact-analysis

Run an impact analysis for a metric (required before activation).

**Response**:
```json
{
  "ok": true,
  "data": {
    "dependent_metrics": 2,
    "dependent_charts": 1,
    "missing_deps": 0,
    "status": "approved"
  }
}
```

**Permission**: `metric.activate`

---

### POST /api/metrics/:id/activate

Activate a metric definition (requires prior approved impact analysis).

**Response**:
```json
{
  "ok": true,
  "message": "Metric activated"
}
```

**Validation**:
- Impact analysis must have been run and approved
- All metric dependencies must be resolved

**Permission**: `metric.activate`

---

### GET /api/metrics/:id/lineage

Get the dependency lineage graph for a metric (includes metrics and charts).

**Response**:
```json
{
  "ok": true,
  "data": [
    {
      "source_type": "metric",
      "source_id": 1,
      "source_name": "Total Orders",
      "target_type": "chart",
      "target_id": 1,
      "target_name": "Order Volume"
    }
  ]
}
```

**Permission**: `metric.read`

---

## 9. Audit Endpoints

### GET /api/audit

List audit log entries with filtering.

**Query Parameters**:
| Param | Type | Description |
|-------|------|-------------|
| `entity_type` | string | Filter by entity type (e.g., `order`, `cart`, `vehicle_model`) |
| `entity_id` | int | Filter by entity ID |
| `page` | int | Page number |
| `per_page` | int | Items per page |

**Response**:
```json
{
  "ok": true,
  "data": {
    "entries": [
      {
        "id": 1,
        "entity_type": "order",
        "entity_id": 1,
        "action": "transition",
        "actor_user_id": 1,
        "actor_role": "administrator",
        "occurred_at": "2025-01-01T00:00:00Z",
        "before_json": { "status": "created" },
        "after_json": { "status": "payment_recorded" },
        "metadata_json": { "reason": "Payment received" }
      }
    ],
    "total": 50,
    "page": 1
  }
}
```

**Permission**: `audit.read`

---

### GET /api/audit/:entity_type/:entity_id

Get audit trail for a specific entity.

**Response**: Same structure as general audit list, filtered to the specific entity.

**Permission**: `audit.read`

---

## 10. Page Routes

These routes serve server-rendered HTML pages (Templ + Go templates).

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/login` | Public | Login page |
| POST | `/login` | Public | Login form submission |
| POST | `/logout` | Authenticated | Logout |
| GET | `/` | `dashboard.read` | Dashboard |
| GET | `/catalog` | `catalog.read` | Catalog listing |
| GET | `/catalog/new` | `catalog.write` | New model form |
| GET | `/catalog/import` | `catalog.import` | Import modal |
| GET | `/catalog/:id` | `catalog.read` | Model detail |
| GET | `/catalog/:id/edit` | `catalog.write` | Model edit form |
| GET | `/cart` | `cart.read` | Cart listing |
| GET | `/cart/:id` | `cart.read` | Cart detail |
| GET | `/cart/:id/merge-modal` | `cart.merge` | Merge cart modal |
| GET | `/cart/:id/add-item` | `cart.write` | Add item modal |
| GET | `/orders` | `order.read` | Orders listing |
| GET | `/orders/:id` | `order.read` | Order detail with timeline |
| GET | `/orders/:id/split-modal` | `order.split` | Split order modal |
| GET | `/notifications` | `notification.read` | Notification center |
| GET | `/alerts` | `alert.read` | Alert dashboard |
| GET | `/alerts/:id/close-modal` | `alert.manage` | Close alert modal |
| GET | `/metrics` | `metric.read` | Metrics listing |
| GET | `/metrics/:id` | `metric.read` | Metric detail |
| GET | `/metrics/new` | `metric.write` | Create metric modal |
| GET | `/audit` | `audit.read` | Audit log viewer |

---

## 11. Static Assets

| Path | Description |
|------|-------------|
| `/static/css/style.css` | Application stylesheet |
| `/static/js/htmx.min.js` | HTMX library for progressive enhancement |
| `/uploads/*` | User-uploaded media files |

---

## 12. Role-Permission Matrix

| Permission | Administrator | Inventory Manager | Sales Associate | Auditor |
|------------|:---:|:---:|:---:|:---:|
| `users.manage` | x | | | |
| `catalog.read` | x | x | x | x |
| `catalog.write` | x | x | | |
| `catalog.publish` | x | x | | |
| `catalog.import` | x | x | | |
| `media.upload` | x | x | | |
| `cart.read` | x | | x | x |
| `cart.write` | x | | x | |
| `cart.merge` | x | | x | |
| `order.read` | x | x | x | x |
| `order.create` | x | | x | |
| `order.transition` | x | x | | |
| `order.notes` | x | | x | |
| `order.payment` | x | | | |
| `order.split` | x | | | |
| `notification.read` | x | x | x | x |
| `notification.manage` | x | | | |
| `alert.read` | x | x | | x |
| `alert.manage` | x | x | | |
| `audit.read` | x | | | x |
| `metric.read` | x | | | x |
| `metric.write` | x | | | |
| `metric.activate` | x | | | |
| `dashboard.read` | x | x | x | x |
| `system.config` | x | | | |
