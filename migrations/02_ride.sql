begin;

-- User roles enumeration
create table "roles"("value" text not null primary key);
insert into
    "roles" ("value")
values
    ('PASSENGER'),     -- Passenger/Customer
    ('DRIVER'),    -- Driver
    ('ADMIN')      -- Administrator
;

-- User status enumeration
create table "user_status"("value" text not null primary key);
insert into
    "user_status" ("value")
values
    ('ACTIVE'),    -- Active user
    ('INACTIVE'),  -- Inactive/Suspended
    ('BANNED')     -- Banned user
;

-- Main users table
create table users (
    id uuid primary key default gen_random_uuid(),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    email varchar(100) unique not null,
    role text references "roles"(value) not null,
    status text references "user_status"(value) not null default 'ACTIVE',
    password_hash text not null,
    attrs jsonb default '{}'::jsonb
);

-- Ride status enumeration
create table "ride_status"("value" text not null primary key);
insert into
    "ride_status" ("value")
values
    ('REQUESTED'),   -- Ride has been requested by customer
    ('MATCHED'),     -- Driver has been matched to the ride
    ('EN_ROUTE'),    -- Driver is on the way to pickup location
    ('ARRIVED'),     -- Driver has arrived at pickup location
    ('IN_PROGRESS'), -- Ride is currently in progress
    ('COMPLETED'),   -- Ride has been successfully completed
    ('CANCELLED')    -- Ride was cancelled
;

-- Ride type enumeration
create table "vehicle_type"("value" text not null primary key);
insert into
    "vehicle_type" ("value")
values
    ('ECONOMY'),     -- Standard economy ride
    ('PREMIUM'),     -- Premium comfort ride
    ('XL')           -- Extra large vehicle for groups
;

-- Coordinates table for real-time location tracking
create table coordinates (
    id uuid primary key default gen_random_uuid(),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    entity_id uuid not null, -- driver_id or passenger_id
    entity_type varchar(20) not null check (entity_type in ('driver', 'passenger')),
    address text not null,
    latitude decimal(10,8) not null check (latitude between -90 and 90),
    longitude decimal(11,8) not null check (longitude between -180 and 180),
    fare_amount decimal(10,2) check (fare_amount >= 0),
    distance_km decimal(8,2) check (distance_km >= 0),
    duration_minutes integer check (duration_minutes >= 0),
    is_current boolean default true
);

-- Create indexes for performance
create index idx_coordinates_entity on coordinates(entity_id, entity_type);
create index idx_coordinates_current on coordinates(entity_id, entity_type) where is_current = true;

-- Main rides table
create table rides (
    id uuid primary key default gen_random_uuid(),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    ride_number varchar(50) unique not null,
    passenger_id uuid not null references users(id),
    driver_id uuid references users(id),
    vehicle_type text references "vehicle_type"(value),
    status text references "ride_status"(value),
    priority integer default 1 check (priority between 1 and 10),
    requested_at timestamptz default now(),
    matched_at timestamptz,
    arrived_at timestamptz,
    started_at timestamptz,
    completed_at timestamptz,
    cancelled_at timestamptz,
    cancellation_reason text,
    estimated_fare decimal(10,2),
    final_fare decimal(10,2),
    pickup_coordinate_id uuid references coordinates(id),
    destination_coordinate_id uuid references coordinates(id)
);

-- Create index for status queries
create index idx_rides_status on rides(status);

-- Event type enumeration for audit trail
create table "ride_event_type"("value" text not null primary key);
insert into
    "ride_event_type" ("value")
values
    ('RIDE_REQUESTED'),    -- Initial ride request
    ('DRIVER_MATCHED'),    -- Driver assigned to ride
    ('DRIVER_ARRIVED'),    -- Driver arrived at pickup
    ('RIDE_STARTED'),      -- Ride began
    ('RIDE_COMPLETED'),    -- Ride finished
    ('RIDE_CANCELLED'),    -- Ride was cancelled
    ('STATUS_CHANGED'),    -- General status change
    ('LOCATION_UPDATED'),  -- Location update during ride
    ('FARE_ADJUSTED')      -- Fare was adjusted
;

-- Event sourcing table for complete ride audit trail
create table ride_events (
    id uuid primary key default gen_random_uuid(),
    created_at timestamptz not null default now(),
    ride_id uuid references rides(id) not null,
    event_type text references "ride_event_type"(value),
    event_data jsonb not null
);

/*
Example event_data field:
{
  "old_status": "requested",
  "new_status": "matched",
  "driver_id": "550e8400-e29b-41d4-a716-446655440001",
  "location": {"lat": 43.238949, "lng": 76.889709},
  "estimated_arrival": "2024-12-16T10:35:00Z"
}
*/

commit;