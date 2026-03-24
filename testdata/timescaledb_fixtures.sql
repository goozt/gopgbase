-- TimescaleDB fixtures: Metrics & observability data (multi-region ready).
-- Purpose: Time-series data for server metrics, application metrics,
--          HTTP request logs, deployment events, uptime checks, and alerts.
-- Designed for Prometheus remote-write and Grafana dashboards.

DROP TABLE IF EXISTS alert_rules CASCADE;
DROP TABLE IF EXISTS alert_events CASCADE;
DROP TABLE IF EXISTS uptime_checks CASCADE;
DROP TABLE IF EXISTS deployment_events CASCADE;
DROP TABLE IF EXISTS http_request_logs CASCADE;
DROP TABLE IF EXISTS app_metrics CASCADE;
DROP TABLE IF EXISTS server_metrics CASCADE;
DROP TABLE IF EXISTS metric_labels CASCADE;
DROP TABLE IF EXISTS monitored_services CASCADE;

-- ─── Monitored Services ─────────────────────────────────────────────

CREATE TABLE monitored_services (
    id              SERIAL PRIMARY KEY,
    org_id          UUID NOT NULL,
    service_name    VARCHAR(255) NOT NULL,
    service_type    VARCHAR(50) NOT NULL,
    region          VARCHAR(30) NOT NULL,
    endpoint        TEXT,
    active          BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, service_name, region)
);

INSERT INTO monitored_services (id, org_id, service_name, service_type, region, endpoint) VALUES
    (1,  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'web-app',        'frontend',  'us-east-1',      'https://app.acmecorp.com'),
    (2,  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'api-gateway',    'backend',   'us-east-1',      'https://api.acmecorp.com'),
    (3,  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'api-gateway',    'backend',   'eu-west-1',      'https://eu.api.acmecorp.com'),
    (4,  'b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 'tanaka-saas',    'frontend',  'ap-northeast-1', 'https://tanaka-saas.com'),
    (5,  'b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 'tanaka-api',     'backend',   'ap-northeast-1', 'https://api.tanaka-saas.com'),
    (6,  'c2aade11-be2d-6a1a-dd8f-8ddbdf5a2c33', 'iot-dashboard',  'frontend',  'eu-central-1',   'https://iot.berlinlabs.de'),
    (7,  'd3bbef22-cf3e-7b2b-eea0-9eece06b3d44', 'cn-platform',    'backend',   'ap-south-1',     'https://platform.cloudnative.in'),
    (8,  '06eeb255-f261-ae5e-11d3-c11f339e6077', 'mega-commerce',  'frontend',  'ap-east-1',      'https://shop.megascale.cn'),
    (9,  '06eeb255-f261-ae5e-11d3-c11f339e6077', 'mega-commerce',  'frontend',  'us-west-2',      'https://us.shop.megascale.cn'),
    (10, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'postgres-primary','database', 'us-east-1',      NULL),
    (11, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'redis-cache',    'cache',     'us-east-1',      NULL),
    (12, '06eeb255-f261-ae5e-11d3-c11f339e6077', 'postgres-primary','database', 'ap-east-1',      NULL);

-- ─── Server Metrics (hypertable) ────────────────────────────────────

CREATE TABLE server_metrics (
    time            TIMESTAMPTZ NOT NULL,
    service_id      INT NOT NULL REFERENCES monitored_services(id),
    host            VARCHAR(100) NOT NULL,
    region          VARCHAR(30) NOT NULL,
    cpu_percent     DOUBLE PRECISION,
    memory_percent  DOUBLE PRECISION,
    memory_bytes    BIGINT,
    disk_percent    DOUBLE PRECISION,
    disk_read_bps   BIGINT,
    disk_write_bps  BIGINT,
    net_rx_bps      BIGINT,
    net_tx_bps      BIGINT,
    load_avg_1m     DOUBLE PRECISION,
    load_avg_5m     DOUBLE PRECISION,
    open_connections INT,
    goroutines      INT
);

SELECT create_hypertable('server_metrics', 'time');

-- Generate server metrics every 30s for the past 2 hours across multiple hosts
INSERT INTO server_metrics (time, service_id, host, region, cpu_percent, memory_percent, memory_bytes, disk_percent, disk_read_bps, disk_write_bps, net_rx_bps, net_tx_bps, load_avg_1m, load_avg_5m, open_connections, goroutines)
SELECT
    ts,
    s.id,
    s.service_name || '-' || s.region || '-' || n.idx,
    s.region,
    20 + 30 * random() + CASE WHEN EXTRACT(minute FROM ts) BETWEEN 30 AND 45 THEN 25 * random() ELSE 0 END,
    40 + 20 * random(),
    (2147483648 + (random() * 2147483648)::BIGINT),
    30 + 10 * random(),
    (50000 + random() * 200000)::BIGINT,
    (30000 + random() * 150000)::BIGINT,
    (100000 + random() * 500000)::BIGINT,
    (80000 + random() * 400000)::BIGINT,
    0.5 + 2.0 * random(),
    0.8 + 1.5 * random(),
    (10 + random() * 90)::INT,
    (50 + random() * 450)::INT
FROM generate_series(
    NOW() - INTERVAL '2 hours',
    NOW(),
    INTERVAL '30 seconds'
) AS ts
CROSS JOIN monitored_services s
CROSS JOIN (SELECT generate_series(1, 2) AS idx) n
WHERE s.service_type IN ('backend', 'frontend');

-- ─── Application Metrics (hypertable) ───────────────────────────────

CREATE TABLE app_metrics (
    time                TIMESTAMPTZ NOT NULL,
    service_id          INT NOT NULL REFERENCES monitored_services(id),
    region              VARCHAR(30) NOT NULL,
    requests_per_sec    DOUBLE PRECISION,
    errors_per_sec      DOUBLE PRECISION,
    p50_latency_ms      DOUBLE PRECISION,
    p95_latency_ms      DOUBLE PRECISION,
    p99_latency_ms      DOUBLE PRECISION,
    active_users        INT,
    active_websockets   INT,
    cache_hit_rate      DOUBLE PRECISION,
    db_pool_active      INT,
    db_pool_idle        INT,
    queue_depth         INT
);

SELECT create_hypertable('app_metrics', 'time');

INSERT INTO app_metrics (time, service_id, region, requests_per_sec, errors_per_sec, p50_latency_ms, p95_latency_ms, p99_latency_ms, active_users, active_websockets, cache_hit_rate, db_pool_active, db_pool_idle, queue_depth)
SELECT
    ts,
    s.id,
    s.region,
    100 + 400 * random() + CASE WHEN EXTRACT(hour FROM ts) BETWEEN 9 AND 17 THEN 200 * random() ELSE 0 END,
    0.5 + 3 * random(),
    5 + 15 * random(),
    20 + 80 * random(),
    50 + 200 * random(),
    (50 + random() * 500)::INT,
    (10 + random() * 100)::INT,
    0.80 + 0.19 * random(),
    (5 + random() * 20)::INT,
    (3 + random() * 10)::INT,
    (random() * 50)::INT
FROM generate_series(
    NOW() - INTERVAL '2 hours',
    NOW(),
    INTERVAL '15 seconds'
) AS ts
CROSS JOIN monitored_services s
WHERE s.service_type IN ('backend', 'frontend');

-- ─── HTTP Request Logs (hypertable) ─────────────────────────────────

CREATE TABLE http_request_logs (
    time            TIMESTAMPTZ NOT NULL,
    service_id      INT NOT NULL REFERENCES monitored_services(id),
    region          VARCHAR(30) NOT NULL,
    method          VARCHAR(10) NOT NULL,
    path            VARCHAR(500) NOT NULL,
    status_code     INT NOT NULL,
    latency_ms      DOUBLE PRECISION NOT NULL,
    request_bytes   INT,
    response_bytes  INT,
    user_agent      TEXT,
    ip_address      VARCHAR(45),
    trace_id        VARCHAR(32),
    span_id         VARCHAR(16)
);

SELECT create_hypertable('http_request_logs', 'time');

-- Sample HTTP request logs
INSERT INTO http_request_logs (time, service_id, region, method, path, status_code, latency_ms, request_bytes, response_bytes, trace_id, span_id)
SELECT
    ts,
    s.id,
    s.region,
    (ARRAY['GET','GET','GET','POST','PUT','DELETE','PATCH'])[1 + (random() * 6)::INT],
    (ARRAY[
        '/api/v1/users', '/api/v1/projects', '/api/v1/deployments',
        '/api/v1/health', '/api/v1/metrics', '/api/v1/auth/login',
        '/api/v1/auth/refresh', '/api/v1/organizations', '/api/v1/billing',
        '/api/v1/webhooks', '/graphql', '/api/v1/search'
    ])[1 + (random() * 11)::INT],
    (ARRAY[200, 200, 200, 200, 200, 201, 204, 301, 400, 401, 403, 404, 500, 502, 503])[1 + (random() * 14)::INT],
    2 + 500 * random() * random(),
    (100 + random() * 5000)::INT,
    (200 + random() * 50000)::INT,
    md5(random()::TEXT),
    substring(md5(random()::TEXT) FROM 1 FOR 16)
FROM generate_series(
    NOW() - INTERVAL '1 hour',
    NOW(),
    INTERVAL '200 milliseconds'
) AS ts
CROSS JOIN monitored_services s
WHERE s.service_type = 'backend';

-- ─── Deployment Events (hypertable) ─────────────────────────────────

CREATE TABLE deployment_events (
    time            TIMESTAMPTZ NOT NULL,
    service_id      INT NOT NULL REFERENCES monitored_services(id),
    org_id          UUID NOT NULL,
    deployment_id   VARCHAR(50) NOT NULL,
    event_type      VARCHAR(30) NOT NULL,
    environment     VARCHAR(30) NOT NULL,
    version         VARCHAR(50),
    commit_sha      VARCHAR(40),
    duration_ms     INT,
    success         BOOLEAN,
    metadata        JSONB NOT NULL DEFAULT '{}'
);

SELECT create_hypertable('deployment_events', 'time');

INSERT INTO deployment_events (time, service_id, org_id, deployment_id, event_type, environment, version, commit_sha, duration_ms, success, metadata) VALUES
    ('2026-03-24 08:00:00+00', 1, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'dpl_001', 'build_start',  'production', 'v2.14.3', 'a1b2c3d4', NULL,  NULL,  '{"trigger": "push", "branch": "main"}'),
    ('2026-03-24 08:00:45+00', 1, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'dpl_001', 'build_end',    'production', 'v2.14.3', 'a1b2c3d4', 45200, true,  '{"size_bytes": 15234567}'),
    ('2026-03-24 08:00:46+00', 1, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'dpl_001', 'deploy_start', 'production', 'v2.14.3', 'a1b2c3d4', NULL,  NULL,  '{"strategy": "rolling", "replicas": 3}'),
    ('2026-03-24 08:01:10+00', 1, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'dpl_001', 'deploy_end',   'production', 'v2.14.3', 'a1b2c3d4', 24000, true,  '{"healthy_replicas": 3}'),
    ('2026-03-24 09:15:00+00', 2, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'dpl_002', 'build_start',  'production', 'v3.8.1',  'c3d4e5f6', NULL,  NULL,  '{"trigger": "push", "branch": "main"}'),
    ('2026-03-24 09:15:12+00', 2, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'dpl_002', 'build_end',    'production', 'v3.8.1',  'c3d4e5f6', 12300, true,  '{"size_bytes": 8234567}'),
    ('2026-03-24 09:15:13+00', 2, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'dpl_002', 'deploy_start', 'production', 'v3.8.1',  'c3d4e5f6', NULL,  NULL,  '{"strategy": "blue_green"}'),
    ('2026-03-24 09:15:30+00', 2, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'dpl_002', 'deploy_end',   'production', 'v3.8.1',  'c3d4e5f6', 17000, true,  '{"healthy_replicas": 2}'),
    ('2026-03-24 07:30:00+00', 4, 'b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 'dpl_003', 'build_start',  'production', 'v1.22.0', 'd4e5f6a1', NULL,  NULL,  '{"trigger": "push"}'),
    ('2026-03-24 07:30:52+00', 4, 'b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 'dpl_003', 'build_end',    'production', 'v1.22.0', 'd4e5f6a1', 52400, true,  '{}'),
    ('2026-03-24 07:31:00+00', 4, 'b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 'dpl_003', 'deploy_end',   'production', 'v1.22.0', 'd4e5f6a1', 8000,  true,  '{}'),
    ('2026-03-24 07:45:00+00', 8, '06eeb255-f261-ae5e-11d3-c11f339e6077', 'dpl_004', 'build_start',  'staging',    'v5.0.0-rc1','b2c3d4e5',NULL,  NULL,  '{"trigger": "push", "branch": "develop"}'),
    ('2026-03-24 07:45:15+00', 8, '06eeb255-f261-ae5e-11d3-c11f339e6077', 'dpl_004', 'build_end',    'staging',    'v5.0.0-rc1','b2c3d4e5',15200, false, '{"error": "test_failure", "failed_tests": 3}'),
    ('2026-03-24 11:00:00+00', 6, 'c2aade11-be2d-6a1a-dd8f-8ddbdf5a2c33', 'dpl_005', 'build_start',  'production', 'v0.9.4',  'e5f6a1b2', NULL,  NULL,  '{"trigger": "manual"}'),
    ('2026-03-24 11:00:41+00', 6, 'c2aade11-be2d-6a1a-dd8f-8ddbdf5a2c33', 'dpl_005', 'build_end',    'production', 'v0.9.4',  'e5f6a1b2', 41000, true,  '{}'),
    ('2026-03-24 11:00:55+00', 6, 'c2aade11-be2d-6a1a-dd8f-8ddbdf5a2c33', 'dpl_005', 'deploy_end',   'production', 'v0.9.4',  'e5f6a1b2', 14000, true,  '{}');

-- ─── Uptime Checks (hypertable) ─────────────────────────────────────

CREATE TABLE uptime_checks (
    time            TIMESTAMPTZ NOT NULL,
    service_id      INT NOT NULL REFERENCES monitored_services(id),
    region          VARCHAR(30) NOT NULL,
    check_type      VARCHAR(20) NOT NULL DEFAULT 'http',
    status_code     INT,
    latency_ms      DOUBLE PRECISION NOT NULL,
    healthy         BOOLEAN NOT NULL,
    error_message   TEXT
);

SELECT create_hypertable('uptime_checks', 'time');

INSERT INTO uptime_checks (time, service_id, region, check_type, status_code, latency_ms, healthy, error_message)
SELECT
    ts,
    s.id,
    probe_region,
    'http',
    CASE WHEN random() > 0.02 THEN 200 ELSE (ARRAY[500, 502, 503, 0])[1 + (random() * 3)::INT] END,
    10 + 200 * random() * random(),
    random() > 0.02,
    CASE WHEN random() > 0.02 THEN NULL ELSE 'connection timeout after 10s' END
FROM generate_series(
    NOW() - INTERVAL '2 hours',
    NOW(),
    INTERVAL '60 seconds'
) AS ts
CROSS JOIN monitored_services s
CROSS JOIN (VALUES ('us-east-1'), ('eu-west-1'), ('ap-northeast-1')) AS probes(probe_region)
WHERE s.service_type IN ('backend', 'frontend') AND s.endpoint IS NOT NULL;

-- ─── Alert Rules ────────────────────────────────────────────────────

CREATE TABLE alert_rules (
    id              SERIAL PRIMARY KEY,
    org_id          UUID NOT NULL,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    metric          VARCHAR(100) NOT NULL,
    condition       VARCHAR(10) NOT NULL,
    threshold       DOUBLE PRECISION NOT NULL,
    duration        INTERVAL NOT NULL DEFAULT '5 minutes',
    severity        VARCHAR(20) NOT NULL DEFAULT 'warning',
    notify_channels TEXT[] NOT NULL DEFAULT '{}',
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO alert_rules (id, org_id, name, description, metric, condition, threshold, duration, severity, notify_channels) VALUES
    (1, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'High CPU Usage',         'CPU exceeds 80% for 5 minutes',         'cpu_percent',       '>',  80,    '5 minutes',  'critical', '{slack,pagerduty}'),
    (2, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'High Error Rate',        'Error rate exceeds 5/sec',              'errors_per_sec',    '>',  5,     '2 minutes',  'critical', '{slack,pagerduty,email}'),
    (3, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'High P99 Latency',       'P99 latency exceeds 500ms',             'p99_latency_ms',    '>',  500,   '5 minutes',  'warning',  '{slack}'),
    (4, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Low Cache Hit Rate',     'Cache hit rate below 70%',              'cache_hit_rate',    '<',  0.70,  '10 minutes', 'warning',  '{slack}'),
    (5, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Disk Usage Critical',    'Disk usage exceeds 90%',                'disk_percent',      '>',  90,    '1 minute',   'critical', '{pagerduty}'),
    (6, 'b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 'High Memory Usage',      'Memory exceeds 85%',                    'memory_percent',    '>',  85,    '5 minutes',  'warning',  '{slack}'),
    (7, 'b1ffcd00-ad1c-5f09-cc7e-7ccace491b22', 'Deployment Failure',     'Deployment build failed',               'deploy_success',    '=',  0,     '0 seconds',  'critical', '{slack,email}'),
    (8, '06eeb255-f261-ae5e-11d3-c11f339e6077', 'High Request Volume',    'Requests exceed 1000/sec',              'requests_per_sec',  '>',  1000,  '3 minutes',  'info',     '{slack}'),
    (9, '06eeb255-f261-ae5e-11d3-c11f339e6077', 'Uptime Check Failed',    'Service unreachable from any region',   'healthy',           '=',  0,     '1 minute',   'critical', '{pagerduty,slack,email}'),
    (10,'c2aade11-be2d-6a1a-dd8f-8ddbdf5a2c33', 'High Queue Depth',       'Queue depth exceeds 100',               'queue_depth',       '>',  100,   '5 minutes',  'warning',  '{slack}');

-- ─── Alert Events (hypertable) ──────────────────────────────────────

CREATE TABLE alert_events (
    time            TIMESTAMPTZ NOT NULL,
    alert_rule_id   INT NOT NULL REFERENCES alert_rules(id),
    service_id      INT NOT NULL REFERENCES monitored_services(id),
    status          VARCHAR(20) NOT NULL,
    value           DOUBLE PRECISION NOT NULL,
    message         TEXT,
    notified        BOOLEAN NOT NULL DEFAULT false
);

SELECT create_hypertable('alert_events', 'time');

INSERT INTO alert_events (time, alert_rule_id, service_id, status, value, message, notified) VALUES
    ('2026-03-24 08:32:00+00', 1, 2, 'firing',   87.5,  'CPU at 87.5% on api-gateway-us-east-1-1', true),
    ('2026-03-24 08:37:00+00', 1, 2, 'resolved', 62.3,  'CPU recovered to 62.3%',                  true),
    ('2026-03-24 09:15:00+00', 3, 2, 'firing',   523.4, 'P99 latency at 523ms during deploy',      true),
    ('2026-03-24 09:20:00+00', 3, 2, 'resolved', 145.2, 'P99 latency recovered after deploy',      true),
    ('2026-03-24 07:45:15+00', 7, 4, 'firing',   0,     'Deployment dpl_004 build failed',          true),
    ('2026-03-24 06:10:00+00', 2, 7, 'firing',   7.2,   'Error rate spike on cn-platform',          true),
    ('2026-03-24 06:12:00+00', 2, 7, 'resolved', 1.1,   'Error rate normalized',                    true),
    ('2026-03-24 10:00:00+00', 8, 8, 'firing',   1250,  'Request volume spike on mega-commerce',    true),
    ('2026-03-24 10:15:00+00', 8, 8, 'resolved', 780,   'Request volume returned to normal',        true),
    ('2026-03-24 04:00:00+00', 5, 10,'firing',   91.2,  'Disk usage critical on postgres-primary',  true),
    ('2026-03-24 04:30:00+00', 5, 10,'resolved', 65.0,  'Disk usage resolved after vacuum',         true),
    ('2026-03-24 09:45:00+00', 4, 11,'firing',   0.65,  'Cache hit rate dropped to 65%',            true);

-- ─── Metric Labels (for Prometheus-compatible label indexing) ────────

CREATE TABLE metric_labels (
    id              SERIAL PRIMARY KEY,
    metric_name     VARCHAR(100) NOT NULL,
    label_key       VARCHAR(100) NOT NULL,
    label_value     VARCHAR(255) NOT NULL,
    service_id      INT NOT NULL REFERENCES monitored_services(id),
    UNIQUE(metric_name, label_key, label_value, service_id)
);

INSERT INTO metric_labels (metric_name, label_key, label_value, service_id) VALUES
    ('http_requests_total',         'method',   'GET',              2),
    ('http_requests_total',         'method',   'POST',             2),
    ('http_requests_total',         'method',   'PUT',              2),
    ('http_requests_total',         'method',   'DELETE',           2),
    ('http_requests_total',         'handler',  '/api/v1/users',    2),
    ('http_requests_total',         'handler',  '/api/v1/projects', 2),
    ('http_request_duration_seconds','quantile', '0.5',             2),
    ('http_request_duration_seconds','quantile', '0.95',            2),
    ('http_request_duration_seconds','quantile', '0.99',            2),
    ('go_goroutines',               'instance', 'api-gateway-1',   2),
    ('go_goroutines',               'instance', 'api-gateway-2',   2),
    ('process_cpu_seconds_total',   'instance', 'api-gateway-1',   2),
    ('pg_stat_activity_count',      'state',    'active',          10),
    ('pg_stat_activity_count',      'state',    'idle',            10),
    ('pg_database_size_bytes',      'datname',  'acme_production', 10),
    ('node_cpu_seconds_total',      'mode',     'user',             1),
    ('node_cpu_seconds_total',      'mode',     'system',           1),
    ('node_memory_MemAvailable_bytes','instance','web-app-1',       1),
    ('container_cpu_usage_seconds_total','container','api-gateway', 2),
    ('container_memory_working_set_bytes','container','api-gateway',2);

-- ─── Compression & Retention Policies ───────────────────────────────

ALTER TABLE server_metrics SET (timescaledb.compress, timescaledb.compress_segmentby = 'service_id, host');
ALTER TABLE app_metrics SET (timescaledb.compress, timescaledb.compress_segmentby = 'service_id');
ALTER TABLE http_request_logs SET (timescaledb.compress, timescaledb.compress_segmentby = 'service_id');
ALTER TABLE uptime_checks SET (timescaledb.compress, timescaledb.compress_segmentby = 'service_id');
ALTER TABLE alert_events SET (timescaledb.compress, timescaledb.compress_segmentby = 'alert_rule_id');
ALTER TABLE deployment_events SET (timescaledb.compress, timescaledb.compress_segmentby = 'service_id');

SELECT add_compression_policy('server_metrics',    INTERVAL '7 days');
SELECT add_compression_policy('app_metrics',       INTERVAL '7 days');
SELECT add_compression_policy('http_request_logs', INTERVAL '3 days');
SELECT add_compression_policy('uptime_checks',     INTERVAL '7 days');
SELECT add_compression_policy('alert_events',      INTERVAL '14 days');
SELECT add_compression_policy('deployment_events', INTERVAL '30 days');

SELECT add_retention_policy('server_metrics',    INTERVAL '90 days');
SELECT add_retention_policy('app_metrics',       INTERVAL '90 days');
SELECT add_retention_policy('http_request_logs', INTERVAL '30 days');
SELECT add_retention_policy('uptime_checks',     INTERVAL '180 days');
SELECT add_retention_policy('alert_events',      INTERVAL '365 days');
SELECT add_retention_policy('deployment_events', INTERVAL '365 days');

-- ─── Sequence sync for manual IDs ─────────────────────────────────────────

SELECT setval('monitored_services_id_seq', COALESCE((SELECT MAX(id) FROM monitored_services), 0) + 1, false);
SELECT setval('alert_rules_id_seq',        COALESCE((SELECT MAX(id) FROM alert_rules), 0) + 1, false);
SELECT setval('metric_labels_id_seq',      COALESCE((SELECT MAX(id) FROM metric_labels), 0) + 1, false);
