CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'sales', 'scheduler')),
    line_id TEXT
);

CREATE TABLE IF NOT EXISTS production_lines (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    capacity_per_day INTEGER NOT NULL CHECK (capacity_per_day > 0)
);

CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    customer TEXT NOT NULL,
    line_id TEXT NOT NULL REFERENCES production_lines(id),
    quantity INTEGER NOT NULL CHECK (quantity BETWEEN 25 AND 2500),
    priority TEXT NOT NULL CHECK (priority IN ('low', 'high')),
    status TEXT NOT NULL CHECK (status IN ('待排程', '已排程', '生產中', '已完成')),
    due_date DATE NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id),
    source_order TEXT REFERENCES orders(id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS schedule_jobs (
    id TEXT PRIMARY KEY,
    line_id TEXT NOT NULL REFERENCES production_lines(id),
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    message TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS schedule_allocations (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id),
    line_id TEXT NOT NULL REFERENCES production_lines(id),
    allocation_date DATE NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    priority TEXT NOT NULL CHECK (priority IN ('low', 'high')),
    locked BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT CHECK (status IN ('敺?蝔?, '撌脫?蝔?, '?銝?, '撌脣???))
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    actor_id TEXT NOT NULL REFERENCES users(id),
    action TEXT NOT NULL,
    resource TEXT NOT NULL,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL
);

INSERT INTO production_lines (id, name, capacity_per_day)
VALUES
    ('A', 'Line A', 10000),
    ('B', 'Line B', 10000),
    ('C', 'Line C', 10000),
    ('D', 'Line D', 10000)
ON CONFLICT (id) DO NOTHING;
