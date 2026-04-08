-- Remove seed data (in reverse order of dependencies)
DELETE FROM chart_metric_dependencies;
DELETE FROM charts;
DELETE FROM metric_definition_versions;
DELETE FROM metric_definitions;
DELETE FROM notification_templates;
DELETE FROM alert_rules;
DELETE FROM customer_accounts;
DELETE FROM vehicle_models;
DELETE FROM series;
DELETE FROM brands;
DELETE FROM user_roles;
DELETE FROM users;
DELETE FROM role_permissions;
DELETE FROM permissions;
DELETE FROM roles;
