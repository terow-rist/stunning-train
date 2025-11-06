package service

import (
	"ride-hail/internal/general/logger"
	"ride-hail/internal/general/rabbitmq"
	"ride-hail/internal/general/websocket"
	"ride-hail/internal/ports"
)

// Service encapsulates the ride service logic and dependencies.
type rideService struct {
	logger        *logger.Logger
	uow           ports.UnitOfWork
	rideRepo      ports.RideRepository
	rideEventRepo ports.RideEventRepository
	coordsRepo    ports.CoordinatesRepository
	driverRepo    ports.DriverRepository
	pub           *rabbitmq.MQPublisher
	rabbitmq      *rabbitmq.Client
	websocket     *websocket.WebSocket
}

// NewService creates a new instance of the RideService with the provided dependencies.
func NewRideService(
	logger *logger.Logger,
	uow ports.UnitOfWork,
	rideRepo ports.RideRepository,
	rideEventRepo ports.RideEventRepository,
	coordsRepo ports.CoordinatesRepository,
	driverRepo ports.DriverRepository,
	pub *rabbitmq.MQPublisher,
	rabbitmq *rabbitmq.Client,
	ws *websocket.WebSocket,
) ports.RideService {
	return &rideService{
		uow:           uow,
		rideRepo:      rideRepo,
		driverRepo:    driverRepo,
		coordsRepo:    coordsRepo,
		rideEventRepo: rideEventRepo,
		pub:           pub,
		logger:        logger,
		rabbitmq:      rabbitmq,
		websocket:     ws,
	}
}
