package shop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Error codes returned in the {"error": {"code": ..., "message": ...}} envelope.
const (
	CodeUnauthorized         = "UNAUTHORIZED"
	CodeNotFound             = "NOT_FOUND"
	CodeValidation           = "VALIDATION_ERROR"
	CodeOutOfStock           = "OUT_OF_STOCK"
	CodeCartNotOpen          = "CART_NOT_OPEN"
	CodeCartEmpty            = "CART_EMPTY"
	CodeInvalidTransition    = "INVALID_TRANSITION"
	CodeTierNotAvailable     = "TIER_NOT_AVAILABLE"
	CodeCouponInvalid        = "COUPON_INVALID"
	CodeCouponWrongRegion    = "COUPON_NOT_VALID_IN_REGION"
	CodeCardDeclined         = "CARD_DECLINED"
	CodeInsufficientFunds    = "INSUFFICIENT_FUNDS"
	CodeGiftCardInvalid      = "GIFT_CARD_INVALID"
	CodeAmountMismatch       = "AMOUNT_MISMATCH"
	CodeCurrencyMismatch     = "CURRENCY_MISMATCH"
	CodePaymentNotRefundable = "PAYMENT_NOT_REFUNDABLE"
	CodeTrackingUnavailable  = "TRACKING_UNAVAILABLE"
	CodeRegionNotFound       = "REGION_NOT_FOUND"
	CodeWrongListener        = "WRONG_LISTENER"
)

// apiError is an HTTP error with a stable machine-readable code.
type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string { return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Message) }

func newError(status int, code, format string, args ...any) *apiError {
	return &apiError{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

func notFound(kind, id string) *apiError {
	return newError(http.StatusNotFound, CodeNotFound, "%s %q not found", kind, id)
}

func validation(format string, args ...any) *apiError {
	return newError(http.StatusBadRequest, CodeValidation, format, args...)
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

func writeError(w http.ResponseWriter, e *apiError) {
	writeJSON(w, e.Status, errorEnvelope{Error: errorBody{Code: e.Code, Message: e.Message}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// decodeJSON parses a JSON request body into v. An empty body is accepted only
// when allowEmpty is set.
func decodeJSON(r *http.Request, v any, allowEmpty bool) *apiError {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return validation("reading request body: %v", err)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		if allowEmpty {
			return nil
		}
		return validation("request body is required")
	}
	if err := json.Unmarshal(body, v); err != nil {
		return validation("malformed JSON body: %v", err)
	}
	return nil
}
