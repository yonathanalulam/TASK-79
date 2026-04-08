package orders

import (
	"errors"
	"fmt"
)

// Order statuses
const (
	StatusCreated             = "created"
	StatusPaymentRecorded     = "payment_recorded"
	StatusCutoff              = "cutoff"
	StatusPicking             = "picking"
	StatusArrival             = "arrival"
	StatusPickup              = "pickup"
	StatusDelivery            = "delivery"
	StatusCompleted           = "completed"
	StatusCancelled           = "cancelled"
	StatusPartiallyBackordered = "partially_backordered"
	StatusSplit               = "split"
)

var ErrInvalidTransition = errors.New("invalid state transition")

// AllowedTransitions defines the state machine
var AllowedTransitions = map[string][]string{
	StatusCreated:             {StatusPaymentRecorded, StatusCutoff, StatusCancelled},
	StatusPaymentRecorded:     {StatusPicking, StatusCancelled},
	StatusCutoff:              {StatusPicking, StatusCancelled},
	StatusPicking:             {StatusArrival, StatusPartiallyBackordered, StatusCancelled},
	StatusArrival:             {StatusPickup, StatusDelivery, StatusCancelled},
	StatusPickup:              {StatusCompleted},
	StatusDelivery:            {StatusCompleted},
	StatusPartiallyBackordered: {StatusSplit, StatusPicking, StatusCancelled},
	StatusSplit:               {},
	StatusCompleted:           {},
	StatusCancelled:           {},
}

// ValidateTransition checks if a transition is allowed
func ValidateTransition(from, to string) error {
	allowed, ok := AllowedTransitions[from]
	if !ok {
		return fmt.Errorf("%w: unknown status %q", ErrInvalidTransition, from)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("%w: cannot transition from %q to %q", ErrInvalidTransition, from, to)
}

// GetAllowedTransitions returns the list of states reachable from current
func GetAllowedTransitions(current string) []string {
	return AllowedTransitions[current]
}

// IsTerminal returns true if no further transitions are possible
func IsTerminal(status string) bool {
	return len(AllowedTransitions[status]) == 0
}
