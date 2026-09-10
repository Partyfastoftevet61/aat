package shop

import "net/http"

// Order states.
const (
	orderCreated   = "created"
	orderPaid      = "paid"
	orderShipped   = "shipped"
	orderDelivered = "delivered"
	orderReturned  = "returned"
	orderCancelled = "cancelled"
)

// Payment states carried on an order.
const (
	paymentUnpaid   = "unpaid"
	paymentCaptured = "captured"
	paymentRefunded = "refunded"
)

// Shipment states.
const (
	shipmentInTransit = "in_transit"
	shipmentDelivered = "delivered"
)

// transitions is the order state machine: state -> action -> next state.
// Every handler that moves an order goes through transition(), so the table is
// the single source of truth for 409 INVALID_TRANSITION.
var transitions = map[string]map[string]string{
	orderCreated:   {"pay": orderPaid, "cancel": orderCancelled},
	orderPaid:      {"ship": orderShipped, "cancel": orderCancelled},
	orderShipped:   {"deliver": orderDelivered},
	orderDelivered: {"return": orderReturned},
}

// transitionError reports why an action is not allowed in the order's current
// state, or nil when it is.
func (o *order) transitionError(action string) *apiError {
	if _, ok := transitions[o.Status][action]; ok {
		return nil
	}
	return newError(http.StatusConflict, CodeInvalidTransition,
		"cannot %s an order in state %s", action, o.Status)
}

// transition applies an action, returning 409 INVALID_TRANSITION when the
// state machine forbids it.
func (o *order) transition(action string) *apiError {
	if e := o.transitionError(action); e != nil {
		return e
	}
	o.Status = transitions[o.Status][action]
	return nil
}
