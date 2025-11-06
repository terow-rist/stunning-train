package service

import (
	"ride-hail/internal/ports"
)

// Service encapsulates the admin dashboard service logic and dependencies.
type adminService struct {
	uow        ports.UnitOfWork
	rideRepo   ports.RideRepository
	coordRepo  ports.CoordinatesRepository
	driverRepo ports.DriverRepository
}

// NewAdminService creates a new instance of the AdminService with the provided dependencies.
func NewAdminService(
	uow ports.UnitOfWork,
	rideRepo ports.RideRepository,
	driverRepo ports.DriverRepository,
	coordRepo ports.CoordinatesRepository,
) ports.AdminService {
	return &adminService{
		uow:        uow,
		rideRepo:   rideRepo,
		coordRepo:  coordRepo,
		driverRepo: driverRepo,
	}
}
