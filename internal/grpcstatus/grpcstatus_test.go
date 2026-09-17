package grpcstatus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNameAndHTTPStatus(t *testing.T) {
	tests := []struct {
		code uint32
		name string
		http int
	}{
		{OK, "OK", 200},
		{Canceled, "CANCELLED", 499},
		{Unknown, "UNKNOWN", 500},
		{InvalidArgument, "INVALID_ARGUMENT", 400},
		{DeadlineExceeded, "DEADLINE_EXCEEDED", 504},
		{NotFound, "NOT_FOUND", 404},
		{AlreadyExists, "ALREADY_EXISTS", 409},
		{PermissionDenied, "PERMISSION_DENIED", 403},
		{ResourceExhausted, "RESOURCE_EXHAUSTED", 429},
		{FailedPrecondition, "FAILED_PRECONDITION", 400},
		{Aborted, "ABORTED", 409},
		{OutOfRange, "OUT_OF_RANGE", 400},
		{Unimplemented, "UNIMPLEMENTED", 501},
		{Internal, "INTERNAL", 500},
		{Unavailable, "UNAVAILABLE", 503},
		{DataLoss, "DATA_LOSS", 500},
		{Unauthenticated, "UNAUTHENTICATED", 401},
	}
	require.Len(t, tests, int(MaxCode)+1, "every code is covered")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.name, Name(tt.code))
			assert.Equal(t, tt.http, HTTPStatus(tt.code))
		})
	}
}

// TestSuccessMapsBelow400 pins the property the engine depends on. Every
// comparison the engine makes is written in HTTP terms, so a gRPC step is
// judged correctly only while OK is the one code below 400.
func TestSuccessMapsBelow400(t *testing.T) {
	assert.Less(t, HTTPStatus(OK), 400, "OK is a success")
	for code := uint32(1); code <= MaxCode; code++ {
		assert.GreaterOrEqual(t, HTTPStatus(code), 400,
			"%s must read as a failure to the engine's status checks", Name(code))
	}
}

func TestUnknownCode(t *testing.T) {
	assert.Equal(t, "", Name(99))
	assert.Equal(t, 500, HTTPStatus(99), "an unrecognised code still reads as a failure")
}

func TestCodeByName(t *testing.T) {
	for _, in := range []string{"NOT_FOUND", "not_found", "NotFound", "not-found", " NOT_FOUND "} {
		code, ok := CodeByName(in)
		assert.True(t, ok, in)
		assert.Equal(t, NotFound, code, in)
	}

	t.Run("Go spells Canceled with one L", func(t *testing.T) {
		code, ok := CodeByName("CANCELED")
		require.True(t, ok)
		assert.Equal(t, Canceled, code)
	})

	t.Run("unknown name", func(t *testing.T) {
		_, ok := CodeByName("TEAPOT")
		assert.False(t, ok)
	})
}

func TestIsName(t *testing.T) {
	assert.True(t, IsName("NOT_FOUND"))
	assert.False(t, IsName("2xx"), "an HTTP status class is not a gRPC code name")
	assert.False(t, IsName(404), "a number is not a name")
	assert.False(t, IsName("TEAPOT"))
}

func TestNames(t *testing.T) {
	names := Names()
	require.Len(t, names, int(MaxCode)+1)
	assert.Equal(t, "OK", names[0])
	assert.Equal(t, "UNAUTHENTICATED", names[MaxCode])
}

func TestFromHTTPStatus(t *testing.T) {
	tests := []struct {
		status int
		want   uint32
	}{
		{200, OK}, {201, OK}, {204, OK},
		{400, InvalidArgument},
		{401, Unauthenticated},
		{403, PermissionDenied},
		{404, NotFound},
		{409, Aborted},
		{422, FailedPrecondition},
		{412, FailedPrecondition},
		{429, ResourceExhausted},
		{402, InvalidArgument}, // payment required has no code of its own
		{500, Internal},
		{501, Unimplemented},
		{503, Unavailable},
		{504, DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(Name(tt.want), func(t *testing.T) {
			assert.Equal(t, tt.want, FromHTTPStatus(tt.status), "HTTP %d", tt.status)
		})
	}
}

// TestFromHTTPStatusAgreesWithHTTPStatus checks the two directions line up
// wherever the mapping is one-to-one: a code that is the only one for its HTTP
// status must survive a round trip.
func TestFromHTTPStatusAgreesWithHTTPStatus(t *testing.T) {
	unique := map[int]uint32{}
	shared := map[int]bool{}
	for code := uint32(0); code <= MaxCode; code++ {
		status := HTTPStatus(code)
		if _, seen := unique[status]; seen {
			shared[status] = true
			continue
		}
		unique[status] = code
	}
	for status, code := range unique {
		if shared[status] {
			continue
		}
		assert.Equal(t, code, FromHTTPStatus(status),
			"%s maps to HTTP %d and should map back", Name(code), status)
	}
}
