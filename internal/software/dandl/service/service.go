package service

import (
	"ride-hail/internal/general/logger"
	"ride-hail/internal/general/rabbitmq"
	"ride-hail/internal/general/websocket"
	"ride-hail/internal/ports"
)

// driverLocationService holds all dependencies required by the Driver & Location service.
type driverLocationService struct {
	logger     *logger.Logger
	uow        ports.UnitOfWork
	drivers    ports.DriverRepository
	sessions   ports.DriverSessionRepository
	coords     ports.CoordinatesRepository
	locHistory ports.LocationHistoryRepository
	rides      ports.RideRepository
	rideEvents ports.RideEventRepository
	pub        *rabbitmq.MQPublisher
	rabbitmq   *rabbitmq.Client
	websocket  *websocket.WebSocket
}

// NewDriverLocationService constructs the service with required dependencies.
func NewDriverLocationService(
	logger *logger.Logger,
	uow ports.UnitOfWork,
	drivers ports.DriverRepository,
	sessions ports.DriverSessionRepository,
	coords ports.CoordinatesRepository,
	locHistory ports.LocationHistoryRepository,
	rides ports.RideRepository,
	rideEvents ports.RideEventRepository,
	pub *rabbitmq.MQPublisher,
	rabbitmq *rabbitmq.Client,
	ws *websocket.WebSocket,
) ports.DriverLocationService {
	return &driverLocationService{
		logger:     logger,
		uow:        uow,
		drivers:    drivers,
		sessions:   sessions,
		coords:     coords,
		locHistory: locHistory,
		rides:      rides,
		rideEvents: rideEvents,
		pub:        pub,
		rabbitmq:   rabbitmq,
		websocket:  ws,
	}
}
