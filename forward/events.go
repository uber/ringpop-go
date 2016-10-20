package forward

// A RequestForwardedEvent is emitted for every forwarded request
type RequestForwardedEvent struct{}

// A InflightRequestsChangedEvent is emitted everytime the number of inflight requests change
type InflightRequestsChangedEvent struct {
	Inflight int64
}

// InflightCountOperation indicates the operation being performed on the inflight counter
type InflightCountOperation string

const (
	// InflightIncrement indicates that the inflight number was being incremented
	InflightIncrement InflightCountOperation = "increment"

	// InflightDecrement indicates that the inflight number was being decremented
	InflightDecrement InflightCountOperation = "decrement"
)

// A InflightRequestsMiscountEvent is emitted when a miscount happend for the inflight requests
type InflightRequestsMiscountEvent struct {
	Operation InflightCountOperation
}

// A SuccessEvent is emitted when the forwarded request responded without an error
type SuccessEvent struct{}

// A FailedEvent is emitted when the forwarded request responded with an error
type FailedEvent struct{}

// A MaxRetriesEvent is emitted when the sender failed to complete the request after the maximum specified amount of retries
type MaxRetriesEvent struct {
	MaxRetries int
}

// A RetryAttemptEvent is emitted when a retry is initiated during forwarding
type RetryAttemptEvent struct{}

// A RetryAbortEvent is emitted when a retry has been aborted. The reason for abortion is embedded
type RetryAbortEvent struct {
	Reason string
}

// A RerouteEvent is emitted when a forwarded request is being rerouted to a new destination
type RerouteEvent struct {
	OldDestination string
	NewDestination string
}

// A RetrySuccessEvent is emitted after a retry resulted in a successful forwarded request
type RetrySuccessEvent struct {
	NumRetries int
}
