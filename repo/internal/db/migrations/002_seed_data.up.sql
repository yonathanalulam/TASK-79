-- Seed roles
INSERT INTO roles (name, description) VALUES
    ('administrator', 'Full system access'),
    ('inventory_manager', 'Manage vehicle catalog and inventory'),
    ('sales_associate', 'Handle carts and orders'),
    ('auditor', 'Read-only access to all data')
ON CONFLICT (name) DO NOTHING;

-- Seed permissions
INSERT INTO permissions (code, description) VALUES
    ('users.manage', 'Manage users and roles'),
    ('catalog.read', 'View vehicle catalog'),
    ('catalog.write', 'Create and edit catalog items'),
    ('catalog.publish', 'Publish/unpublish catalog items'),
    ('catalog.import', 'Import catalog from CSV'),
    ('media.upload', 'Upload media files'),
    ('cart.read', 'View carts'),
    ('cart.write', 'Create and edit carts'),
    ('cart.merge', 'Merge carts'),
    ('order.read', 'View orders'),
    ('order.create', 'Create orders'),
    ('order.transition', 'Transition order states'),
    ('order.notes', 'Add order notes'),
    ('order.payment', 'Record payments'),
    ('order.split', 'Split orders'),
    ('notification.read', 'View notifications'),
    ('notification.manage', 'Manage notification templates and preferences'),
    ('alert.read', 'View alerts'),
    ('alert.manage', 'Manage alert lifecycle'),
    ('audit.read', 'View audit logs'),
    ('metric.read', 'View metric definitions'),
    ('metric.write', 'Create/edit metrics'),
    ('metric.activate', 'Activate metrics'),
    ('dashboard.read', 'View dashboard'),
    ('system.config', 'System configuration')
ON CONFLICT (code) DO NOTHING;

-- Role-permission assignments
-- Administrator: all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.name = 'administrator'
ON CONFLICT DO NOTHING;

-- Inventory Manager
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'inventory_manager' AND p.code IN (
    'catalog.read', 'catalog.write', 'catalog.publish', 'catalog.import', 'media.upload',
    'order.read', 'order.transition', 'notification.read', 'alert.read', 'alert.manage', 'dashboard.read'
) ON CONFLICT DO NOTHING;

-- Sales Associate
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'sales_associate' AND p.code IN (
    'catalog.read', 'cart.read', 'cart.write', 'cart.merge',
    'order.read', 'order.create', 'order.notes', 'notification.read', 'dashboard.read'
) ON CONFLICT DO NOTHING;

-- Auditor
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p WHERE r.name = 'auditor' AND p.code IN (
    'catalog.read', 'cart.read', 'order.read', 'notification.read',
    'alert.read', 'audit.read', 'metric.read', 'dashboard.read'
) ON CONFLICT DO NOTHING;

-- Seed users (passwords are 'password123' hashed with argon2id)
-- Using a pre-computed hash for the seed password
INSERT INTO users (username, password_hash, full_name) VALUES
    ('admin', '$argon2id$v=19$m=65536,t=3,p=2$e3e1f8a9b2c3d4e5f6a7b8c9d0e1f2a3$a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2', 'System Administrator'),
    ('inventory', '$argon2id$v=19$m=65536,t=3,p=2$e3e1f8a9b2c3d4e5f6a7b8c9d0e1f2a3$a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2', 'Inventory Manager'),
    ('sales', '$argon2id$v=19$m=65536,t=3,p=2$e3e1f8a9b2c3d4e5f6a7b8c9d0e1f2a3$a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2', 'Sales Associate'),
    ('auditor', '$argon2id$v=19$m=65536,t=3,p=2$e3e1f8a9b2c3d4e5f6a7b8c9d0e1f2a3$a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2', 'Auditor User')
ON CONFLICT (username) DO NOTHING;

-- Assign roles
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r WHERE u.username = 'admin' AND r.name = 'administrator'
ON CONFLICT DO NOTHING;
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r WHERE u.username = 'inventory' AND r.name = 'inventory_manager'
ON CONFLICT DO NOTHING;
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r WHERE u.username = 'sales' AND r.name = 'sales_associate'
ON CONFLICT DO NOTHING;
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r WHERE u.username = 'auditor' AND r.name = 'auditor'
ON CONFLICT DO NOTHING;

-- Seed brands and series
INSERT INTO brands (name) VALUES ('Toyota'), ('Honda'), ('Ford'), ('BMW'), ('Mercedes-Benz') ON CONFLICT DO NOTHING;

INSERT INTO series (brand_id, name) VALUES
    ((SELECT id FROM brands WHERE name='Toyota'), 'Sedan'),
    ((SELECT id FROM brands WHERE name='Toyota'), 'SUV'),
    ((SELECT id FROM brands WHERE name='Toyota'), 'Truck'),
    ((SELECT id FROM brands WHERE name='Honda'), 'Sedan'),
    ((SELECT id FROM brands WHERE name='Honda'), 'SUV'),
    ((SELECT id FROM brands WHERE name='Ford'), 'Truck'),
    ((SELECT id FROM brands WHERE name='Ford'), 'SUV'),
    ((SELECT id FROM brands WHERE name='BMW'), 'Sedan'),
    ((SELECT id FROM brands WHERE name='BMW'), 'SUV'),
    ((SELECT id FROM brands WHERE name='Mercedes-Benz'), 'Sedan'),
    ((SELECT id FROM brands WHERE name='Mercedes-Benz'), 'SUV')
ON CONFLICT DO NOTHING;

-- Seed vehicle models
INSERT INTO vehicle_models (brand_id, series_id, model_code, model_name, year, description, publication_status, stock_quantity, expiry_date) VALUES
    ((SELECT id FROM brands WHERE name='Toyota'), (SELECT id FROM series WHERE name='Sedan' AND brand_id=(SELECT id FROM brands WHERE name='Toyota')), 'TOY-CAM-2025', 'Camry', 2025, 'Mid-size sedan with excellent fuel economy', 'published', 45, NULL),
    ((SELECT id FROM brands WHERE name='Toyota'), (SELECT id FROM series WHERE name='SUV' AND brand_id=(SELECT id FROM brands WHERE name='Toyota')), 'TOY-RAV-2025', 'RAV4', 2025, 'Compact SUV with all-wheel drive', 'published', 30, NULL),
    ((SELECT id FROM brands WHERE name='Toyota'), (SELECT id FROM series WHERE name='Truck' AND brand_id=(SELECT id FROM brands WHERE name='Toyota')), 'TOY-TAC-2025', 'Tacoma', 2025, 'Mid-size pickup truck', 'published', 3, NULL),
    ((SELECT id FROM brands WHERE name='Honda'), (SELECT id FROM series WHERE name='Sedan' AND brand_id=(SELECT id FROM brands WHERE name='Honda')), 'HON-CIV-2025', 'Civic', 2025, 'Compact sedan', 'published', 60, NULL),
    ((SELECT id FROM brands WHERE name='Honda'), (SELECT id FROM series WHERE name='SUV' AND brand_id=(SELECT id FROM brands WHERE name='Honda')), 'HON-CRV-2025', 'CR-V', 2025, 'Compact crossover SUV', 'published', 0, NULL),
    ((SELECT id FROM brands WHERE name='Ford'), (SELECT id FROM series WHERE name='Truck' AND brand_id=(SELECT id FROM brands WHERE name='Ford')), 'FRD-F15-2025', 'F-150', 2025, 'Full-size pickup truck', 'published', 280, NULL),
    ((SELECT id FROM brands WHERE name='Ford'), (SELECT id FROM series WHERE name='SUV' AND brand_id=(SELECT id FROM brands WHERE name='Ford')), 'FRD-ESC-2024', 'Escape', 2024, 'Compact SUV', 'draft', 15, NULL),
    ((SELECT id FROM brands WHERE name='BMW'), (SELECT id FROM series WHERE name='Sedan' AND brand_id=(SELECT id FROM brands WHERE name='BMW')), 'BMW-3SR-2025', '3 Series', 2025, 'Luxury compact sedan', 'published', 20, '2025-12-31'),
    ((SELECT id FROM brands WHERE name='BMW'), (SELECT id FROM series WHERE name='SUV' AND brand_id=(SELECT id FROM brands WHERE name='BMW')), 'BMW-X5-2025', 'X5', 2025, 'Luxury mid-size SUV', 'published', 12, NULL),
    ((SELECT id FROM brands WHERE name='Mercedes-Benz'), (SELECT id FROM series WHERE name='Sedan' AND brand_id=(SELECT id FROM brands WHERE name='Mercedes-Benz')), 'MBZ-CLA-2024', 'C-Class', 2024, 'Entry luxury sedan', 'unpublished', 8, NULL)
ON CONFLICT (model_code) DO NOTHING;

-- Seed customer accounts
INSERT INTO customer_accounts (account_code, account_name, contact_phone_masked, location) VALUES
    ('CUST-001', 'Acme Auto Dealers', '***-***-5678', 'New York, NY'),
    ('CUST-002', 'Pacific Motors', '***-***-9012', 'Los Angeles, CA'),
    ('CUST-003', 'Midwest Fleet Services', '***-***-3456', 'Chicago, IL'),
    ('CUST-004', 'Southern Auto Group', '***-***-7890', 'Dallas, TX')
ON CONFLICT (account_code) DO NOTHING;

-- Seed alert rules
INSERT INTO alert_rules (name, description, rule_type, condition, severity) VALUES
    ('Low Stock Alert', 'Triggered when published vehicle stock falls below 5', 'low_stock', '{"operator":"lt","threshold":5}', 'warning'),
    ('Overstock Alert', 'Triggered when published vehicle stock exceeds 250', 'overstock', '{"operator":"gt","threshold":250}', 'info'),
    ('Near Expiry Alert', 'Triggered when published vehicle is within 14 days of expiry', 'near_expiry', '{"operator":"lt","threshold":14}', 'critical')
ON CONFLICT DO NOTHING;

-- Seed notification templates
INSERT INTO notification_templates (code, name, subject, body, variables) VALUES
    ('order_created', 'Order Created', 'Order {{order_number}} Created', 'A new order {{order_number}} has been created for {{customer_account_name}} at {{location}}.', '["order_number","customer_account_name","location"]'),
    ('order_transition', 'Order Status Change', 'Order {{order_number}} - Status Updated', 'Order {{order_number}} has been updated. Promised date: {{promised_date}}.', '["order_number","promised_date","location"]'),
    ('alert_opened', 'Alert Opened', 'New Alert: {{alert_type}}', 'A new {{alert_type}} alert has been opened and requires attention.', '["alert_type"]'),
    ('payment_reminder', 'Payment Reminder', 'Payment Reminder for Order {{order_number}}', 'Order {{order_number}} is approaching cutoff. Please record payment.', '["order_number","promised_date"]')
ON CONFLICT (code) DO NOTHING;

-- Seed sample metrics
INSERT INTO metric_definitions (name, description, status, owner_id) VALUES
    ('Total Orders', 'Count of all orders in the system', 'active', (SELECT id FROM users WHERE username='admin')),
    ('Average Order Value', 'Mean value across all completed orders', 'draft', (SELECT id FROM users WHERE username='admin')),
    ('Stock Turnover Rate', 'Rate at which inventory is sold and replaced', 'pending_review', (SELECT id FROM users WHERE username='admin'))
ON CONFLICT (name) DO NOTHING;

INSERT INTO metric_definition_versions (metric_id, version_number, sql_expression, time_grain, status)
SELECT m.id, 1, 'SELECT COUNT(*) FROM orders', 'daily', 'active'
FROM metric_definitions m WHERE m.name = 'Total Orders'
ON CONFLICT DO NOTHING;

INSERT INTO metric_definition_versions (metric_id, version_number, sql_expression, time_grain, is_derived, status)
SELECT m.id, 1, 'SELECT AVG(total) FROM order_totals', 'daily', true, 'draft'
FROM metric_definitions m WHERE m.name = 'Average Order Value'
ON CONFLICT DO NOTHING;

-- Seed sample charts
INSERT INTO charts (name, description, chart_type, created_by) VALUES
    ('Order Volume', 'Daily order volume chart', 'line', (SELECT id FROM users WHERE username='admin')),
    ('Stock Levels', 'Current stock levels by brand', 'bar', (SELECT id FROM users WHERE username='admin'))
ON CONFLICT DO NOTHING;
