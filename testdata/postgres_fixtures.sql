-- PostgreSQL fixtures: Platform & system administration data.
-- Purpose: Core platform operations — admin accounts, billing,
--          subscriptions, sessions, feature flags, and audit trails.

DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS admin_sessions CASCADE;
DROP TABLE IF EXISTS api_keys CASCADE;
DROP TABLE IF EXISTS invoices CASCADE;
DROP TABLE IF EXISTS subscriptions CASCADE;
DROP TABLE IF EXISTS plans CASCADE;
DROP TABLE IF EXISTS platform_settings CASCADE;
DROP TABLE IF EXISTS feature_flags CASCADE;
DROP TABLE IF EXISTS admins CASCADE;

-- ─── Platform Plans ─────────────────────────────────────────────────

CREATE TABLE plans (
    id              SERIAL PRIMARY KEY,
    slug            VARCHAR(50) UNIQUE NOT NULL,
    name            VARCHAR(100) NOT NULL,
    description     TEXT,
    price_cents     INT NOT NULL DEFAULT 0,
    billing_period  VARCHAR(20) NOT NULL DEFAULT 'monthly',
    max_projects    INT NOT NULL DEFAULT 1,
    max_members     INT NOT NULL DEFAULT 1,
    max_storage_mb  INT NOT NULL DEFAULT 512,
    max_bandwidth_mb INT NOT NULL DEFAULT 5120,
    features        JSONB NOT NULL DEFAULT '{}',
    active          BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO plans (id, slug, name, description, price_cents, billing_period, max_projects, max_members, max_storage_mb, max_bandwidth_mb, features) VALUES
    (1, 'free',       'Free',       'For hobby projects and experimentation',      0,     'monthly', 2,   1,   512,    5120,    '{"ssl": true, "custom_domain": false, "support": "community", "sla": null}'),
    (2, 'starter',    'Starter',    'For small teams getting started',             2900,  'monthly', 5,   5,   5120,   51200,   '{"ssl": true, "custom_domain": true, "support": "email", "sla": "99.5%"}'),
    (3, 'pro',        'Pro',        'For growing teams with production workloads', 9900,  'monthly', 25,  25,  51200,  512000,  '{"ssl": true, "custom_domain": true, "support": "priority", "sla": "99.9%", "audit_log": true}'),
    (4, 'enterprise', 'Enterprise', 'Custom solutions for large organizations',    49900, 'monthly', -1,  -1,  -1,     -1,      '{"ssl": true, "custom_domain": true, "support": "dedicated", "sla": "99.99%", "audit_log": true, "sso": true, "vpc_peering": true}'),
    (5, 'starter-yr', 'Starter Annual', 'Starter plan billed annually',            29000, 'yearly',  5,   5,   5120,   51200,   '{"ssl": true, "custom_domain": true, "support": "email", "sla": "99.5%"}'),
    (6, 'pro-yr',     'Pro Annual',     'Pro plan billed annually',                99000, 'yearly',  25,  25,  51200,  512000,  '{"ssl": true, "custom_domain": true, "support": "priority", "sla": "99.9%", "audit_log": true}');

-- ─── Admin Accounts ─────────────────────────────────────────────────

CREATE TABLE admins (
    id              SERIAL PRIMARY KEY,
    email           VARCHAR(255) UNIQUE NOT NULL,
    name            VARCHAR(255) NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    role            VARCHAR(50) NOT NULL DEFAULT 'admin',
    mfa_enabled     BOOLEAN NOT NULL DEFAULT false,
    mfa_secret      VARCHAR(255),
    avatar_url      TEXT,
    last_login_at   TIMESTAMPTZ,
    locked_at       TIMESTAMPTZ,
    failed_attempts INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO admins (id, email, name, password_hash, role, mfa_enabled, last_login_at) VALUES
    (1,  'root@gopgbase.io',       'Root Admin',       '$2a$12$LJ3m4ys7Gk.PLACEHOLDER.rootadmin',       'superadmin', true,  '2026-03-24 08:15:00+00'),
    (2,  'alice@gopgbase.io',      'Alice Chen',       '$2a$12$LJ3m4ys7Gk.PLACEHOLDER.alicechen',       'admin',      true,  '2026-03-24 09:30:00+00'),
    (3,  'bob@gopgbase.io',        'Bob Martinez',     '$2a$12$LJ3m4ys7Gk.PLACEHOLDER.bobmartin',       'admin',      false, '2026-03-23 14:20:00+00'),
    (4,  'carol@gopgbase.io',      'Carol Williams',   '$2a$12$LJ3m4ys7Gk.PLACEHOLDER.carolwil',        'billing',    false, '2026-03-22 11:00:00+00'),
    (5,  'dave@gopgbase.io',       'Dave Johnson',     '$2a$12$LJ3m4ys7Gk.PLACEHOLDER.davejohn',        'support',    false, '2026-03-24 07:45:00+00'),
    (6,  'eve@gopgbase.io',        'Eve Thompson',     '$2a$12$LJ3m4ys7Gk.PLACEHOLDER.evethom',         'readonly',   false, '2026-03-20 16:00:00+00'),
    (7,  'frank@gopgbase.io',      'Frank Garcia',     '$2a$12$LJ3m4ys7Gk.PLACEHOLDER.frankgar',        'admin',      true,  '2026-03-24 10:00:00+00'),
    (8,  'grace@gopgbase.io',      'Grace Kim',        '$2a$12$LJ3m4ys7Gk.PLACEHOLDER.gracekim',        'support',    false, '2026-03-23 09:15:00+00');

-- Ensure sequence values are advanced after manual PK inserts to avoid nnextval collision
SELECT setval('admins_id_seq', COALESCE((SELECT MAX(id) FROM admins), 0) + 1, false);

-- ─── Admin Sessions ─────────────────────────────────────────────────

CREATE TABLE admin_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id        INT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    token_hash      VARCHAR(255) NOT NULL,
    ip_address      INET NOT NULL,
    user_agent      TEXT,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_sessions_admin_id ON admin_sessions(admin_id);
CREATE INDEX idx_admin_sessions_expires ON admin_sessions(expires_at);

INSERT INTO admin_sessions (admin_id, token_hash, ip_address, user_agent, expires_at) VALUES
    (1, 'sha256$a1b2c3d4e5f6...placeholder01', '10.0.0.1',     'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)',  NOW() + INTERVAL '24 hours'),
    (2, 'sha256$a1b2c3d4e5f6...placeholder02', '10.0.0.2',     'Mozilla/5.0 (Windows NT 10.0; Win64; x64)',        NOW() + INTERVAL '24 hours'),
    (5, 'sha256$a1b2c3d4e5f6...placeholder03', '192.168.1.50', 'Mozilla/5.0 (X11; Linux x86_64)',                  NOW() + INTERVAL '12 hours'),
    (7, 'sha256$a1b2c3d4e5f6...placeholder04', '10.0.0.7',     'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)',  NOW() + INTERVAL '24 hours'),
    (1, 'sha256$a1b2c3d4e5f6...placeholder05', '172.16.0.5',   'curl/8.5.0',                                       NOW() + INTERVAL '1 hour'),
    (3, 'sha256$a1b2c3d4e5f6...placeholder06', '10.0.0.3',     'Mozilla/5.0 (Windows NT 10.0; Win64; x64)',        NOW() - INTERVAL '2 hours');

-- ─── Subscriptions ──────────────────────────────────────────────────

CREATE TABLE subscriptions (
    id                  SERIAL PRIMARY KEY,
    external_org_id     UUID NOT NULL,
    plan_id             INT NOT NULL REFERENCES plans(id),
    status              VARCHAR(30) NOT NULL DEFAULT 'active',
    stripe_customer_id  VARCHAR(255),
    stripe_sub_id       VARCHAR(255),
    current_period_start TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    current_period_end  TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '30 days',
    cancel_at           TIMESTAMPTZ,
    canceled_at         TIMESTAMPTZ,
    trial_end           TIMESTAMPTZ,
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_org ON subscriptions(external_org_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);

INSERT INTO subscriptions (id, external_org_id, plan_id, status, stripe_customer_id, stripe_sub_id, current_period_start, current_period_end, trial_end) VALUES
    (1,  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 4, 'active',   'cus_enterprise01', 'sub_enterprise01', '2026-03-01', '2026-04-01', NULL),
    (2,  'b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 3, 'active',   'cus_pro01',        'sub_pro01',        '2026-03-10', '2026-04-10', NULL),
    (3,  'c2aade11-be2d-6a1a-dd8f-8ddbdf5a2c33', 3, 'active',   'cus_pro02',        'sub_pro02',        '2026-03-15', '2026-04-15', NULL),
    (4,  'd3bbef22-cf3e-7b2b-eea0-9eece06b3d44', 2, 'active',   'cus_starter01',    'sub_starter01',    '2026-03-05', '2026-04-05', NULL),
    (5,  'e4ccf033-d04f-8c3c-ffb1-affd117c4e55', 2, 'trialing', 'cus_starter02',    'sub_starter02',    '2026-03-20', '2026-04-20', '2026-04-03'),
    (6,  'f5dda144-e150-9d4d-00c2-b00e228d5f66', 1, 'active',   NULL,               NULL,               '2026-03-01', '2026-04-01', NULL),
    (7,  '06eeb255-f261-ae5e-11d3-c11f339e6077', 1, 'active',   NULL,               NULL,               '2026-03-12', '2026-04-12', NULL),
    (8,  '17ffc366-0372-bf6f-22e4-d220440f7188', 6, 'active',   'cus_proyr01',      'sub_proyr01',      '2026-01-01', '2027-01-01', NULL),
    (9,  '2800d477-1483-c070-33f5-e3315510a299', 3, 'canceled', 'cus_pro03',        'sub_pro03',        '2026-02-15', '2026-03-15', NULL),
    (10, '3911e588-2594-d181-4406-f4426621b3aa', 2, 'past_due', 'cus_starter03',    'sub_starter03',    '2026-02-20', '2026-03-20', NULL);

-- ─── Invoices ───────────────────────────────────────────────────────

CREATE TABLE invoices (
    id                  SERIAL PRIMARY KEY,
    subscription_id     INT NOT NULL REFERENCES subscriptions(id),
    stripe_invoice_id   VARCHAR(255),
    amount_cents        INT NOT NULL,
    currency            VARCHAR(3) NOT NULL DEFAULT 'usd',
    status              VARCHAR(30) NOT NULL DEFAULT 'draft',
    period_start        TIMESTAMPTZ NOT NULL,
    period_end          TIMESTAMPTZ NOT NULL,
    paid_at             TIMESTAMPTZ,
    pdf_url             TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoices_subscription ON invoices(subscription_id);
CREATE INDEX idx_invoices_status ON invoices(status);

INSERT INTO invoices (subscription_id, stripe_invoice_id, amount_cents, status, period_start, period_end, paid_at) VALUES
    (1, 'in_enterprise_202601', 49900, 'paid',    '2026-01-01', '2026-02-01', '2026-01-01 00:05:00+00'),
    (1, 'in_enterprise_202602', 49900, 'paid',    '2026-02-01', '2026-03-01', '2026-02-01 00:05:00+00'),
    (1, 'in_enterprise_202603', 49900, 'paid',    '2026-03-01', '2026-04-01', '2026-03-01 00:05:00+00'),
    (2, 'in_pro01_202603',       9900, 'paid',    '2026-03-10', '2026-04-10', '2026-03-10 00:05:00+00'),
    (3, 'in_pro02_202603',       9900, 'paid',    '2026-03-15', '2026-04-15', '2026-03-15 00:05:00+00'),
    (4, 'in_starter01_202603',   2900, 'paid',    '2026-03-05', '2026-04-05', '2026-03-05 00:05:00+00'),
    (5, 'in_starter02_202603',   2900, 'open',    '2026-03-20', '2026-04-20', NULL),
    (8, 'in_proyr01_202601',    99000, 'paid',    '2026-01-01', '2027-01-01', '2026-01-01 00:05:00+00'),
    (9, 'in_pro03_202602',       9900, 'paid',    '2026-02-15', '2026-03-15', '2026-02-15 00:05:00+00'),
    (10,'in_starter03_202602',   2900, 'past_due','2026-02-20', '2026-03-20', NULL);

-- ─── API Keys ───────────────────────────────────────────────────────

CREATE TABLE api_keys (
    id              SERIAL PRIMARY KEY,
    admin_id        INT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    name            VARCHAR(100) NOT NULL,
    key_prefix      VARCHAR(10) NOT NULL,
    key_hash        VARCHAR(255) NOT NULL,
    scopes          TEXT[] NOT NULL DEFAULT '{}',
    last_used_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_admin ON api_keys(admin_id);
CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix);

INSERT INTO api_keys (admin_id, name, key_prefix, key_hash, scopes, last_used_at, expires_at) VALUES
    (1, 'CI/CD Pipeline',     'gpb_live_', 'sha256$apikey...placeholder01', '{read,write,deploy,admin}',  '2026-03-24 10:00:00+00', '2027-03-24'),
    (1, 'Monitoring Service',  'gpb_live_', 'sha256$apikey...placeholder02', '{read,metrics}',             '2026-03-24 09:58:00+00', '2027-03-24'),
    (2, 'Dashboard Access',    'gpb_live_', 'sha256$apikey...placeholder03', '{read,write}',               '2026-03-24 09:30:00+00', '2026-09-24'),
    (3, 'Build Automation',    'gpb_live_', 'sha256$apikey...placeholder04', '{read,write,deploy}',        '2026-03-23 14:20:00+00', '2026-09-23'),
    (5, 'Support Portal',     'gpb_live_', 'sha256$apikey...placeholder05', '{read}',                     '2026-03-24 07:45:00+00', '2026-06-24'),
    (7, 'Staging Deploys',    'gpb_test_', 'sha256$apikey...placeholder06', '{read,write,deploy}',        '2026-03-24 10:00:00+00', '2026-06-24');

-- ─── Platform Settings ──────────────────────────────────────────────

CREATE TABLE platform_settings (
    key         VARCHAR(100) PRIMARY KEY,
    value       JSONB NOT NULL,
    description TEXT,
    updated_by  INT REFERENCES admins(id),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO platform_settings (key, value, description, updated_by) VALUES
    ('maintenance_mode',        'false',                                                    'Enable platform-wide maintenance mode',        1),
    ('signup_enabled',          'true',                                                     'Allow new user registrations',                 1),
    ('default_plan',            '"free"',                                                   'Default plan for new organizations',           1),
    ('max_free_orgs_per_user',  '3',                                                        'Maximum free organizations per user',          1),
    ('smtp_config',             '{"host": "smtp.gopgbase.io", "port": 587, "tls": true}',  'Outbound email configuration',                 2),
    ('webhook_retry_policy',    '{"max_retries": 5, "backoff_ms": [1000, 2000, 4000, 8000, 16000]}', 'Webhook delivery retry policy',      2),
    ('rate_limit_default',      '{"requests_per_minute": 60, "burst": 10}',                'Default API rate limit',                       1),
    ('cdn_config',              '{"provider": "cloudflare", "zone_id": "abc123"}',          'CDN configuration',                            1),
    ('alert_emails',            '["ops@gopgbase.io", "oncall@gopgbase.io"]',                'Alert notification recipients',                1),
    ('password_policy',         '{"min_length": 12, "require_upper": true, "require_number": true, "require_symbol": true}', 'Password requirements', 1);

-- ─── Feature Flags ──────────────────────────────────────────────────

CREATE TABLE feature_flags (
    id              SERIAL PRIMARY KEY,
    slug            VARCHAR(100) UNIQUE NOT NULL,
    description     TEXT,
    enabled         BOOLEAN NOT NULL DEFAULT false,
    rollout_pct     INT NOT NULL DEFAULT 0,
    allowed_orgs    UUID[] NOT NULL DEFAULT '{}',
    allowed_plans   TEXT[] NOT NULL DEFAULT '{}',
    created_by      INT REFERENCES admins(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO feature_flags (slug, description, enabled, rollout_pct, allowed_plans, created_by) VALUES
    ('gpu_instances',       'GPU compute instances',                        false, 0,   '{enterprise}',                 1),
    ('edge_functions',      'Edge function deployments',                    true,  100, '{pro,enterprise,pro-yr}',      1),
    ('preview_deployments', 'PR preview deployments',                       true,  100, '{starter,pro,enterprise,starter-yr,pro-yr}', 2),
    ('custom_domains_v2',   'New custom domain management system',          true,  50,  '{starter,pro,enterprise}',     2),
    ('ipv6_support',        'IPv6 networking for deployments',              true,  25,  '{}',                           7),
    ('ai_log_analysis',     'AI-powered log analysis and anomaly detection',false, 0,   '{enterprise}',                 1),
    ('multi_region_db',     'Multi-region database replication',            true,  100, '{pro,enterprise,pro-yr}',      1),
    ('websocket_support',   'WebSocket connections in deployments',         true,  100, '{}',                           2),
    ('build_cache_v2',      'Next-gen build caching system',                true,  10,  '{}',                           7),
    ('usage_based_billing', 'Pay-per-use billing model',                    false, 0,   '{}',                           4);

-- ─── Audit Logs ─────────────────────────────────────────────────────

CREATE TABLE audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    admin_id        INT REFERENCES admins(id),
    action          VARCHAR(100) NOT NULL,
    resource_type   VARCHAR(50) NOT NULL,
    resource_id     VARCHAR(255),
    details         JSONB NOT NULL DEFAULT '{}',
    ip_address      INET,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_admin ON audit_logs(admin_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_logs_created ON audit_logs(created_at DESC);

INSERT INTO audit_logs (admin_id, action, resource_type, resource_id, details, ip_address, created_at) VALUES
    (1, 'login',              'admin',        '1',  '{"mfa": true}',                                              '10.0.0.1',     '2026-03-24 08:15:00+00'),
    (1, 'update_setting',     'setting',      'maintenance_mode', '{"old": true, "new": false}',                   '10.0.0.1',     '2026-03-24 08:16:00+00'),
    (2, 'login',              'admin',        '2',  '{"mfa": true}',                                              '10.0.0.2',     '2026-03-24 09:30:00+00'),
    (2, 'create_api_key',     'api_key',      '3',  '{"name": "Dashboard Access", "scopes": ["read","write"]}',   '10.0.0.2',     '2026-03-24 09:31:00+00'),
    (1, 'toggle_feature',     'feature_flag', 'edge_functions',   '{"enabled": true, "rollout_pct": 100}',        '10.0.0.1',     '2026-03-24 09:45:00+00'),
    (4, 'process_refund',     'invoice',      '10', '{"amount_cents": 2900, "reason": "payment_failure"}',         '10.0.0.4',     '2026-03-24 10:00:00+00'),
    (5, 'login',              'admin',        '5',  '{"mfa": false}',                                             '192.168.1.50', '2026-03-24 07:45:00+00'),
    (5, 'view_subscription',  'subscription', '10', '{"reason": "support_ticket_4521"}',                           '192.168.1.50', '2026-03-24 07:50:00+00'),
    (7, 'login',              'admin',        '7',  '{"mfa": true}',                                              '10.0.0.7',     '2026-03-24 10:00:00+00'),
    (7, 'deploy_hotfix',      'deployment',   'dpl_abc123', '{"version": "v2.14.3", "env": "production"}',         '10.0.0.7',     '2026-03-24 10:05:00+00'),
    (1, 'update_plan',        'plan',         '3',  '{"field": "max_members", "old": 20, "new": 25}',             '10.0.0.1',     '2026-03-23 15:00:00+00'),
    (3, 'login',              'admin',        '3',  '{"mfa": false}',                                             '10.0.0.3',     '2026-03-23 14:20:00+00'),
    (3, 'rotate_api_key',     'api_key',      '4',  '{"name": "Build Automation"}',                                '10.0.0.3',     '2026-03-23 14:25:00+00'),
    (1, 'suspend_org',        'organization', 'f5dda144-e150-9d4d-00c2-b00e228d5f66', '{"reason": "tos_violation"}','10.0.0.1',    '2026-03-22 18:00:00+00'),
    (1, 'unsuspend_org',      'organization', 'f5dda144-e150-9d4d-00c2-b00e228d5f66', '{"reason": "resolved"}',    '10.0.0.1',    '2026-03-22 20:00:00+00');

-- ─── Sequence sync for manual IDs ─────────────────────────────────────────

SELECT setval('plans_id_seq', COALESCE((SELECT MAX(id) FROM plans), 0) + 1, false);
SELECT setval('admins_id_seq', COALESCE((SELECT MAX(id) FROM admins), 0) + 1, false);
SELECT setval('subscriptions_id_seq', COALESCE((SELECT MAX(id) FROM subscriptions), 0) + 1, false);
SELECT setval('invoices_id_seq', COALESCE((SELECT MAX(id) FROM invoices), 0) + 1, false);
SELECT setval('api_keys_id_seq', COALESCE((SELECT MAX(id) FROM api_keys), 0) + 1, false);
SELECT setval('feature_flags_id_seq', COALESCE((SELECT MAX(id) FROM feature_flags), 0) + 1, false);
SELECT setval('audit_logs_id_seq', COALESCE((SELECT MAX(id) FROM audit_logs), 0) + 1, false);
