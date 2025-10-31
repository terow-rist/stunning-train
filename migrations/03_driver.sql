begin;

-- Driver status enumeration
create table "driver_status"("value" text not null primary key);
insert into
    "driver_status" ("value")
values
    ('OFFLINE'),      -- Driver is not accepting rides
    ('AVAILABLE'),    -- Driver is available to accept rides
    ('BUSY'),         -- Driver is currently occupied
    ('EN_ROUTE')      -- Driver is on the way to pickup
;

-- Main drivers table
create table drivers (
    id uuid primary key references users(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    license_number varchar(50) unique not null,
    vehicle_type text references "vehicle_type"(value),
    vehicle_attrs jsonb,
    rating decimal(3,2) default 5.0 check (rating between 1.0 and 5.0),
    total_rides integer default 0 check (total_rides >= 0),
    total_earnings decimal(10,2) default 0 check (total_earnings >= 0),
    status text references "driver_status"(value),
    is_verified boolean default false
);

-- Create index for status queries
create index idx_drivers_status on drivers(status);

/* vehicle_attrs example:
{
  "vehicle_make": "Toyota",
  "vehicle_model": "Camry",
  "vehicle_color": "White",
  "vehicle_plate": "KZ 123 ABC",
  "vehicle_year": 2020
}
*/

-- Driver sessions for tracking online/offline times
create table driver_sessions (
    id uuid primary key default gen_random_uuid(),
    driver_id uuid references drivers(id) not null,
    started_at timestamptz not null default now(),
    ended_at timestamptz,
    total_rides integer default 0,
    total_earnings decimal(10,2) default 0
);

-- Location history for analytics and dispute resolution
create table location_history (
    id uuid primary key default gen_random_uuid(),
    coordinate_id uuid references coordinates(id),
    driver_id uuid references drivers(id),
    latitude decimal(10,8) not null check (latitude between -90 and 90),
    longitude decimal(11,8) not null check (longitude between -180 and 180),
    accuracy_meters decimal(6,2),
    speed_kmh decimal(5,2),
    heading_degrees decimal(5,2) check (heading_degrees between 0 and 360),
    recorded_at timestamptz not null default now(),
    ride_id uuid references rides(id)
);

commit;