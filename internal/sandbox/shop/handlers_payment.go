package shop

import "net/http"

// Payment methods accepted by paymentCharge.
const (
	methodCard     = "card"
	methodGiftCard = "gift_card"
	methodPayPal   = "paypal"
)

// handleCharge implements paymentCharge (POST /payments/charges) on the
// payments listener. The amount must equal the order total; the order moves
// created -> paid on success. Declines are decided after the simulated
// gateway latency, like a real processor.
func (s *Server) handleCharge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID      string `json:"orderId"`
		Amount       Money  `json:"amount"`
		Currency     string `json:"currency"`
		Method       string `json:"method"`
		CardNumber   string `json:"cardNumber"`
		GiftCardCode string `json:"giftCardCode"`
		PayPalEmail  string `json:"paypalEmail"`
	}
	if e := decodeJSON(r, &req, false); e != nil {
		writeError(w, e)
		return
	}
	switch {
	case req.OrderID == "":
		writeError(w, validation("orderId is required"))
		return
	case req.Amount <= 0:
		writeError(w, validation("amount must be a positive integer in minor units"))
		return
	case req.Currency == "":
		writeError(w, validation("currency is required"))
		return
	}
	switch req.Method {
	case methodCard:
		if req.CardNumber == "" {
			writeError(w, validation("cardNumber is required for method card"))
			return
		}
	case methodGiftCard:
		if req.GiftCardCode == "" {
			writeError(w, validation("giftCardCode is required for method gift_card"))
			return
		}
	case methodPayPal:
		if req.PayPalEmail == "" {
			writeError(w, validation("paypalEmail is required for method paypal"))
			return
		}
	default:
		writeError(w, validation("method must be one of card, gift_card, paypal (got %q)", req.Method))
		return
	}

	st := s.store(r)
	st.mu.Lock()
	o, ok := st.orders[req.OrderID]
	if !ok {
		st.mu.Unlock()
		writeError(w, notFound("order", req.OrderID))
		return
	}
	if req.Currency != o.Currency {
		st.mu.Unlock()
		writeError(w, newError(http.StatusUnprocessableEntity, CodeCurrencyMismatch,
			"order %s is priced in %s, not %s", o.ID, o.Currency, req.Currency))
		return
	}
	if req.Amount != o.Total {
		st.mu.Unlock()
		writeError(w, newError(http.StatusUnprocessableEntity, CodeAmountMismatch,
			"amount %d does not match order total %d (%s)", req.Amount, o.Total, o.TotalDisplay))
		return
	}
	if e := o.transitionError("pay"); e != nil {
		st.mu.Unlock()
		writeError(w, e)
		return
	}
	st.mu.Unlock()

	s.sleep(r.Context(), paymentLatency)

	switch req.Method {
	case methodCard:
		if req.CardNumber == DeclinedCard {
			writeError(w, newError(http.StatusPaymentRequired, CodeCardDeclined,
				"card ending in %s was declined by the issuer", last4(req.CardNumber)))
			return
		}
	case methodGiftCard:
		balance, ok := giftCards[req.GiftCardCode]
		if !ok {
			writeError(w, newError(http.StatusNotFound, CodeGiftCardInvalid, "gift card %q does not exist", req.GiftCardCode))
			return
		}
		if balance < req.Amount {
			writeError(w, newError(http.StatusPaymentRequired, CodeInsufficientFunds,
				"gift card %s balance %d is less than the amount %d", req.GiftCardCode, balance, req.Amount))
			return
		}
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	o, ok = st.orders[req.OrderID]
	if !ok {
		writeError(w, notFound("order", req.OrderID))
		return
	}
	if e := o.transition("pay"); e != nil {
		writeError(w, e)
		return
	}
	p := &payment{
		ID:            st.nextID("pay"),
		OrderID:       o.ID,
		OrderStatus:   o.Status,
		Status:        paymentCaptured,
		Method:        req.Method,
		Amount:        req.Amount,
		AmountDisplay: st.region.format(req.Amount),
		Currency:      o.Currency,
		CreatedAt:     rfc3339(s.now()),
	}
	st.payments[p.ID] = p
	o.PaymentStatus = paymentCaptured
	o.PaymentID = p.ID
	writeJSON(w, http.StatusCreated, p)
}

// handleRefund implements paymentRefund (POST /payments/refunds). Refunds are
// keyed by order so callers need not track payment IDs across a slot boundary.
// Omitting amount refunds the remaining balance; a partial refund leaves the
// payment partially_refunded until the captured amount is used up.
func (s *Server) handleRefund(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID string `json:"orderId"`
		Amount  *Money `json:"amount"`
	}
	if e := decodeJSON(r, &req, false); e != nil {
		writeError(w, e)
		return
	}
	if req.OrderID == "" {
		writeError(w, validation("orderId is required"))
		return
	}
	if req.Amount != nil && *req.Amount <= 0 {
		writeError(w, validation("amount must be a positive integer in minor units; omit it to refund the remaining balance"))
		return
	}
	st := s.store(r)
	st.mu.Lock()
	defer st.mu.Unlock()
	o, ok := st.orders[req.OrderID]
	if !ok {
		writeError(w, notFound("order", req.OrderID))
		return
	}
	p := st.payments[o.PaymentID]
	if p == nil {
		writeError(w, newError(http.StatusConflict, CodePaymentNotRefundable, "order %s has no captured payment", o.ID))
		return
	}
	remaining := p.Amount - p.refunded
	if remaining == 0 {
		writeError(w, newError(http.StatusConflict, CodePaymentNotRefundable, "payment %s is already fully refunded", p.ID))
		return
	}
	amount := remaining
	if req.Amount != nil {
		amount = *req.Amount
	}
	if amount > remaining {
		writeError(w, newError(http.StatusUnprocessableEntity, CodeAmountMismatch,
			"refund amount %d exceeds the refundable balance %d", amount, remaining))
		return
	}
	rf := &refund{
		ID:            st.nextID("ref"),
		PaymentID:     p.ID,
		OrderID:       o.ID,
		Status:        paymentRefunded,
		Amount:        amount,
		AmountDisplay: st.region.format(amount),
		Currency:      o.Currency,
		CreatedAt:     rfc3339(s.now()),
	}
	st.refunds[rf.ID] = rf
	p.refunded += amount
	p.Status = paymentPartiallyRefunded
	if p.refunded == p.Amount {
		p.Status = paymentRefunded
	}
	o.PaymentStatus = p.Status
	writeJSON(w, http.StatusCreated, rf)
}

func last4(s string) string {
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}
