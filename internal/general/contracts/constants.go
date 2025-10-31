package contracts

// Exchanges
const (
	ExchangeRideTopic      = "ride_topic"
	ExchangeDriverTopic    = "driver_topic"
	ExchangeLocationFanout = "location_fanout"
)

// Queues
const (
	QueueRideRequests        = "ride_requests"
	QueueRideStatus          = "ride_status"
	QueueDriverMatching      = "driver_matching"
	QueueDriverResponses     = "driver_responses"
	QueueDriverStatus        = "driver_status"
	QueueLocationUpdatesRide = "location_updates_ride"
)

// Routing patterns
const (
	RouteRideRequestPrefix  = "ride.request."    // {ride_type}
	RouteRideStatusPrefix   = "ride.status."     // {status}
	RouteDriverRespPrefix   = "driver.response." // {ride_id}
	RouteDriverStatusPrefix = "driver.status."   // {driver_id}
)
