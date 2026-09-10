package shop

import "net/http"

type productListJSON struct {
	Region   string        `json:"region"`
	Currency string        `json:"currency"`
	Products []productJSON `json:"products"`
}

// handleListProducts implements listProducts (GET /products?category=).
func (s *Server) handleListProducts(w http.ResponseWriter, r *http.Request) {
	st := s.store(r)
	category := r.URL.Query().Get("category")
	out := productListJSON{Region: st.region.Name, Currency: st.region.Currency, Products: []productJSON{}}
	for i := range catalog {
		if category != "" && catalog[i].Category != category {
			continue
		}
		out.Products = append(out.Products, st.productView(&catalog[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleCheckInventory implements checkInventory (GET /inventory/{sku}). It
// always answers 200 with a status envelope; the first read of StaleSKU per
// bearer token reports status ERROR / STALE_READ so plans can exercise
// response-body error detection and retry.
func (s *Server) handleCheckInventory(w http.ResponseWriter, r *http.Request) {
	sku := r.PathValue("sku")
	p := findProduct(sku)
	if p == nil {
		writeError(w, notFound("product", sku))
		return
	}
	st := s.store(r)
	st.mu.Lock()
	defer st.mu.Unlock()
	token := tokenFromContext(r.Context())
	if sku == StaleSKU && !st.staleSeen[token] {
		st.staleSeen[token] = true
		writeJSON(w, http.StatusOK, inventoryJSON{
			Status:       "ERROR",
			SKU:          sku,
			ErrorCode:    "STALE_READ",
			ErrorMessage: "inventory cache for " + sku + " is stale; retry the read",
		})
		return
	}
	writeJSON(w, http.StatusOK, inventoryJSON{Status: "OK", SKU: sku, Available: p.Available})
}
