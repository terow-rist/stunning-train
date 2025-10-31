package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ride-hail/internal/domain/ride"
	"ride-hail/internal/ports"

	"github.com/jackc/pgx/v5"
)

// RideRepo persists rides using pgx and plain SQL.
type RideRepo struct{}

// NewRideRepo constructs a new RideRepo.
func NewRideRepo() ports.RideRepository {
	return &RideRepo{}
}

// GetRidesByDriver returns recent rides for a driver
func (r *RideRepo) GetRidesByDriver(ctx context.Context, driverID string, limit int) ([]*ride.Ride, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("get transaction from context: %w", err)
	}

	query := `
        SELECT id, ride_number, passenger_id, driver_id, vehicle_type, status, 
               pickup_coordinate_id, destination_coordinate_id, created_at, updated_at,
               requested_at, matched_at, arrived_at, started_at, completed_at, cancelled_at,
               cancellation_reason, estimated_fare, final_fare, priority
        FROM rides 
        WHERE driver_id = $1
        ORDER BY created_at DESC 
        LIMIT $2`

	rows, err := tx.Query(ctx, query, driverID, limit)
	if err != nil {
		return nil, fmt.Errorf("query rides by driver: %w", err)
	}
	defer rows.Close()

	var rides []*ride.Ride
	for rows.Next() {
		var rd ride.Ride
		err := rows.Scan(
			&rd.ID, &rd.RideNumber, &rd.PassengerID, &rd.DriverID, &rd.VehicleType, &rd.Status,
			&rd.PickupCoordinateID, &rd.DestinationCoordinateID, &rd.CreatedAt, &rd.UpdatedAt,
			&rd.RequestedAt, &rd.MatchedAt, &rd.ArrivedAt, &rd.StartedAt, &rd.CompletedAt, &rd.CancelledAt,
			&rd.CancellationReason, &rd.EstimatedFare, &rd.FinalFare, &rd.Priority,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ride: %w", err)
		}
		rides = append(rides, &rd)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return rides, nil
}

// Create inserts a new ride row and writes an initial RIDE_REQUESTED event.
func (repo *RideRepo) CreateRide(ctx context.Context, ride *ride.Ride) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	// insert only the columns we actually have values for at creation time
	err = tx.QueryRow(ctx, `
		INSERT INTO rides (
			ride_number, passenger_id, vehicle_type, status, priority,
			estimated_fare, pickup_coordinate_id, destination_coordinate_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at, requested_at
	`,
		ride.RideNumber,
		ride.PassengerID,
		ride.VehicleType.String(),
		ride.Status.String(), // typically "REQUESTED"
		ride.Priority,
		ride.EstimatedFare, // 0 if not known yet is fine
		ride.PickupCoordinateID,
		ride.DestinationCoordinateID,
	).Scan(&ride.ID, &ride.CreatedAt, &ride.UpdatedAt, &ride.RequestedAt)
	if err != nil {
		return err
	}

	// include only what we actually know at create time
	eventData := map[string]any{
		"new_status":        ride.Status.String(),
		"estimated_arrival": nil, // no ETA yet at request time; omit or set when you can compute it
	}

	// maybe delete it later
	// if you happen to have driver assigned at creation (rare), include it
	// if ride.DriverID != nil && *ride.DriverID != "" {
	// 	eventData["driver_id"] = *ride.DriverID
	// }

	// insert RIDE_REQUESTED event
	if err := insertRideEvent(ctx, tx, ride.ID, "RIDE_REQUESTED", eventData); err != nil {
		return err
	}

	return nil
}

// GetByID fetches a ride by primary key (uuid).
func (repo *RideRepo) GetByID(ctx context.Context, id string) (*ride.Ride, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var out ride.Ride
	var vehicleType, status string

	// fetch all columns
	err = tx.QueryRow(ctx, `
		SELECT
			id, created_at, updated_at, ride_number, passenger_id, driver_id,
			vehicle_type, status, priority, requested_at, matched_at, arrived_at,
			started_at, completed_at, cancelled_at, cancellation_reason,
			estimated_fare, final_fare, pickup_coordinate_id, destination_coordinate_id
		FROM rides
		WHERE id = $1
	`, id).Scan(
		&out.ID, &out.CreatedAt, &out.UpdatedAt, &out.RideNumber, &out.PassengerID, &out.DriverID,
		&vehicleType, &status, &out.Priority, &out.RequestedAt, &out.MatchedAt, &out.ArrivedAt,
		&out.StartedAt, &out.CompletedAt, &out.CancelledAt, &out.CancellationReason,
		&out.EstimatedFare, &out.FinalFare, &out.PickupCoordinateID, &out.DestinationCoordinateID,
	)
	if err != nil {
		return nil, err
	}
	out.VehicleType = ride.VehicleType(vehicleType)
	out.Status = ride.Status(status)

	return &out, nil
}

// GetActiveForDriver fetches the most recent active (non-terminal) ride for a given driver.
func (repo *RideRepo) GetActiveForDriver(ctx context.Context, driverID string) (*ride.Ride, error) {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var out ride.Ride
	var vehicleType, status string

	// fetch all columns for the latest non-finished ride
	err = tx.QueryRow(ctx, `
		SELECT
			id, created_at, updated_at, ride_number, passenger_id, driver_id,
			vehicle_type, status, priority, requested_at, matched_at, arrived_at,
			started_at, completed_at, cancelled_at, cancellation_reason,
			estimated_fare, final_fare, pickup_coordinate_id, destination_coordinate_id
		FROM rides
		WHERE driver_id = $1
		  AND status IN ('MATCHED', 'EN_ROUTE', 'ARRIVED', 'IN_PROGRESS')
		ORDER BY created_at DESC
		LIMIT 1
	`, driverID).Scan(
		&out.ID, &out.CreatedAt, &out.UpdatedAt, &out.RideNumber, &out.PassengerID, &out.DriverID,
		&vehicleType, &status, &out.Priority, &out.RequestedAt, &out.MatchedAt, &out.ArrivedAt,
		&out.StartedAt, &out.CompletedAt, &out.CancelledAt, &out.CancellationReason,
		&out.EstimatedFare, &out.FinalFare, &out.PickupCoordinateID, &out.DestinationCoordinateID,
	)
	if err != nil {
		// no active ride found
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	out.VehicleType = ride.VehicleType(vehicleType)
	out.Status = ride.Status(status)

	return &out, nil
}

// UpdateStatus sets the ride status and stamps the corresponding timeline column.
func (repo *RideRepo) UpdateStatus(ctx context.Context, id string, status ride.Status, updatedAt time.Time) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	// lock the row and read current status to enforce transitions
	var current string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM rides
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&current)
	if err != nil {
		return err
	}

	// idempotent success
	if current == status.String() {
		return nil
	}

	// validate new status
	if !status.Valid() {
		return errors.New("invalid ride status")
	}

	// lifecycle checks: do not move out of terminal states
	if current == "COMPLETED" || current == "CANCELLED" {
		return errors.New("cannot transition from a terminal state")
	}

	// pick the timeline column to stamp for this status
	timelineColumn := timelineColumnFor(status)

	// update ride status and timeline column
	query := `
	UPDATE rides
	SET status = $1,
	    updated_at = now()
	`

	// check if we need to set a specific timeline column
	if timelineColumn != "updated_at" {
		query += `, ` + timelineColumn + ` = $2
		WHERE id = $3`
	} else {
		// don't assign updated_at twice
		query += `
		WHERE id = $3`
	}

	// execute the update
	_, err = tx.Exec(ctx, query, status.String(), updatedAt, id)
	if err != nil {
		return err
	}

	// insert status change event
	evType := specificEventTypeFor(status)
	eventData := map[string]any{
		"old_status": current,
		"new_status": status.String(),
		"timestamp":  updatedAt.UTC().Format(time.RFC3339),
	}
	if err := insertRideEvent(ctx, tx, id, evType, eventData); err != nil {
		return err
	}

	return nil
}

// AssignDriver sets driver_id, stamps matched_at, moves status to MATCHED.
func (repo *RideRepo) AssignDriver(ctx context.Context, rideID, driverID string, matchedAt time.Time) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	var current string
	var existingDriver *string
	err = tx.QueryRow(ctx, `
		SELECT status, driver_id
		FROM rides
		WHERE id = $1
		FOR UPDATE
	`, rideID).Scan(&current, &existingDriver)
	if err != nil {
		return err
	}

	// idempotent success if already assigned to the same driver and status MATCHED
	if current == "MATCHED" && existingDriver != nil && *existingDriver == driverID {
		return nil
	}

	// only allow from REQUESTED -> MATCHED
	if current != "REQUESTED" {
		return errors.New("can only assign driver when ride is in REQUESTED state")
	}

	// update ride to assign driver and set status to MATCHED
	_, err = tx.Exec(ctx, `
		UPDATE rides
		SET driver_id = $1,
		    status = 'MATCHED',
		    matched_at = $2,
		    updated_at = now()
		WHERE id = $3
	`, driverID, matchedAt, rideID)
	if err != nil {
		return err
	}

	// insert DRIVER_MATCHED event
	eventData := map[string]any{
		"old_status": current,
		"new_status": "MATCHED",
		"driver_id":  driverID,
		"matched_at": matchedAt.UTC().Format(time.RFC3339),
	}
	if err := insertRideEvent(ctx, tx, rideID, "DRIVER_MATCHED", eventData); err != nil {
		return err
	}

	return nil
}

// Complete finalizes a ride with final fare, stamps completed_at, and moves to COMPLETED.
func (repo *RideRepo) Complete(ctx context.Context, rideID string, finalFare float64, completedAt time.Time) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	var current string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM rides
		WHERE id = $1
		FOR UPDATE
	`, rideID).Scan(&current)
	if err != nil {
		return err
	}

	// idempotent success
	if current == "COMPLETED" {
		return nil
	}

	// cannot complete a cancelled ride
	if current == "CANCELLED" {
		return errors.New("cannot complete a cancelled ride")
	}

	// only allow from IN_PROGRESS -> COMPLETED
	if current != "IN_PROGRESS" {
		return errors.New("complete is only allowed from IN_PROGRESS")
	}

	// update ride to COMPLETED
	_, err = tx.Exec(ctx, `
		UPDATE rides
		SET status = 'COMPLETED',
		    final_fare = $1,
		    completed_at = $2,
		    updated_at = now()
		WHERE id = $3
	`, finalFare, completedAt, rideID)
	if err != nil {
		return err
	}

	// insert RIDE_COMPLETED event
	eventData := map[string]any{
		"old_status":   current,
		"new_status":   "COMPLETED",
		"final_fare":   finalFare,
		"completed_at": completedAt.UTC().Format(time.RFC3339),
	}
	if err := insertRideEvent(ctx, tx, rideID, "RIDE_COMPLETED", eventData); err != nil {
		return err
	}

	return nil
}

// Cancel sets cancellation_reason, stamps cancelled_at, and moves to CANCELLED.
func (repo *RideRepo) Cancel(ctx context.Context, rideID, reason string, cancelledAt time.Time) error {
	tx, err := MustTxFromContext(ctx)
	if err != nil {
		return err
	}

	var current string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM rides
		WHERE id = $1
		FOR UPDATE
	`, rideID).Scan(&current)
	if err != nil {
		return err
	}

	// idempotent success
	if current == "CANCELLED" {
		return nil
	}

	// cannot cancel a completed ride
	if current == "COMPLETED" {
		return errors.New("cannot cancel a completed ride")
	}

	// update ride to CANCELLED
	_, err = tx.Exec(ctx, `
		UPDATE rides
		SET status = 'CANCELLED',
		    cancellation_reason = $1,
		    cancelled_at = $2,
		    updated_at = now()
		WHERE id = $3
	`, reason, cancelledAt, rideID)
	if err != nil {
		return err
	}

	// insert RIDE_CANCELLED event
	eventData := map[string]any{
		"old_status":   current,
		"new_status":   "CANCELLED",
		"reason":       reason,
		"cancelled_at": cancelledAt.UTC().Format(time.RFC3339),
	}
	if err := insertRideEvent(ctx, tx, rideID, "RIDE_CANCELLED", eventData); err != nil {
		return err
	}

	return nil
}

// --- helpers ---

// insertRideEvent writes a row into ride_events with encoded event_data.
func insertRideEvent(ctx context.Context, tx pgx.Tx, rideID, eventType string, eventData any) error {
	body, err := json.Marshal(eventData)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO ride_events (ride_id, event_type, event_data)
		VALUES ($1, $2, $3::jsonb)
	`, rideID, eventType, string(body))
	return err
}

// timelineColumnFor maps a status to the timeline column that must be stamped.
func timelineColumnFor(status ride.Status) string {
	switch status {
	case ride.StatusMatched:
		return "matched_at"
	case ride.StatusEnRoute:
		return "updated_at" // no dedicated timeline column in schema for EN_ROUTE, so use updated_at
	case ride.StatusArrived:
		return "arrived_at"
	case ride.StatusInProgress:
		return "started_at"
	case ride.StatusCompleted:
		return "completed_at"
	case ride.StatusCancelled:
		return "cancelled_at"
	default:
		// fallback to updated_at only for other statuses
		return "updated_at"
	}
}

// specificEventTypeFor returns a more precise event name when appropriate.
func specificEventTypeFor(status ride.Status) string {
	switch status {
	case ride.StatusMatched:
		return "DRIVER_MATCHED"
	case ride.StatusArrived:
		return "DRIVER_ARRIVED"
	case ride.StatusInProgress:
		return "RIDE_STARTED"
	case ride.StatusCompleted:
		return "RIDE_COMPLETED"
	case ride.StatusCancelled:
		return "RIDE_CANCELLED"
	default:
		return "STATUS_CHANGED"
	}
}
