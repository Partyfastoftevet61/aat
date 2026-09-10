// Package shop implements the offline e-commerce sandbox served by the
// aat-sandbox binary: a small, deterministic REST API with two regions,
// OAuth2 and API-key auth, an order state machine, simulated latency, and two
// chaos hooks (a stale inventory read and a warming-up tracking endpoint). It
// exists so the examples/shop project can exercise AAT's features without any
// external service.
//
// The package is a leaf: it depends only on the standard library. The HTTP
// contract is documented in examples/shop/openapi.yaml, and the package tests
// validate every response against that spec.
package shop
