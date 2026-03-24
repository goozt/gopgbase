-- CockroachDB fixtures: Platform user data (multi-region ready).
-- Purpose: User accounts, organizations, projects, deployments,
--          environments, secrets, domains, and team membership.
-- Note: Uses explicit IDs to ensure foreign key consistency across
--       CockroachDB's unique_rowid() SERIAL behavior.

DROP TABLE IF EXISTS deployment_logs CASCADE;
DROP TABLE IF EXISTS custom_domains CASCADE;
DROP TABLE IF EXISTS secrets CASCADE;
DROP TABLE IF EXISTS deployments CASCADE;
DROP TABLE IF EXISTS environments CASCADE;
DROP TABLE IF EXISTS team_members CASCADE;
DROP TABLE IF EXISTS projects CASCADE;
DROP TABLE IF EXISTS org_invitations CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;
DROP TABLE IF EXISTS user_sessions CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- ─── Users ──────────────────────────────────────────────────────────

CREATE TABLE users (
    id              BIGINT PRIMARY KEY DEFAULT unique_rowid(),
    email           VARCHAR(255) UNIQUE NOT NULL,
    name            VARCHAR(255) NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    avatar_url      TEXT,
    provider        VARCHAR(50) NOT NULL DEFAULT 'email',
    provider_id     VARCHAR(255),
    email_verified  BOOLEAN NOT NULL DEFAULT false,
    mfa_enabled     BOOLEAN NOT NULL DEFAULT false,
    timezone        VARCHAR(50) NOT NULL DEFAULT 'UTC',
    locale          VARCHAR(10) NOT NULL DEFAULT 'en',
    metadata        JSONB NOT NULL DEFAULT '{}',
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

INSERT INTO users (id, email, name, password_hash, provider, email_verified, timezone, locale, last_login_at) VALUES
    (1, 'sarah@acmecorp.com',       'Sarah Connor',     '$2a$12$PLACEHOLDER.sarahconnor',   'github',  true,  'America/New_York',    'en', '2026-03-24 10:00:00+00'),
    (2, 'john@acmecorp.com',        'John Reese',       '$2a$12$PLACEHOLDER.johnreese',     'github',  true,  'America/New_York',    'en', '2026-03-24 09:30:00+00'),
    (3, 'root@acmecorp.com',        'Root Admin',       '$2a$12$PLACEHOLDER.rootadmin',     'email',   true,  'America/Chicago',     'en', '2026-03-23 18:00:00+00'),
    (4, 'yuki@tanakadev.jp',        'Yuki Tanaka',      '$2a$12$PLACEHOLDER.yukitanaka',    'google',  true,  'Asia/Tokyo',          'ja', '2026-03-24 08:00:00+00'),
    (5, 'hans@berlinlabs.de',       'Hans Mueller',     '$2a$12$PLACEHOLDER.hansmueller',   'gitlab',  true,  'Europe/Berlin',       'de', '2026-03-24 11:15:00+00'),
    (6, 'priya@cloudnative.in',     'Priya Sharma',     '$2a$12$PLACEHOLDER.priyasharma',   'github',  true,  'Asia/Kolkata',        'en', '2026-03-24 06:30:00+00'),
    (7, 'maria@startup.io',         'Maria Santos',     '$2a$12$PLACEHOLDER.mariasantos',   'email',   true,  'America/Sao_Paulo',   'pt', '2026-03-23 22:00:00+00'),
    (8, 'alex@devshop.co.uk',       'Alex Wright',      '$2a$12$PLACEHOLDER.alexwright',    'github',  true,  'Europe/London',       'en', '2026-03-24 09:00:00+00'),
    (9, 'chen@megascale.cn',        'Chen Wei',         '$2a$12$PLACEHOLDER.chenwei',       'email',   true,  'Asia/Shanghai',       'zh', '2026-03-24 07:45:00+00'),
    (10,'luna@freelance.dev',       'Luna Park',        '$2a$12$PLACEHOLDER.lunapark',      'google',  true,  'America/Los_Angeles', 'en', '2026-03-23 20:00:00+00'),
    (11,'omar@cloudops.ae',         'Omar Hassan',      '$2a$12$PLACEHOLDER.omarhassan',    'email',   false, 'Asia/Dubai',          'ar', '2026-03-22 14:00:00+00'),
    (12,'emma@startup.io',          'Emma Wilson',      '$2a$12$PLACEHOLDER.emmawilson',    'github',  true,  'America/Sao_Paulo',   'en', '2026-03-24 01:00:00+00');

-- ─── User Sessions ──────────────────────────────────────────────────

CREATE TABLE user_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      VARCHAR(255) NOT NULL,
    ip_address      VARCHAR(45) NOT NULL,
    user_agent      TEXT,
    region          VARCHAR(30),
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_sessions_user ON user_sessions(user_id);

INSERT INTO user_sessions (user_id, token_hash, ip_address, user_agent, region, expires_at) VALUES
    (1,  'sha256$sess...placeholder01', '203.0.113.10',  'Mozilla/5.0 Chrome/120',  'us-east-1',    NOW() + INTERVAL '7 days'),
    (2,  'sha256$sess...placeholder02', '203.0.113.11',  'Mozilla/5.0 Firefox/121', 'us-east-1',    NOW() + INTERVAL '7 days'),
    (4,  'sha256$sess...placeholder03', '198.51.100.20', 'Mozilla/5.0 Safari/17',   'ap-northeast-1',NOW() + INTERVAL '7 days'),
    (5,  'sha256$sess...placeholder04', '192.0.2.30',    'Mozilla/5.0 Chrome/120',  'eu-central-1', NOW() + INTERVAL '7 days'),
    (6,  'sha256$sess...placeholder05', '198.51.100.40', 'Mozilla/5.0 Chrome/120',  'ap-south-1',   NOW() + INTERVAL '7 days'),
    (8,  'sha256$sess...placeholder06', '192.0.2.50',    'Mozilla/5.0 Safari/17',   'eu-west-1',    NOW() + INTERVAL '7 days'),
    (9,  'sha256$sess...placeholder07', '198.51.100.60', 'Mozilla/5.0 Chrome/120',  'ap-east-1',    NOW() + INTERVAL '7 days'),
    (1,  'sha256$sess...placeholder08', '203.0.113.12',  'gopgbase-cli/1.5.0',      'us-east-1',    NOW() + INTERVAL '30 days');

-- ─── Organizations ──────────────────────────────────────────────────

CREATE TABLE organizations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            VARCHAR(100) UNIQUE NOT NULL,
    name            VARCHAR(255) NOT NULL,
    owner_id        BIGINT NOT NULL REFERENCES users(id),
    avatar_url      TEXT,
    billing_email   VARCHAR(255),
    region          VARCHAR(30) NOT NULL DEFAULT 'us-east-1',
    metadata        JSONB NOT NULL DEFAULT '{}',
    suspended_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_organizations_owner ON organizations(owner_id);

INSERT INTO organizations (id, slug, name, owner_id, billing_email, region) VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'acme-corp',      'Acme Corporation',     1, 'billing@acmecorp.com',    'us-east-1'),
    ('b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 'tanaka-dev',     'Tanaka Development',   4, 'billing@tanakadev.jp',    'ap-northeast-1'),
    ('c2aade11-be2d-6a1a-dd8f-8ddbdf5a2c33', 'berlin-labs',    'Berlin Labs GmbH',     5, 'finance@berlinlabs.de',   'eu-central-1'),
    ('d3bbef22-cf3e-7b2b-eea0-9eece06b3d44', 'cloudnative',    'CloudNative Solutions', 6, 'billing@cloudnative.in',  'ap-south-1'),
    ('e4ccf033-d04f-8c3c-ffb1-affd117c4e55', 'startup-io',     'Startup.io',           7, 'maria@startup.io',        'sa-east-1'),
    ('f5dda144-e150-9d4d-00c2-b00e228d5f66', 'luna-freelance',  'Luna Freelance',       10,'luna@freelance.dev',      'us-west-2'),
    ('06eeb255-f261-ae5e-11d3-c11f339e6077', 'megascale',       'MegaScale Inc',        9, 'finance@megascale.cn',    'ap-east-1'),
    ('17ffc366-0372-bf6f-22e4-d220440f7188', 'devshop-uk',      'DevShop UK',           8, 'billing@devshop.co.uk',   'eu-west-1');

-- ─── Org Invitations ────────────────────────────────────────────────

CREATE TABLE org_invitations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email           VARCHAR(255) NOT NULL,
    role            VARCHAR(30) NOT NULL DEFAULT 'member',
    invited_by      BIGINT NOT NULL REFERENCES users(id),
    accepted_at     TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '7 days',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO org_invitations (org_id, email, role, invited_by, accepted_at) VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'newdev@acmecorp.com',     'member',  1, NULL),
    ('c2aade11-be2d-6a1a-dd8f-8ddbdf5a2c33', 'intern@berlinlabs.de',    'viewer',  5, NULL),
    ('e4ccf033-d04f-8c3c-ffb1-affd117c4e55', 'emma@startup.io',         'admin',   7, '2026-03-22 10:00:00+00'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'contractor@external.com', 'viewer',  1, NULL);

-- ─── Team Members ───────────────────────────────────────────────────

CREATE TABLE team_members (
    id              BIGINT PRIMARY KEY DEFAULT unique_rowid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            VARCHAR(30) NOT NULL DEFAULT 'member',
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, user_id)
);

CREATE INDEX idx_team_members_org ON team_members(org_id);
CREATE INDEX idx_team_members_user ON team_members(user_id);

INSERT INTO team_members (org_id, user_id, role) VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 1, 'owner'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 2, 'admin'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 3, 'member'),
    ('b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 4, 'owner'),
    ('c2aade11-be2d-6a1a-dd8f-8ddbdf5a2c33', 5, 'owner'),
    ('d3bbef22-cf3e-7b2b-eea0-9eece06b3d44', 6, 'owner'),
    ('e4ccf033-d04f-8c3c-ffb1-affd117c4e55', 7, 'owner'),
    ('e4ccf033-d04f-8c3c-ffb1-affd117c4e55', 12,'admin'),
    ('f5dda144-e150-9d4d-00c2-b00e228d5f66', 10,'owner'),
    ('06eeb255-f261-ae5e-11d3-c11f339e6077', 9, 'owner'),
    ('17ffc366-0372-bf6f-22e4-d220440f7188', 8, 'owner'),
    ('17ffc366-0372-bf6f-22e4-d220440f7188', 11,'member');

-- ─── Projects ───────────────────────────────────────────────────────

CREATE TABLE projects (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug            VARCHAR(100) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    framework       VARCHAR(50),
    repo_url        TEXT,
    default_branch  VARCHAR(100) NOT NULL DEFAULT 'main',
    build_command   VARCHAR(500),
    output_dir      VARCHAR(255),
    root_dir        VARCHAR(255) NOT NULL DEFAULT '/',
    node_version    VARCHAR(20),
    region          VARCHAR(30) NOT NULL DEFAULT 'us-east-1',
    created_by      BIGINT NOT NULL REFERENCES users(id),
    archived_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, slug)
);

CREATE INDEX idx_projects_org ON projects(org_id);

INSERT INTO projects (id, org_id, slug, name, description, framework, repo_url, build_command, output_dir, region, created_by) VALUES
    ('10000000-0000-0000-0000-000000000001', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'web-app',        'Web Application',        'Main customer-facing web app',       'nextjs',   'https://github.com/acme/web-app',        'npm run build',   '.next',   'us-east-1',      1),
    ('10000000-0000-0000-0000-000000000002', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'api-gateway',    'API Gateway',            'REST and GraphQL gateway',           'go',       'https://github.com/acme/api-gateway',    'go build .',      'bin',     'us-east-1',      2),
    ('10000000-0000-0000-0000-000000000003', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'docs',           'Documentation',          'Public documentation site',          'astro',    'https://github.com/acme/docs',           'npm run build',   'dist',    'us-east-1',      1),
    ('10000000-0000-0000-0000-000000000004', 'b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 'tanaka-saas',    'Tanaka SaaS Platform',   'B2B SaaS product',                   'nuxt',     'https://github.com/tanaka/saas',         'npm run build',   '.output', 'ap-northeast-1', 4),
    ('10000000-0000-0000-0000-000000000005', 'b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 'tanaka-api',     'Tanaka API',             'Backend API service',                'go',       'https://github.com/tanaka/api',          'go build .',      'bin',     'ap-northeast-1', 4),
    ('10000000-0000-0000-0000-000000000006', 'c2aade11-be2d-6a1a-dd8f-8ddbdf5a2c33', 'iot-dashboard',  'IoT Dashboard',          'Real-time IoT monitoring',           'svelte',   'https://github.com/berlinlabs/iot',      'npm run build',   'build',   'eu-central-1',   5),
    ('10000000-0000-0000-0000-000000000007', 'd3bbef22-cf3e-7b2b-eea0-9eece06b3d44', 'cn-platform',    'CloudNative Platform',   'Multi-cloud deployment platform',    'react',    'https://github.com/cloudnative/platform','npm run build',   'dist',    'ap-south-1',     6),
    ('10000000-0000-0000-0000-000000000008', 'e4ccf033-d04f-8c3c-ffb1-affd117c4e55', 'startup-mvp',    'Startup MVP',            'Minimum viable product',             'nextjs',   'https://github.com/startup-io/mvp',      'npm run build',   '.next',   'sa-east-1',      7),
    ('10000000-0000-0000-0000-000000000009', 'f5dda144-e150-9d4d-00c2-b00e228d5f66', 'portfolio',      'Portfolio Site',          'Personal portfolio',                 'astro',    'https://github.com/luna/portfolio',       'npm run build',   'dist',    'us-west-2',      10),
    ('10000000-0000-0000-0000-000000000010', '06eeb255-f261-ae5e-11d3-c11f339e6077', 'mega-commerce',  'MegaScale Commerce',     'High-traffic e-commerce platform',   'nextjs',   'https://github.com/megascale/commerce',  'npm run build',   '.next',   'ap-east-1',      9),
    ('10000000-0000-0000-0000-000000000011', '17ffc366-0372-bf6f-22e4-d220440f7188', 'devshop-site',   'DevShop Website',        'Agency marketing website',           'gatsby',   'https://github.com/devshop/website',     'npm run build',   'public',  'eu-west-1',      8),
    ('10000000-0000-0000-0000-000000000012', '17ffc366-0372-bf6f-22e4-d220440f7188', 'client-portal',  'Client Portal',          'Customer self-service portal',       'react',    'https://github.com/devshop/portal',      'npm run build',   'dist',    'eu-west-1',      8);

-- ─── Environments ───────────────────────────────────────────────────

CREATE TABLE environments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    slug            VARCHAR(50) NOT NULL,
    name            VARCHAR(100) NOT NULL,
    branch          VARCHAR(100),
    auto_deploy     BOOLEAN NOT NULL DEFAULT true,
    url             TEXT,
    protected       BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, slug)
);

INSERT INTO environments (id, project_id, slug, name, branch, auto_deploy, url, protected) VALUES
    ('20000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'production',  'Production',  'main',    true,  'https://app.acmecorp.com',           true),
    ('20000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', 'staging',     'Staging',     'develop', true,  'https://staging.app.acmecorp.com',   false),
    ('20000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001', 'preview',     'Preview',     NULL,      false, NULL,                                  false),
    ('20000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000002', 'production',  'Production',  'main',    true,  'https://api.acmecorp.com',           true),
    ('20000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000002', 'staging',     'Staging',     'develop', true,  'https://staging.api.acmecorp.com',   false),
    ('20000000-0000-0000-0000-000000000006', '10000000-0000-0000-0000-000000000004', 'production',  'Production',  'main',    true,  'https://tanaka-saas.com',            true),
    ('20000000-0000-0000-0000-000000000007', '10000000-0000-0000-0000-000000000006', 'production',  'Production',  'main',    true,  'https://iot.berlinlabs.de',          true),
    ('20000000-0000-0000-0000-000000000008', '10000000-0000-0000-0000-000000000007', 'production',  'Production',  'main',    true,  'https://platform.cloudnative.in',    true),
    ('20000000-0000-0000-0000-000000000009', '10000000-0000-0000-0000-000000000008', 'production',  'Production',  'main',    true,  'https://app.startup.io',             true),
    ('20000000-0000-0000-0000-000000000010', '10000000-0000-0000-0000-000000000009', 'production',  'Production',  'main',    true,  'https://lunapark.dev',               true),
    ('20000000-0000-0000-0000-000000000011', '10000000-0000-0000-0000-000000000010', 'production',  'Production',  'main',    true,  'https://shop.megascale.cn',          true),
    ('20000000-0000-0000-0000-000000000012', '10000000-0000-0000-0000-000000000010', 'staging',     'Staging',     'develop', true,  'https://staging.shop.megascale.cn',  false);

-- ─── Secrets (encrypted values are placeholders) ────────────────────

CREATE TABLE secrets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    key             VARCHAR(255) NOT NULL,
    encrypted_value TEXT NOT NULL,
    created_by      BIGINT NOT NULL REFERENCES users(id),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(environment_id, key)
);

INSERT INTO secrets (environment_id, key, encrypted_value, created_by) VALUES
    ('20000000-0000-0000-0000-000000000001', 'DATABASE_URL',         'enc:v1:AAAADQxMjM0NTY3ODkw...prod_db',    1),
    ('20000000-0000-0000-0000-000000000001', 'REDIS_URL',            'enc:v1:AAAADQxMjM0NTY3ODkw...prod_redis', 1),
    ('20000000-0000-0000-0000-000000000001', 'STRIPE_SECRET_KEY',    'enc:v1:AAAADQxMjM0NTY3ODkw...stripe_sk',  1),
    ('20000000-0000-0000-0000-000000000001', 'JWT_SECRET',           'enc:v1:AAAADQxMjM0NTY3ODkw...jwt_prod',   2),
    ('20000000-0000-0000-0000-000000000002', 'DATABASE_URL',         'enc:v1:AAAADQxMjM0NTY3ODkw...stg_db',     1),
    ('20000000-0000-0000-0000-000000000002', 'REDIS_URL',            'enc:v1:AAAADQxMjM0NTY3ODkw...stg_redis',  1),
    ('20000000-0000-0000-0000-000000000004', 'DATABASE_URL',         'enc:v1:AAAADQxMjM0NTY3ODkw...api_db',     2),
    ('20000000-0000-0000-0000-000000000004', 'API_SIGNING_KEY',      'enc:v1:AAAADQxMjM0NTY3ODkw...api_sign',   2),
    ('20000000-0000-0000-0000-000000000006', 'DATABASE_URL',         'enc:v1:AAAADQxMjM0NTY3ODkw...tanaka_db',  4),
    ('20000000-0000-0000-0000-000000000006', 'OPENAI_API_KEY',       'enc:v1:AAAADQxMjM0NTY3ODkw...oai_key',    4),
    ('20000000-0000-0000-0000-000000000007', 'MQTT_BROKER_URL',      'enc:v1:AAAADQxMjM0NTY3ODkw...mqtt',       5),
    ('20000000-0000-0000-0000-000000000007', 'INFLUXDB_TOKEN',       'enc:v1:AAAADQxMjM0NTY3ODkw...influx',     5),
    ('20000000-0000-0000-0000-000000000011', 'STRIPE_SECRET_KEY',    'enc:v1:AAAADQxMjM0NTY3ODkw...mega_sk',    9),
    ('20000000-0000-0000-0000-000000000011', 'ALGOLIA_API_KEY',      'enc:v1:AAAADQxMjM0NTY3ODkw...algolia',    9);

-- ─── Deployments ────────────────────────────────────────────────────

CREATE TABLE deployments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    commit_sha      VARCHAR(40) NOT NULL,
    commit_message  TEXT,
    branch          VARCHAR(100) NOT NULL,
    status          VARCHAR(30) NOT NULL DEFAULT 'queued',
    url             TEXT,
    build_duration_ms INT,
    triggered_by    BIGINT NOT NULL REFERENCES users(id),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_deployments_env ON deployments(environment_id);
CREATE INDEX idx_deployments_status ON deployments(status);
CREATE INDEX idx_deployments_created ON deployments(created_at DESC);

INSERT INTO deployments (id, environment_id, commit_sha, commit_message, branch, status, url, build_duration_ms, triggered_by, started_at, finished_at, created_at) VALUES
    ('30000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', 'a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2', 'feat: add user dashboard',          'main',    'ready',    'https://app.acmecorp.com',              45200,  1, '2026-03-24 08:00:00+00', '2026-03-24 08:00:45+00', '2026-03-24 08:00:00+00'),
    ('30000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000002', 'b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3', 'fix: resolve auth token refresh',   'develop', 'ready',    'https://staging.app.acmecorp.com',      38100,  2, '2026-03-24 09:00:00+00', '2026-03-24 09:00:38+00', '2026-03-24 09:00:00+00'),
    ('30000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000004', 'c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4', 'perf: optimize query batching',     'main',    'ready',    'https://api.acmecorp.com',              12300,  2, '2026-03-24 09:15:00+00', '2026-03-24 09:15:12+00', '2026-03-24 09:15:00+00'),
    ('30000000-0000-0000-0000-000000000004', '20000000-0000-0000-0000-000000000006', 'd4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5', 'feat: add real-time notifications',  'main',    'ready',    'https://tanaka-saas.com',               52400,  4, '2026-03-24 07:30:00+00', '2026-03-24 07:30:52+00', '2026-03-24 07:30:00+00'),
    ('30000000-0000-0000-0000-000000000005', '20000000-0000-0000-0000-000000000007', 'e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6', 'feat: MQTT dashboard widgets',      'main',    'ready',    'https://iot.berlinlabs.de',             41000,  5, '2026-03-24 11:00:00+00', '2026-03-24 11:00:41+00', '2026-03-24 11:00:00+00'),
    ('30000000-0000-0000-0000-000000000006', '20000000-0000-0000-0000-000000000008', 'f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1', 'fix: k8s health check endpoint',    'main',    'ready',    'https://platform.cloudnative.in',       33500,  6, '2026-03-24 06:00:00+00', '2026-03-24 06:00:33+00', '2026-03-24 06:00:00+00'),
    ('30000000-0000-0000-0000-000000000007', '20000000-0000-0000-0000-000000000009', 'a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1c4', 'chore: update dependencies',        'main',    'ready',    'https://app.startup.io',                28000,  7, '2026-03-23 22:00:00+00', '2026-03-23 22:00:28+00', '2026-03-23 22:00:00+00'),
    ('30000000-0000-0000-0000-000000000008', '20000000-0000-0000-0000-000000000011', 'b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2d5', 'feat: implement shopping cart',     'main',    'building', 'https://shop.megascale.cn',             NULL,   9, '2026-03-24 07:40:00+00', NULL,                      '2026-03-24 07:40:00+00'),
    ('30000000-0000-0000-0000-000000000009', '20000000-0000-0000-0000-000000000012', 'c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3e6', 'test: add e2e checkout tests',      'develop', 'failed',   NULL,                                    15200,  9, '2026-03-24 07:45:00+00', '2026-03-24 07:45:15+00', '2026-03-24 07:45:00+00'),
    ('30000000-0000-0000-0000-000000000010', '20000000-0000-0000-0000-000000000001', 'd4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4f7', 'feat: add billing page',            'main',    'queued',   NULL,                                    NULL,   1, NULL,                      NULL,                      '2026-03-24 10:05:00+00');

-- ─── Deployment Logs ────────────────────────────────────────────────

CREATE TABLE deployment_logs (
    id              BIGINT PRIMARY KEY DEFAULT unique_rowid(),
    deployment_id   UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    level           VARCHAR(10) NOT NULL DEFAULT 'info',
    message         TEXT NOT NULL,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_deployment_logs_deployment ON deployment_logs(deployment_id);

INSERT INTO deployment_logs (deployment_id, level, message, timestamp) VALUES
    ('30000000-0000-0000-0000-000000000001', 'info',  'Build started',                            '2026-03-24 08:00:00+00'),
    ('30000000-0000-0000-0000-000000000001', 'info',  'Installing dependencies (npm ci)...',      '2026-03-24 08:00:05+00'),
    ('30000000-0000-0000-0000-000000000001', 'info',  'Dependencies installed in 12.3s',          '2026-03-24 08:00:17+00'),
    ('30000000-0000-0000-0000-000000000001', 'info',  'Running build command: npm run build',     '2026-03-24 08:00:18+00'),
    ('30000000-0000-0000-0000-000000000001', 'info',  'Build completed in 25.1s',                 '2026-03-24 08:00:43+00'),
    ('30000000-0000-0000-0000-000000000001', 'info',  'Deploying to edge network...',             '2026-03-24 08:00:43+00'),
    ('30000000-0000-0000-0000-000000000001', 'info',  'Deployment ready at https://app.acmecorp.com', '2026-03-24 08:00:45+00'),
    ('30000000-0000-0000-0000-000000000009', 'info',  'Build started',                            '2026-03-24 07:45:00+00'),
    ('30000000-0000-0000-0000-000000000009', 'info',  'Installing dependencies...',               '2026-03-24 07:45:04+00'),
    ('30000000-0000-0000-0000-000000000009', 'info',  'Running build command: npm run build',     '2026-03-24 07:45:08+00'),
    ('30000000-0000-0000-0000-000000000009', 'error', 'Test suite failed: 3 tests failed',        '2026-03-24 07:45:14+00'),
    ('30000000-0000-0000-0000-000000000009', 'error', 'Build failed with exit code 1',            '2026-03-24 07:45:15+00');

-- ─── Custom Domains ─────────────────────────────────────────────────

CREATE TABLE custom_domains (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    domain          VARCHAR(255) UNIQUE NOT NULL,
    ssl_status      VARCHAR(30) NOT NULL DEFAULT 'pending',
    ssl_expires_at  TIMESTAMPTZ,
    dns_configured  BOOLEAN NOT NULL DEFAULT false,
    verified_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO custom_domains (environment_id, domain, ssl_status, ssl_expires_at, dns_configured, verified_at) VALUES
    ('20000000-0000-0000-0000-000000000001', 'app.acmecorp.com',          'active',  '2026-09-24', true,  '2026-01-15 10:00:00+00'),
    ('20000000-0000-0000-0000-000000000001', 'www.acmecorp.com',          'active',  '2026-09-24', true,  '2026-01-15 10:05:00+00'),
    ('20000000-0000-0000-0000-000000000004', 'api.acmecorp.com',          'active',  '2026-09-24', true,  '2026-01-15 10:10:00+00'),
    ('20000000-0000-0000-0000-000000000006', 'tanaka-saas.com',           'active',  '2026-08-15', true,  '2026-02-01 08:00:00+00'),
    ('20000000-0000-0000-0000-000000000006', 'www.tanaka-saas.com',       'active',  '2026-08-15', true,  '2026-02-01 08:05:00+00'),
    ('20000000-0000-0000-0000-000000000007', 'iot.berlinlabs.de',         'active',  '2026-10-01', true,  '2026-02-20 14:00:00+00'),
    ('20000000-0000-0000-0000-000000000008', 'platform.cloudnative.in',   'active',  '2026-07-30', true,  '2026-01-30 06:00:00+00'),
    ('20000000-0000-0000-0000-000000000009', 'app.startup.io',            'active',  '2026-11-01', true,  '2026-03-01 12:00:00+00'),
    ('20000000-0000-0000-0000-000000000010', 'lunapark.dev',              'active',  '2026-12-01', true,  '2026-03-10 20:00:00+00'),
    ('20000000-0000-0000-0000-000000000011', 'shop.megascale.cn',         'active',  '2026-06-15', true,  '2026-01-01 07:00:00+00'),
    ('20000000-0000-0000-0000-000000000011', 'megascale.com',             'pending', NULL,          false, NULL);
