package shop

import (
	"fmt"
	"math"
)

// Money is an amount in minor currency units (cents).
type Money int64

// regionConfig captures the pricing rules of one region. Regions have
// independent stores and independent ID sequences.
type regionConfig struct {
	Name        string
	Currency    string
	Symbol      string
	PriceFactor float64 // multiplier applied to the USD catalog price
	TaxRate     float64
	TaxIncluded bool // true: prices include tax (VAT); false: tax is added at checkout
	TaxLabel    string
	Tiers       map[string]Money // shipping tier -> cost
	Description string
}

var regions = map[string]*regionConfig{
	"us": {
		Name:        "us",
		Currency:    "USD",
		Symbol:      "$",
		PriceFactor: 1,
		TaxRate:     0.0825,
		TaxLabel:    "Sales tax 8.25%",
		Tiers:       map[string]Money{"standard": 599, "express": 1499, "overnight": 2999},
		Description: "USD, sales tax added at checkout",
	},
	"eu": {
		Name:        "eu",
		Currency:    "EUR",
		Symbol:      "€",
		PriceFactor: 0.92,
		TaxRate:     0.20,
		TaxIncluded: true,
		TaxLabel:    "VAT 20% (included)",
		Tiers:       map[string]Money{"standard": 499, "express": 1299},
		Description: "EUR, VAT included, no overnight tier",
	},
}

// regionNames lists the regions in display order.
var regionNames = []string{"us", "eu"}

// allTiers lists every shipping tier the API understands, in display order. A
// region may offer a subset (see regionConfig.Tiers).
var allTiers = []string{"standard", "express", "overnight"}

func isKnownTier(tier string) bool {
	for _, t := range allTiers {
		if t == tier {
			return true
		}
	}
	return false
}

func (r *regionConfig) format(m Money) string {
	sign := ""
	if m < 0 {
		sign = "-"
		m = -m
	}
	return fmt.Sprintf("%s%s%d.%02d", sign, r.Symbol, m/100, m%100)
}

func (r *regionConfig) price(base Money) Money {
	return Money(math.Round(float64(base) * r.PriceFactor))
}

// tax returns the tax on a taxable amount: the amount to add (US) or the
// portion already included (EU).
func (r *regionConfig) tax(taxable Money) Money {
	if r.TaxIncluded {
		net := Money(math.Round(float64(taxable) / (1 + r.TaxRate)))
		return taxable - net
	}
	return Money(math.Round(float64(taxable) * r.TaxRate))
}

func percentOff(amount Money, pct int) Money {
	return Money(math.Round(float64(amount) * float64(pct) / 100))
}

// coupon is a discount code. Region-restricted coupons are rejected with 422
// elsewhere.
type coupon struct {
	Code         string
	Description  string
	PercentOff   int
	FreeShipping bool
	Region       string // empty = valid everywhere
}

var coupons = map[string]coupon{
	"SAVE10":   {Code: "SAVE10", Description: "10% off the subtotal", PercentOff: 10},
	"FREESHIP": {Code: "FREESHIP", Description: "Free shipping on any tier", FreeShipping: true},
	"EU-ONLY":  {Code: "EU-ONLY", Description: "10% off, EU region only", PercentOff: 10, Region: "eu"},
}

// Payment test values.
const (
	DeclinedCard = "4000000000000002" // paymentCharge -> 402 CARD_DECLINED
)

// giftCards maps demo gift card codes to their balance in minor units.
var giftCards = map[string]Money{
	"GC-10-DEMO":  1000,
	"GC-100-DEMO": 10000,
	"GC-500-DEMO": 50000,
}

// totals is the money breakdown of a checkout.
type totals struct {
	Subtotal Money
	Discount Money
	Shipping Money
	Tax      Money
	Total    Money
}

// computeTotals prices a cart for a shipping tier in a region.
func (r *regionConfig) computeTotals(lines []orderLine, couponCode, tier string) totals {
	var t totals
	for _, l := range lines {
		t.Subtotal += l.LineTotal
	}
	cp, hasCoupon := coupons[couponCode]
	if hasCoupon {
		t.Discount = percentOff(t.Subtotal, cp.PercentOff)
	}
	t.Shipping = r.Tiers[tier]
	if hasCoupon && cp.FreeShipping {
		t.Shipping = 0
	}
	taxable := t.Subtotal - t.Discount
	t.Tax = r.tax(taxable)
	t.Total = taxable + t.Shipping
	if !r.TaxIncluded {
		t.Total += t.Tax
	}
	return t
}
