-- Test fixtures for gopgbase integration tests.

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    age INT,
    active BOOLEAN DEFAULT true,
    metadata JSONB,
    tags TEXT[],
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO users (name, email, age, active, metadata) VALUES
    ('Alice', 'alice@example.com', 30, true, '{"role": "admin"}'),
    ('Bob', 'bob@example.com', 25, true, '{"role": "user"}'),
    ('Charlie', 'charlie@example.com', 35, false, '{"role": "user"}')
ON CONFLICT (email) DO NOTHING;

INSERT INTO orders (user_id, amount, status) VALUES
    (1, 100.00, 'completed'),
    (1, 50.50, 'pending'),
    (2, 75.25, 'completed')
ON CONFLICT DO NOTHING;
