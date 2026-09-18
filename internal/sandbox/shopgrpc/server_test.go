package shopgrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aat "github.com/gburgyan/aat"
	"github.com/gburgyan/aat/internal/protoreg"
	"github.com/gburgyan/aat/internal/sandbox/shop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/dynamicpb"
)

// start runs the façade over a fresh sandbox and returns a connection to it
// plus the shop's HTTP handler, so a test can set an order up over HTTP and
// charge it over gRPC.
func start(t *testing.T) (*grpc.ClientConn, *shop.Server, *protoreg.Registry) {
	t.Helper()

	descriptors, err := aat.PaymentsDescriptorSet()
	require.NoError(t, err)

	srv := shop.New(shop.Options{Latency: 0, Seed: 1, NoAuth: true})
	facade, err := New(descriptors, srv.PaymentsHandler(), shop.DefaultRegion)
	require.NoError(t, err)

	grpcSrv := grpc.NewServer()
	require.NoError(t, facade.Register(grpcSrv))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	reg, err := protoreg.LoadDescriptorSets(filepath.Join("..", "..", "..", "examples", "grpc-payments", "payments.protoset"))
	require.NoError(t, err)
	return conn, srv, reg
}

// charge calls shop.v1.Payments/Charge with the given request JSON.
func charge(t *testing.T, conn *grpc.ClientConn, reg *protoreg.Registry, requestJSON string) (*dynamicpb.Message, error) {
	t.Helper()
	return call(t, conn, reg, "Charge", requestJSON)
}

// call invokes one Payments method with a request written as JSON.
func call(t *testing.T, conn *grpc.ClientConn, reg *protoreg.Registry, method, requestJSON string) (*dynamicpb.Message, error) {
	t.Helper()
	md, err := reg.Method("shop.v1.Payments", method)
	require.NoError(t, err)

	in, err := reg.JSONToMessage(md.Input(), []byte(requestJSON))
	require.NoError(t, err)
	out := dynamicpb.NewMessage(md.Output())

	ctx := metadata.AppendToOutgoingContext(context.Background(), apiKeyHeader, shop.DemoAPIKey)
	err = conn.Invoke(ctx, "/shop.v1.Payments/"+method, in, out)
	return out, err
}

// reply decodes a response message the way AAT reads one.
func reply(t *testing.T, reg *protoreg.Registry, out *dynamicpb.Message) map[string]any {
	t.Helper()
	body, err := reg.MessageToJSON(out)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	return decoded
}

func TestNew_RejectsABadDescriptorSet(t *testing.T) {
	_, err := New([]byte("not a descriptor set"), nil, "us")
	require.ErrorContains(t, err, "descriptor set")
}

func TestServer_ChargeUnknownOrder(t *testing.T) {
	conn, _, reg := start(t)

	_, err := charge(t, conn, reg, `{"orderId":"ord_missing","amount":"100","currency":"USD","method":"card","cardNumber":"4242424242424242"}`)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	// The shop answers 404, which the façade maps to NOT_FOUND, and its stable
	// error code survives in the message.
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "ord_missing")
}

func TestServer_ChargeValidationError(t *testing.T) {
	conn, _, reg := start(t)

	_, err := charge(t, conn, reg, `{"orderId":"ord_0001","currency":"USD","method":"card","cardNumber":"4242424242424242"}`)
	require.Error(t, err)
	st, _ := status.FromError(err)
	// A missing amount is the handler's own validation error, reached through
	// the façade rather than reimplemented in it.
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "amount")
}

// placeOrder drives the shop's HTTP API to a checked-out order, so a test can
// charge it over gRPC — the same crossing examples/grpc-payments makes.
func placeOrder(t *testing.T, srv *shop.Server) (orderID, currency string, total int64) {
	t.Helper()
	api := srv.APIHandler()

	post := func(path, body string) map[string]any {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/us/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		require.Less(t, rec.Code, 400, "%s: %s", path, rec.Body.String())
		var out map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		return out
	}

	cart := post("/carts", `{}`)
	cartID, _ := cart["cartId"].(string)
	require.NotEmpty(t, cartID)
	post("/carts/"+cartID+"/items", `{"sku":"SKU-1001","quantity":2}`)
	order := post("/carts/"+cartID+"/checkout", `{"shippingTier":"standard","postalCode":"78701"}`)

	orderID, _ = order["orderId"].(string)
	currency, _ = order["currency"].(string)
	totalF, _ := order["total"].(float64)
	require.NotEmpty(t, orderID)
	return orderID, currency, int64(totalF)
}

// TestServer_ChargeAnOrderPlacedOverHTTP is the crossing the project exists to
// show: an order created through the HTTP API, charged through the gRPC one,
// against the same store.
func TestServer_ChargeAnOrderPlacedOverHTTP(t *testing.T) {
	conn, srv, reg := start(t)
	orderID, currency, total := placeOrder(t, srv)

	out, err := charge(t, conn, reg, fmt.Sprintf(
		`{"orderId":%q,"amount":"%d","currency":%q,"method":"card","cardNumber":"4242424242424242"}`,
		orderID, total, currency))
	require.NoError(t, err)

	body, err := reg.MessageToJSON(out)
	require.NoError(t, err)

	var payment map[string]any
	require.NoError(t, json.Unmarshal(body, &payment))
	assert.Equal(t, "captured", payment["status"])
	assert.Equal(t, "paid", payment["orderStatus"], "the order moved on, so the façade reached the real store")
	assert.Equal(t, orderID, payment["orderId"])
	assert.NotEmpty(t, payment["paymentId"], "the id the HTTP API assigned survives the crossing")
	// An int64 reads back as a JSON string; this is the encoding rule plan
	// authors meet first.
	assert.Equal(t, fmt.Sprintf("%d", total), payment["amount"])
}

// TestServer_Refund covers the reply the façade once emptied: a refund is its
// own record, naming itself and the payment it came from.
func TestServer_Refund(t *testing.T) {
	conn, srv, reg := start(t)
	orderID, currency, total := placeOrder(t, srv)
	out, err := charge(t, conn, reg, fmt.Sprintf(
		`{"orderId":%q,"amount":"%d","currency":%q,"method":"card","cardNumber":"4242424242424242"}`,
		orderID, total, currency))
	require.NoError(t, err)
	paymentID := reply(t, reg, out)["paymentId"]

	// An amount of 0 is sent and refused. Read as "omitted", it would refund
	// the whole payment.
	_, err = call(t, conn, reg, "Refund", fmt.Sprintf(`{"orderId":%q,"amount":"0"}`, orderID))
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "error: %v", err)

	out, err = call(t, conn, reg, "Refund", fmt.Sprintf(`{"orderId":%q,"amount":"100"}`, orderID))
	require.NoError(t, err)
	partial := reply(t, reg, out)
	assert.NotEmpty(t, partial["refundId"])
	assert.Equal(t, paymentID, partial["paymentId"])
	assert.Equal(t, "100", partial["amount"])

	// No amount refunds what is left.
	out, err = call(t, conn, reg, "Refund", fmt.Sprintf(`{"orderId":%q}`, orderID))
	require.NoError(t, err)
	rest := reply(t, reg, out)
	assert.Equal(t, fmt.Sprintf("%d", total-100), rest["amount"])
	assert.NotEqual(t, partial["refundId"], rest["refundId"])
}

func TestServer_DeclinedCard(t *testing.T) {
	conn, srv, reg := start(t)
	orderID, currency, total := placeOrder(t, srv)

	_, err := charge(t, conn, reg, fmt.Sprintf(
		`{"orderId":%q,"amount":"%d","currency":%q,"method":"card","cardNumber":%q}`,
		orderID, total, currency, shop.DeclinedCard))
	require.Error(t, err)

	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code(), "the shop answers 402, which has no gRPC code of its own")
	assert.Contains(t, st.Message(), "CARD_DECLINED", "the API's stable error code survives")
}

func TestServer_RegionTravelsAsMetadata(t *testing.T) {
	// An HTTP path carries the region; a gRPC method has none, so it travels
	// as metadata. Charging a us order while asking for eu must miss.
	conn, srv, reg := start(t)
	orderID, currency, total := placeOrder(t, srv)

	md, err := reg.Method("shop.v1.Payments", "Charge")
	require.NoError(t, err)
	in, err := reg.JSONToMessage(md.Input(), []byte(fmt.Sprintf(
		`{"orderId":%q,"amount":"%d","currency":%q,"method":"card","cardNumber":"4242424242424242"}`,
		orderID, total, currency)))
	require.NoError(t, err)

	ctx := metadata.AppendToOutgoingContext(context.Background(),
		apiKeyHeader, shop.DemoAPIKey, regionHeader, "eu")
	err = conn.Invoke(ctx, "/shop.v1.Payments/Charge", in, dynamicpb.NewMessage(md.Output()))
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code(), "the eu region has no such order")
}

// TestServer_ServesEveryMethodTheDescriptorSetDeclares guards the dynamic
// registration: adding an RPC to the .proto and regenerating should be enough
// for it to be served, with no generated code to update.
func TestServer_ServesEveryMethodTheDescriptorSetDeclares(t *testing.T) {
	_, _, reg := start(t)
	for _, name := range []string{"Charge", "Refund"} {
		_, err := reg.Method("shop.v1.Payments", name)
		require.NoError(t, err, "the descriptor set declares %s", name)
	}
}

func TestDescriptorSetIsTheProjectsOwnFile(t *testing.T) {
	embedded, err := aat.PaymentsDescriptorSet()
	require.NoError(t, err)
	onDisk, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "grpc-payments", "payments.protoset"))
	require.NoError(t, err)
	assert.Equal(t, onDisk, embedded)
}
