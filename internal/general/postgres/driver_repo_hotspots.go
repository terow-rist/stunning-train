package postgres

import (
	"context"
	"ride-hail/internal/ports"
)

// Hotspots returns top locations by combined activity (active rides + waiting drivers).
func (repo *DriverRepo) Hotspots(ctx context.Context, limit int) ([]ports.Hotspot, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}

	rows, err := tx.Query(ctx, `
		WITH waiting AS (
			SELECT c.address, COUNT(*) AS waiting_drivers
			FROM drivers d
			JOIN coordinates c
			  ON c.entity_id = d.id
			 AND c.entity_type = 'driver'
			 AND c.is_current = true
			WHERE d.status = 'AVAILABLE'
			GROUP BY c.address
		),
		active AS (
			SELECT c.address, COUNT(*) AS active_rides
			FROM drivers d
			JOIN rides r
			  ON r.driver_id = d.id
			 AND r.status IN ('EN_ROUTE','ARRIVED','IN_PROGRESS')
			JOIN coordinates c
			  ON c.entity_id = d.id
			 AND c.entity_type = 'driver'
			 AND c.is_current = true
			GROUP BY c.address
		)
		SELECT
			COALESCE(a.address, w.address) AS location,
			COALESCE(a.active_rides, 0)    AS active_rides,
			COALESCE(w.waiting_drivers, 0) AS waiting_drivers
		FROM active a
		FULL OUTER JOIN waiting w ON a.address = w.address
		ORDER BY (COALESCE(a.active_rides, 0) + COALESCE(w.waiting_drivers, 0)) DESC,
		         COALESCE(a.address, w.address)
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ports.Hotspot
	for rows.Next() {
		var h ports.Hotspot
		if err := rows.Scan(&h.Location, &h.ActiveRides, &h.WaitingDrivers); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
