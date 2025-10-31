package service

import (
	"context"
	"ride-hail/internal/ports"
	"strconv"
)

// GetActiveRides returns a paginated list of active rides.
func (service *adminService) GetActiveRides(ctx context.Context, page, pageSize string) (ports.ActiveRidesResult, error) {
	// convert page and pageSize to integers with fallback defaults
	pageInt, err := strconv.Atoi(page)
	if err != nil || pageInt < 1 {
		pageInt = 1
	}
	sizeInt, err := strconv.Atoi(pageSize)
	if err != nil || sizeInt < 1 {
		sizeInt = 10
	}

	var res ports.ActiveRidesResult
	res.Page = pageInt
	res.PageSize = sizeInt

	// collect the metrics within a transaction
	err = service.uow.WithinTx(ctx, func(txCtx context.Context) error {
		// count the active rides
		nActive, err := service.rideRepo.CountActive(txCtx)
		if err != nil {
			return err
		}
		res.TotalCount = nActive

		// page slice
		offset := (pageInt - 1) * sizeInt
		rows, err := service.rideRepo.HydrateActiveRows(txCtx, offset, sizeInt)
		if err != nil {
			return err
		}

		// map to API DTO
		res.Rides = res.Rides[:0]
		for _, r := range rows {
			res.Rides = append(res.Rides, ports.ActiveRideRow{
				RideID:              r.RideID,
				RideNumber:          r.RideNumber,
				Status:              r.Status,
				PassengerID:         r.PassengerID,
				DriverID:            r.DriverID,
				PickupAddress:       r.PickupAddress,
				DestinationAddress:  r.DestinationAddress,
				StartedAt:           r.StartedAt.UTC(),
				EstimatedCompletion: r.EstimatedCompletion.UTC(),
				CurrentDriverLocation: ports.GeoPoint{
					Latitude:  r.CurrentDriverLocation.Latitude,
					Longitude: r.CurrentDriverLocation.Longitude,
				},
				DistanceCompletedKM: r.DistanceCompletedKM,
				DistanceRemainingKM: r.DistanceRemainingKM,
			})
		}
		return nil
	})
	if err != nil {
		return ports.ActiveRidesResult{}, err
	}

	return res, nil
}
