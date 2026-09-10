package shop

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// product is a catalog entry. Prices are USD minor units; regions scale them.
type product struct {
	SKU       string
	Name      string
	Category  string
	BasePrice Money
	Available int
}

// catalog is fixed: six SKUs across three categories. SKU-1005 is out of
// stock and SKU-1004 is the stale-read chaos SKU.
var catalog = []product{
	{SKU: "SKU-1001", Name: "Trail Backpack", Category: "gear", BasePrice: 8999, Available: 42},
	{SKU: "SKU-1002", Name: "Camp Stove", Category: "gear", BasePrice: 4599, Available: 17},
	{SKU: "SKU-1003", Name: "Merino Tee", Category: "apparel", BasePrice: 3499, Available: 120},
	{SKU: "SKU-1004", Name: "Rain Shell", Category: "apparel", BasePrice: 12999, Available: 9},
	{SKU: "SKU-1005", Name: "Trail Runners", Category: "footwear", BasePrice: 11999, Available: 0},
	{SKU: "SKU-1006", Name: "Wool Socks", Category: "footwear", BasePrice: 1899, Available: 300},
}

// StaleSKU is the SKU whose first inventory read per bearer token reports
// status ERROR / STALE_READ. A retry succeeds.
const StaleSKU = "SKU-1004"

// TrackingWarmupCalls is how many getShipment calls per shipment return 503
// before tracking data appears.
const TrackingWarmupCalls = 2

func findProduct(sku string) *product {
	for i := range catalog {
		if catalog[i].SKU == sku {
			return &catalog[i]
		}
	}
	return nil
}

// Cart states.
const (
	cartOpen       = "open"
	cartCheckedOut = "checked_out"
)

type cartLine struct {
	SKU      string
	Quantity int
}

type cart struct {
	ID            string
	Status        string
	CustomerEmail string
	CouponCode    string
	Lines         []cartLine
	CreatedAt     time.Time
}

type orderLine struct {
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice Money  `json:"unitPrice"`
	LineTotal Money  `json:"lineTotal"`
}

// order doubles as the JSON representation; unexported fields never serialize.
type order struct {
	ID              string      `json:"orderId"`
	ReceiptNumber   string      `json:"receiptNumber"`
	Status          string      `json:"status"`
	PaymentStatus   string      `json:"paymentStatus"`
	Currency        string      `json:"currency"`
	ShippingTier    string      `json:"shippingTier"`
	PostalCode      string      `json:"postalCode"`
	CustomerEmail   string      `json:"customerEmail,omitempty"`
	DeliveryDate    string      `json:"deliveryDate,omitempty"`
	Notes           string      `json:"notes,omitempty"`
	CouponCode      string      `json:"couponCode,omitempty"`
	Subtotal        Money       `json:"subtotal"`
	SubtotalDisplay string      `json:"subtotalDisplay"`
	Discount        Money       `json:"discount"`
	DiscountDisplay string      `json:"discountDisplay"`
	Shipping        Money       `json:"shipping"`
	ShippingDisplay string      `json:"shippingDisplay"`
	Tax             Money       `json:"tax"`
	TaxDisplay      string      `json:"taxDisplay"`
	TaxLabel        string      `json:"taxLabel"`
	Total           Money       `json:"total"`
	TotalDisplay    string      `json:"totalDisplay"`
	ShipmentID      string      `json:"shipmentId,omitempty"`
	PaymentID       string      `json:"paymentId,omitempty"`
	Lines           []orderLine `json:"lines"`
	CreatedAt       string      `json:"createdAt"`
}

type shipment struct {
	ID             string `json:"shipmentId"`
	OrderID        string `json:"orderId"`
	OrderStatus    string `json:"orderStatus"`
	Status         string `json:"status"`
	Carrier        string `json:"carrier"`
	TrackingNumber string `json:"trackingNumber"`
	ETA            string `json:"eta"`
	CreatedAt      string `json:"createdAt"`
	polls          int
}

type payment struct {
	ID            string `json:"paymentId"`
	OrderID       string `json:"orderId"`
	OrderStatus   string `json:"orderStatus"`
	Status        string `json:"status"`
	Method        string `json:"method"`
	Amount        Money  `json:"amount"`
	AmountDisplay string `json:"amountDisplay"`
	Currency      string `json:"currency"`
	CreatedAt     string `json:"createdAt"`
}

type refund struct {
	ID            string `json:"refundId"`
	PaymentID     string `json:"paymentId"`
	OrderID       string `json:"orderId"`
	Status        string `json:"status"`
	Amount        Money  `json:"amount"`
	AmountDisplay string `json:"amountDisplay"`
	Currency      string `json:"currency"`
	CreatedAt     string `json:"createdAt"`
}

type orderReturn struct {
	ID          string `json:"returnId"`
	RMANumber   string `json:"rmaNumber"`
	OrderID     string `json:"orderId"`
	OrderStatus string `json:"orderStatus"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"createdAt"`
}

// store holds one region's data. Handlers hold mu for the duration of their
// critical section; simulated latency sleeps happen outside the lock.
type store struct {
	mu        sync.Mutex
	region    *regionConfig
	carts     map[string]*cart
	orders    map[string]*order
	shipments map[string]*shipment
	payments  map[string]*payment
	refunds   map[string]*refund
	returns   map[string]*orderReturn
	seq       map[string]int
	staleSeen map[string]bool // bearer token -> stale read already served
	rng       *rand.Rand
}

func newStore(region *regionConfig, seed int64) *store {
	return &store{
		region:    region,
		carts:     map[string]*cart{},
		orders:    map[string]*order{},
		shipments: map[string]*shipment{},
		payments:  map[string]*payment{},
		refunds:   map[string]*refund{},
		returns:   map[string]*orderReturn{},
		seq:       map[string]int{},
		staleSeen: map[string]bool{},
		rng:       rand.New(rand.NewSource(seed)), //nolint:gosec // deterministic demo data, not security
	}
}

// nextID returns sequential IDs such as cart_0001, ord_0002.
func (st *store) nextID(prefix string) string {
	st.seq[prefix]++
	return fmt.Sprintf("%s_%04d", prefix, st.seq[prefix])
}

// nextNumber returns human-facing numbers such as RCPT-US-0001, RMA-EU-0003.
func (st *store) nextNumber(prefix string) string {
	key := "num:" + prefix
	st.seq[key]++
	return fmt.Sprintf("%s-%s-%04d", prefix, strings.ToUpper(st.region.Name), st.seq[key])
}

func (st *store) trackingNumber() string {
	return fmt.Sprintf("1Z%09d", st.rng.Intn(1_000_000_000))
}

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func dateOnly(t time.Time) string { return t.UTC().Format("2006-01-02") }

// JSON views ---------------------------------------------------------------

type productJSON struct {
	SKU          string `json:"sku"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	Price        Money  `json:"price"`
	PriceDisplay string `json:"priceDisplay"`
	Currency     string `json:"currency"`
	InStock      bool   `json:"inStock"`
}

func (st *store) productView(p *product) productJSON {
	price := st.region.price(p.BasePrice)
	return productJSON{
		SKU:          p.SKU,
		Name:         p.Name,
		Category:     p.Category,
		Price:        price,
		PriceDisplay: st.region.format(price),
		Currency:     st.region.Currency,
		InStock:      p.Available > 0,
	}
}

type cartLineJSON struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

type cartProductJSON struct {
	SKU          string `json:"sku"`
	Name         string `json:"name"`
	Price        Money  `json:"price"`
	PriceDisplay string `json:"priceDisplay"`
}

type cartJSON struct {
	CartID          string            `json:"cartId"`
	Status          string            `json:"status"`
	Currency        string            `json:"currency"`
	CustomerEmail   string            `json:"customerEmail,omitempty"`
	CouponCode      string            `json:"couponCode,omitempty"`
	LineCount       int               `json:"lineCount"`
	Subtotal        Money             `json:"subtotal"`
	SubtotalDisplay string            `json:"subtotalDisplay"`
	Discount        Money             `json:"discount"`
	DiscountDisplay string            `json:"discountDisplay"`
	Lines           []cartLineJSON    `json:"lines"`
	Products        []cartProductJSON `json:"products"`
	CreatedAt       string            `json:"createdAt"`
}

// cartView renders a cart. Lines carry only sku+quantity; the products array
// holds the referenced catalog entries so clients (and AAT's Lua transform)
// join the two.
func (st *store) cartView(c *cart) cartJSON {
	r := st.region
	v := cartJSON{
		CartID:        c.ID,
		Status:        c.Status,
		Currency:      r.Currency,
		CustomerEmail: c.CustomerEmail,
		CouponCode:    c.CouponCode,
		Lines:         []cartLineJSON{},
		Products:      []cartProductJSON{},
		CreatedAt:     rfc3339(c.CreatedAt),
	}
	var subtotal Money
	for _, l := range c.Lines {
		p := findProduct(l.SKU)
		price := r.price(p.BasePrice)
		subtotal += price * Money(l.Quantity)
		v.Lines = append(v.Lines, cartLineJSON(l))
		v.Products = append(v.Products, cartProductJSON{SKU: p.SKU, Name: p.Name, Price: price, PriceDisplay: r.format(price)})
	}
	v.LineCount = len(c.Lines)
	v.Subtotal = subtotal
	v.SubtotalDisplay = r.format(subtotal)
	if cp, ok := coupons[c.CouponCode]; ok {
		v.Discount = percentOff(subtotal, cp.PercentOff)
	}
	v.DiscountDisplay = r.format(v.Discount)
	return v
}

// orderLines snapshots cart lines with the region's prices.
func (st *store) orderLines(c *cart) []orderLine {
	lines := make([]orderLine, 0, len(c.Lines))
	for _, l := range c.Lines {
		p := findProduct(l.SKU)
		price := st.region.price(p.BasePrice)
		lines = append(lines, orderLine{
			SKU:       p.SKU,
			Name:      p.Name,
			Quantity:  l.Quantity,
			UnitPrice: price,
			LineTotal: price * Money(l.Quantity),
		})
	}
	return lines
}

type inventoryJSON struct {
	Status       string `json:"status"`
	SKU          string `json:"sku"`
	Available    int    `json:"available"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}
