INSERT INTO users (id, username, password_hash, role, line_id)
VALUES
    ('user-sales', 'sales', 'demo', 'sales', NULL),
    ('user-scheduler-a', 'scheduler-a', 'demo', 'scheduler', 'A'),
    ('user-scheduler-b', 'scheduler-b', 'demo', 'scheduler', 'B'),
    ('user-scheduler-c', 'scheduler-c', 'demo', 'scheduler', 'C'),
    ('user-scheduler-d', 'scheduler-d', 'demo', 'scheduler', 'D')
ON CONFLICT (id) DO NOTHING;

INSERT INTO orders (
    id,
    customer,
    line_id,
    quantity,
    priority,
    status,
    due_date,
    created_by,
    created_at,
    updated_at
)
VALUES
    ('ORD-DEMO-1', 'TSMC Demo', 'A', 2500, 'high', '待排程', '2026-05-03', 'user-sales', NOW(), NOW()),
    ('ORD-DEMO-2', 'ACME Silicon', 'A', 1800, 'low', '已排程', '2026-05-04', 'user-sales', NOW(), NOW()),
    ('ORD-DEMO-3', 'Northstar Fabless', 'B', 2200, 'low', '待排程', '2026-05-05', 'user-sales', NOW(), NOW()),
    ('ORD-DEMO-4', 'Orion Devices', 'C', 1250, 'high', '生產中', '2026-05-02', 'user-sales', NOW(), NOW()),
    ('ORD-DEMO-5', 'Helio Sensors', 'D', 900, 'low', '待排程', '2026-05-06', 'user-sales', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;
