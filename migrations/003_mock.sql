begin;

-- 1️⃣ Ensure supporting enumerations exist (in case migration 001 not yet applied)
insert into roles (value)
    values ('PASSENGER'), ('DRIVER'), ('ADMIN')
on conflict do nothing;

insert into user_status (value)
    values ('ACTIVE'), ('INACTIVE'), ('BANNED')
on conflict do nothing;

insert into vehicle_type (value)
    values ('ECONOMY'), ('PREMIUM'), ('XL')
on conflict do nothing;

insert into driver_status (value)
    values ('OFFLINE'), ('AVAILABLE'), ('BUSY'), ('EN_ROUTE')
on conflict do nothing;

-- 2️⃣ Create a mock driver user (also works for ride & location testing)

-- If user already exists, skip insertion.
insert into users (id, email, role, status, password_hash)
values (
    '660e8400-e29b-41d4-a716-446655440001',
    'mockdriver@example.com',
    'DRIVER',
    'ACTIVE',
    'mock-hash'
)
on conflict (email) do nothing;

-- 3️⃣ Create driver record linked to user
insert into drivers (
    id,
    license_number,
    vehicle_type,
    status,
    is_verified,
    vehicle_attrs
)
values (
    '660e8400-e29b-41d4-a716-446655440001',
    'MOCK-DR-001',
    'ECONOMY',
    'OFFLINE',
    true,
    '{
        "vehicle_make": "Toyota",
        "vehicle_model": "Camry",
        "vehicle_color": "White",
        "vehicle_plate": "KZ 123 ABC",
        "vehicle_year": 2020
    }'::jsonb
)
on conflict (id) do nothing;

-- 4️⃣ Optional: insert starting coordinate for driver
insert into coordinates (
    id,
    entity_id,
    entity_type,
    address,
    latitude,
    longitude,
    is_current
)
values (
    gen_random_uuid(),
    '660e8400-e29b-41d4-a716-446655440001',
    'driver',
    'Almaty Central Park',
    43.238949,
    76.889709,
    true
);

commit;
