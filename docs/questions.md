# Project Clarification Questions

## Business Logic Questions Log

---

### 1. Frontend Rendering Approach

**Question:** What rendering strategy should the Templ-rendered web UI use—server-side only, or a hybrid with client-side interactivity?
**My Understanding:** The prompt says "Templ-rendered web UI" and mentions "no page reload surprises," but does not specify whether to use a full SPA framework or server-rendered HTML with progressive enhancement.
**Solution:** Implement a dual rendering system: Templ (Go template engine v0.3) generates server-side HTML components (layout, dashboard, catalog, cart, orders, notifications), supplemented with Go `html/template` for additional views (alerts, audit, metrics, catalog detail/edit). HTMX is included for in-page interactions without full page reloads. Flash messages via cookies provide success/error banners.

---

### 2. Password Hashing Algorithm

**Question:** What specific password hashing algorithm should be used for "salted and hashed" passwords?
**My Understanding:** The prompt requires passwords to be "salted and hashed" but does not name a specific algorithm. The choice affects security strength and performance.
**Solution:** Implement Argon2id password hashing (via `golang.org/x/crypto/argon2`) with parameters: 64 MB memory, 3 iterations, 2 parallelism lanes, 16-byte salt, 32-byte key length. Passwords are stored in the format `$argon2id$v=19$m=65536,t=3,p=2$<salt_hex>$<hash_hex>`. Verification uses constant-time byte comparison to prevent timing attacks.

---

### 3. Encryption Algorithm for Sensitive Fields

**Question:** What encryption algorithm should be used for encrypting sensitive fields like customer phone numbers at rest?
**My Understanding:** The prompt requires "sensitive fields such as customer phone numbers are masked in the UI and encrypted at rest" but does not specify the cipher.
**Solution:** Implement AES-256-GCM encryption using the `crypto/aes` and `crypto/cipher` standard library packages. The encryption key is a 32-byte hex string provided via the `ENCRYPTION_KEY` environment variable. Each encrypted field stores both the ciphertext (`BYTEA`) and nonce (`BYTEA`) in separate database columns, alongside a pre-computed masked representation (e.g., `***-***-1234`) for UI display without decryption.

---

### 4. Session Management Strategy

**Question:** How should user sessions be managed—JWT tokens, server-side sessions, or cookie-based sessions?
**My Understanding:** The prompt requires "local username and password" sign-in but does not specify the session mechanism.
**Solution:** Implement server-side sessions stored in PostgreSQL. On login, a cryptographically random 64-character hex session ID is generated, stored in the `sessions` table with a 24-hour expiry, and sent to the client as a cookie. Session validation queries the database on each authenticated request. Logout destroys the session record. Expired sessions are cleaned up by the `CleanExpiredSessions` method.

---

### 5. Order State Machine Transitions and Terminal States

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

### 6. Cutoff Mechanism Details

**Question:** How exactly should the automatic cutoff work—what triggers it, and what happens to the order?
**My Understanding:** The prompt says "cutoff occurs automatically 30 minutes after creation unless payment-recorded is posted by an authorized role" but does not detail the implementation mechanism.
**Solution:** Implement a scheduled job (via the `Scheduler` component) that runs at a configurable interval (default 60 seconds). The `ProcessCutoffs` method queries for all orders in `created` status where `created_at + 30 minutes < NOW()` and no `payment_recorded_at` has been set. These orders are automatically transitioned to `cutoff` status with the actor recorded as `system`. Each transition writes to `order_state_history` with `actor_type = 'system'`.

---

### 7. Partial Out-of-Stock and Order Split Behavior

**Question:** How should partial stock allocation and order splitting work during checkout and fulfillment?
**My Understanding:** The prompt mentions "partial out-of-stock handling that allows backorder lines and optional order split for immediate fulfillment" but does not define the allocation algorithm or split mechanics.
**Solution:** During order creation from cart checkout, each line item is evaluated against current stock (with `FOR UPDATE` row locking). If stock is sufficient, the full quantity is allocated and stock is deducted. If stock is insufficient, available stock is allocated and the remainder is marked as backordered. Line statuses are set to `allocated`, `backordered`, or `partial`. If any line has a backorder, the order status can transition to `partially_backordered`. The split operation creates a new child order (linked via `split_parent_order_id`) containing only the backordered quantities, while the original order retains allocated quantities and transitions to `split` status. Order lines track `quantity_requested`, `quantity_allocated`, `quantity_backordered`, and snapshot the stock level and publication status at time of creation.


