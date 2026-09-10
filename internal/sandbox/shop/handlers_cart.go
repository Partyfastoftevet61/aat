package shop

import (
	"net/http"
	"strings"
	"time"
)

// handleCreateCart implements createCart (POST /carts).
func (s *Server) handleCreateCart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerEmail string `json:"customerEmail"`
	}
	if e := decodeJSON(r, &req, true); e != nil {
		writeError(w, e)
		return
	}
	if req.CustomerEmail != "" && !strings.Contains(req.CustomerEmail, "@") {
		writeError(w, validation("customerEmail must be an email address"))
		return
	}
	st := s.store(r)
	st.mu.Lock()
	defer st.mu.Unlock()
	c := &cart{ID: st.nextID("cart"), Status: cartOpen, CustomerEmail: req.CustomerEmail, CreatedAt: s.now()}
	st.carts[c.ID] = c
	writeJSON(w, http.StatusCreated, st.cartView(c))
}

// handleAddItem implements addItem (POST /carts/{cartId}/items). Checks run
// in the order validation (400) -> cart lookup (404) -> cart state (409) ->
// product lookup (404) -> stock (409).
func (s *Server) handleAddItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SKU      string `json:"sku"`
		Quantity int    `json:"quantity"`
	}
	if e := decodeJSON(r, &req, false); e != nil {
		writeError(w, e)
		return
	}
	if req.SKU == "" {
		writeError(w, validation("sku is required"))
		return
	}
	if req.Quantity < 1 {
		writeError(w, validation("quantity must be at least 1 (got %d)", req.Quantity))
		return
	}
	st := s.store(r)
	st.mu.Lock()
	defer st.mu.Unlock()
	c, ok := st.carts[r.PathValue("cartId")]
	if !ok {
		writeError(w, notFound("cart", r.PathValue("cartId")))
		return
	}
	if c.Status != cartOpen {
		writeError(w, newError(http.StatusConflict, CodeCartNotOpen, "cart %s is %s", c.ID, c.Status))
		return
	}
	p := findProduct(req.SKU)
	if p == nil {
		writeError(w, notFound("product", req.SKU))
		return
	}
	if p.Available == 0 {
		writeError(w, newError(http.StatusConflict, CodeOutOfStock, "%s (%s) is out of stock", p.SKU, p.Name))
		return
	}
	merged := false
	for i := range c.Lines {
		if c.Lines[i].SKU == req.SKU {
			c.Lines[i].Quantity += req.Quantity
			merged = true
		}
	}
	if !merged {
		c.Lines = append(c.Lines, cartLine{SKU: req.SKU, Quantity: req.Quantity})
	}
	writeJSON(w, http.StatusCreated, st.cartView(c))
}

// handleGetCart implements getCart (GET /carts/{cartId}).
func (s *Server) handleGetCart(w http.ResponseWriter, r *http.Request) {
	st := s.store(r)
	st.mu.Lock()
	defer st.mu.Unlock()
	c, ok := st.carts[r.PathValue("cartId")]
	if !ok {
		writeError(w, notFound("cart", r.PathValue("cartId")))
		return
	}
	writeJSON(w, http.StatusOK, st.cartView(c))
}

// handleDeleteCart implements deleteCart (DELETE /carts/{cartId}).
func (s *Server) handleDeleteCart(w http.ResponseWriter, r *http.Request) {
	st := s.store(r)
	st.mu.Lock()
	defer st.mu.Unlock()
	id := r.PathValue("cartId")
	if _, ok := st.carts[id]; !ok {
		writeError(w, notFound("cart", id))
		return
	}
	delete(st.carts, id)
	w.WriteHeader(http.StatusNoContent)
}

// handleApplyCoupon implements applyCoupon (POST /carts/{cartId}/coupon).
func (s *Server) handleApplyCoupon(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if e := decodeJSON(r, &req, false); e != nil {
		writeError(w, e)
		return
	}
	if req.Code == "" {
		writeError(w, validation("code is required"))
		return
	}
	st := s.store(r)
	st.mu.Lock()
	defer st.mu.Unlock()
	c, ok := st.carts[r.PathValue("cartId")]
	if !ok {
		writeError(w, notFound("cart", r.PathValue("cartId")))
		return
	}
	if c.Status != cartOpen {
		writeError(w, newError(http.StatusConflict, CodeCartNotOpen, "cart %s is %s", c.ID, c.Status))
		return
	}
	cp, ok := coupons[strings.ToUpper(req.Code)]
	if !ok {
		writeError(w, newError(http.StatusNotFound, CodeCouponInvalid, "coupon %q does not exist", req.Code))
		return
	}
	if cp.Region != "" && cp.Region != st.region.Name {
		writeError(w, newError(http.StatusUnprocessableEntity, CodeCouponWrongRegion,
			"coupon %s is only valid in region %s", cp.Code, cp.Region))
		return
	}
	c.CouponCode = cp.Code
	writeJSON(w, http.StatusOK, st.cartView(c))
}

// handleCheckout implements checkoutCart (POST /carts/{cartId}/checkout).
func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ShippingTier  string `json:"shippingTier"`
		PostalCode    string `json:"postalCode"`
		CustomerEmail string `json:"customerEmail"`
		DeliveryDate  string `json:"deliveryDate"`
		Notes         string `json:"notes"`
	}
	if e := decodeJSON(r, &req, false); e != nil {
		writeError(w, e)
		return
	}
	if req.ShippingTier == "" {
		writeError(w, validation("shippingTier is required (%s)", strings.Join(allTiers, ", ")))
		return
	}
	if !isKnownTier(req.ShippingTier) {
		writeError(w, validation("unknown shippingTier %q (%s)", req.ShippingTier, strings.Join(allTiers, ", ")))
		return
	}
	if req.PostalCode == "" {
		writeError(w, validation("postalCode is required"))
		return
	}
	if req.DeliveryDate != "" {
		if _, err := time.Parse("2006-01-02", req.DeliveryDate); err != nil {
			if _, err := time.Parse(time.RFC3339, req.DeliveryDate); err != nil {
				writeError(w, validation("deliveryDate must be YYYY-MM-DD (got %q)", req.DeliveryDate))
				return
			}
		}
	}
	st := s.store(r)
	st.mu.Lock()
	defer st.mu.Unlock()
	c, ok := st.carts[r.PathValue("cartId")]
	if !ok {
		writeError(w, notFound("cart", r.PathValue("cartId")))
		return
	}
	if c.Status != cartOpen {
		writeError(w, newError(http.StatusConflict, CodeCartNotOpen, "cart %s is %s", c.ID, c.Status))
		return
	}
	if len(c.Lines) == 0 {
		writeError(w, newError(http.StatusConflict, CodeCartEmpty, "cart %s has no items", c.ID))
		return
	}
	if _, ok := st.region.Tiers[req.ShippingTier]; !ok {
		writeError(w, newError(http.StatusUnprocessableEntity, CodeTierNotAvailable,
			"shipping tier %q is not available in region %s", req.ShippingTier, st.region.Name))
		return
	}
	email := c.CustomerEmail
	if req.CustomerEmail != "" {
		email = req.CustomerEmail
	}
	lines := st.orderLines(c)
	t := st.region.computeTotals(lines, c.CouponCode, req.ShippingTier)
	reg := st.region
	o := &order{
		ID:              st.nextID("ord"),
		ReceiptNumber:   st.nextNumber("RCPT"),
		Status:          orderCreated,
		PaymentStatus:   paymentUnpaid,
		Currency:        reg.Currency,
		ShippingTier:    req.ShippingTier,
		PostalCode:      req.PostalCode,
		CustomerEmail:   email,
		DeliveryDate:    req.DeliveryDate,
		Notes:           req.Notes,
		CouponCode:      c.CouponCode,
		Subtotal:        t.Subtotal,
		SubtotalDisplay: reg.format(t.Subtotal),
		Discount:        t.Discount,
		DiscountDisplay: reg.format(t.Discount),
		Shipping:        t.Shipping,
		ShippingDisplay: reg.format(t.Shipping),
		Tax:             t.Tax,
		TaxDisplay:      reg.format(t.Tax),
		TaxLabel:        reg.TaxLabel,
		Total:           t.Total,
		TotalDisplay:    reg.format(t.Total),
		Lines:           lines,
		CreatedAt:       rfc3339(s.now()),
	}
	st.orders[o.ID] = o
	c.Status = cartCheckedOut
	writeJSON(w, http.StatusCreated, o)
}
