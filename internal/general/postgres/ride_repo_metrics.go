package postgres

import (
	"context"
	"ride-hail/internal/ports"
	"time"
)

// CountActive returns the number of rides in non-terminal states (REQUESTED, MATCHED, EN_ROUTE, ARRIVED, IN_PROGRESS).
func (repo *RideRepo) CountActive(ctx context.Context) (int, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return 0, err
	}

	var n int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM rides
		WHERE status IN ('REQUESTED', 'MATCHED', 'EN_ROUTE', 'ARRIVED', 'IN_PROGRESS')
	`).Scan(&n)
	if err != nil {
		return 0, err
	}

	return n, nil
}

// CountCreatedBetween returns the number of rides that were created within the specified time range [start, end).
func (repo *RideRepo) CountCreatedBetween(ctx context.Context, start, end time.Time) (int, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return 0, err
	}

	var n int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM rides
		WHERE requested_at >= $1 AND requested_at < $2
	`, start, end).Scan(&n)
	if err != nil {
		return 0, err
	}

	return n, nil
}

// CancellationRateBetween returns the cancellation rate for rides whose request time falls within [start, end).
func (repo *RideRepo) CancellationRateBetween(ctx context.Context, start, end time.Time) (float64, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return 0, err
	}

	var total, cancelled int64
	err = tx.QueryRow(ctx, `
    SELECT
        COUNT(*) FILTER (WHERE requested_at >= $1 AND requested_at < $2) AS total_cnt,
        COUNT(*) FILTER (WHERE requested_at >= $1 AND requested_at < $2 AND status = 'CANCELLED') AS cancelled_cnt
    FROM rides
`, start, end).Scan(&total, &cancelled)
	if err != nil {
		return 0, err
	}

	if total == 0 {
		return 0, nil
	}
	return float64(cancelled) / float64(total), nil
}

// SumFinalFareCompletedBetween returns the total revenue from rides that were completed within the time range [start, end).
func (repo *RideRepo) SumFinalFareCompletedBetween(ctx context.Context, start, end time.Time) (float64, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return 0, err
	}

	var total float64
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(final_fare), 0)
		FROM rides
		WHERE status = 'COMPLETED'
		  AND completed_at >= $1 AND completed_at < $2
	`, start, end).Scan(&total)
	if err != nil {
		return 0, err
	}

	return total, nil
}

// AvgWaitMinutesBetween returns the average passenger wait time for rides within the specified time range [start, end).
func (repo *RideRepo) AvgWaitMinutesBetween(ctx context.Context, start, end time.Time) (float64, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return 0, err
	}

	var avg float64
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (matched_at - requested_at)) / 60.0), 0)
		FROM rides
		WHERE matched_at IS NOT NULL
		  AND requested_at IS NOT NULL
		  AND matched_at >= $1 AND matched_at < $2
	`, start, end).Scan(&avg)
	if err != nil {
		return 0, err
	}

	return avg, nil
}

// AvgRideDurationMinutesBetween returns the average ride duration for rides completed within the time range [start, end).
func (repo *RideRepo) AvgRideDurationMinutesBetween(ctx context.Context, start, end time.Time) (float64, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return 0, err
	}

	var avg float64
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at)) / 60.0), 0)
		FROM rides
		WHERE status = 'COMPLETED'
		  AND started_at IS NOT NULL
		  AND completed_at IS NOT NULL
		  AND completed_at >= $1 AND completed_at < $2
	`, start, end).Scan(&avg)
	if err != nil {
		return 0, err
	}

	return avg, nil
}

// HydrateActiveRows returns a page of active rides.
func (repo *RideRepo) HydrateActiveRows(ctx context.Context, offset, limit int) ([]ports.ActiveRideRow, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := tx.Query(ctx, `
		WITH base AS (
			SELECT
				r.id,
				r.ride_number,
				r.status,
				r.passenger_id,
				r.driver_id,
				r.started_at,
				pc.address AS pickup_address,
				dc.address AS destination_address,
				pc.latitude  AS pickup_lat,
				pc.longitude AS pickup_lng,
				dc.latitude  AS dest_lat,
				dc.longitude AS dest_lng
			FROM rides r
			LEFT JOIN coordinates pc ON pc.id = r.pickup_coordinate_id
			LEFT JOIN coordinates dc ON dc.id = r.destination_coordinate_id
			WHERE r.status = 'IN_PROGRESS' AND r.started_at IS NOT NULL
			ORDER BY r.started_at DESC
			OFFSET $1
			LIMIT  $2
		),
		cur AS (
			SELECT
				c.entity_id AS driver_id,
				c.latitude  AS cur_lat,
				c.longitude AS cur_lng
			FROM coordinates c
			WHERE c.entity_type = 'driver' AND c.is_current = true
		),
		latest_spd AS (
			SELECT DISTINCT ON (lh.driver_id)
				lh.driver_id,
				lh.speed_kmh
			FROM location_history lh
			ORDER BY lh.driver_id, lh.recorded_at DESC
		),
		calc AS (
			SELECT
				b.*,
				cur.cur_lat,
				cur.cur_lng,
				-- distances in km (PostGIS geography); fall back to 0.0 when any side is NULL
				COALESCE(
					ST_Distance(
						ST_MakePoint(b.pickup_lng, b.pickup_lat)::geography,
						ST_MakePoint(cur.cur_lng,  cur.cur_lat )::geography
					) / 1000.0, 0.0
				) AS dist_completed_km,
				COALESCE(
					ST_Distance(
						ST_MakePoint(cur.cur_lng,  cur.cur_lat )::geography,
						ST_MakePoint(b.dest_lng,   b.dest_lat )::geography
					) / 1000.0, 0.0
				) AS dist_remaining_km,
				-- effective speed: latest if available; default 30; guard against near-zero with 15
				CASE
					WHEN COALESCE(ls.speed_kmh, 30.0) <= 1.0 THEN 15.0
					ELSE COALESCE(ls.speed_kmh, 30.0)
				END AS eff_speed_kmh
			FROM base b
			LEFT JOIN cur        ON cur.driver_id = b.driver_id
			LEFT JOIN latest_spd ls ON ls.driver_id = b.driver_id
		)
		SELECT
			id,
			ride_number,
			status,
			passenger_id,
			driver_id,
			COALESCE(pickup_address, '')      AS pickup_address,
			COALESCE(destination_address, '') AS destination_address,
			started_at,
			COALESCE(cur_lat, 0.0)            AS cur_lat,
			COALESCE(cur_lng, 0.0)            AS cur_lng,
			dist_completed_km,
			dist_remaining_km,
			-- ETA = now + (remaining_km / eff_speed_kmh) hours
			now() + (dist_remaining_km / NULLIF(eff_speed_kmh, 0.0)) * interval '1 hour' AS estimated_completion
		FROM calc
	`, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ports.ActiveRideRow
	for rows.Next() {
		var r ports.ActiveRideRow
		if err := rows.Scan(
			&r.RideID,
			&r.RideNumber,
			&r.Status,
			&r.PassengerID,
			&r.DriverID,
			&r.PickupAddress,
			&r.DestinationAddress,
			&r.StartedAt,
			&r.CurrentDriverLocation.Latitude,
			&r.CurrentDriverLocation.Longitude,
			&r.DistanceCompletedKM,
			&r.DistanceRemainingKM,
			&r.EstimatedCompletion,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
