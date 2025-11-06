package service

import (
	"context"
	"ride-hail/internal/domain/driver"
	"ride-hail/internal/domain/ride"
	"ride-hail/internal/ports"
	"time"
)

// GetSystemOverview collects a set of aggregate metrics about the current state of the system.
func (service *adminService) GetSystemOverview(ctx context.Context) (ports.SystemOverviewResult, error) {
	// create a new system overview result struct
	var res ports.SystemOverviewResult
	now := time.Now().UTC()
	res.Timestamp = now

	// define the start and end of the day
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := startOfDay.Add(24 * time.Hour)

	// collect the metrics within a transaction
	err := service.uow.WithinTx(ctx, func(txCtx context.Context) error {
		// ----- Ride service metrics -----

		// count the active rides
		nActive, err := service.rideRepo.CountActive(txCtx)
		if err != nil {
			return err
		}
		res.Metrics.ActiveRides = nActive

		// count the total rides today
		totalToday, err := service.rideRepo.CountCreatedBetween(txCtx, startOfDay, endOfDay)
		if err != nil {
			return err
		}
		res.Metrics.TotalRidesToday = totalToday

		// sum the total revenue today
		revenueToday, err := service.rideRepo.SumFinalFareCompletedBetween(txCtx, startOfDay, endOfDay)
		if err != nil {
			return err
		}
		res.Metrics.TotalRevenueToday = revenueToday

		// calculate the average wait time today
		avgWait, err := service.rideRepo.AvgWaitMinutesBetween(txCtx, startOfDay, endOfDay)
		if err != nil {
			return err
		}
		res.Metrics.AverageWaitTimeMinutes = avgWait

		// calculate the average ride duration today
		avgRideDur, err := service.rideRepo.AvgRideDurationMinutesBetween(txCtx, startOfDay, endOfDay)
		if err != nil {
			return err
		}
		res.Metrics.AverageRideDurationMinutes = avgRideDur

		// calculate the cancellation rate today
		cancelRate, err := service.rideRepo.CancellationRateBetween(txCtx, startOfDay, endOfDay)
		if err != nil {
			return err
		}
		res.Metrics.CancellationRate = cancelRate

		// ----- Driver service metrics -----

		// count the available drivers
		nAvailable, err := service.driverRepo.CountByStatus(txCtx, driver.DriverStatusAvailable)
		if err != nil {
			return err
		}
		res.Metrics.AvailableDrivers = nAvailable

		// count the busy drivers
		nBusy, err := service.driverRepo.CountByStatus(txCtx, driver.DriverStatusBusy)
		if err != nil {
			return err
		}

		// count the en route drivers
		nEnRoute, err := service.driverRepo.CountByStatus(txCtx, driver.DriverStatusEnRoute)
		if err != nil {
			return err
		}
		res.Metrics.BusyDrivers = nBusy + nEnRoute

		// count the economy drivers
		ecoCnt, err := service.driverRepo.CountByVehicleType(txCtx, ride.VehicleEconomy)
		if err != nil {
			return err
		}
		res.DriverDistribution.Economy = ecoCnt

		// count the premium drivers
		premCnt, err := service.driverRepo.CountByVehicleType(txCtx, ride.VehiclePremium)
		if err != nil {
			return err
		}
		res.DriverDistribution.Premium = premCnt

		// count the XL drivers
		xlCnt, err := service.driverRepo.CountByVehicleType(txCtx, ride.VehicleXL)
		if err != nil {
			return err
		}
		res.DriverDistribution.XL = xlCnt

		// ----- Hotspots metrics-----

		// get the hotspots
		hs, err := service.driverRepo.Hotspots(txCtx, 10)
		if err != nil {
			return err
		}

		// build the hotspots
		res.Hotspots = res.Hotspots[:0]
		for _, h := range hs {
			res.Hotspots = append(res.Hotspots, struct {
				Location       string `json:"location"`
				ActiveRides    int    `json:"active_rides"`
				WaitingDrivers int    `json:"waiting_drivers"`
			}{
				Location:       h.Location,
				ActiveRides:    h.ActiveRides,
				WaitingDrivers: h.WaitingDrivers,
			})
		}

		return nil
	})
	if err != nil {
		return ports.SystemOverviewResult{}, err
	}

	return res, nil
}
