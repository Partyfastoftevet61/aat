package protoreg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitFullMethod(t *testing.T) {
	tests := []struct {
		in              string
		service, method string
		ok              bool
	}{
		{"shop.v1.Carts/CreateCart", "shop.v1.Carts", "CreateCart", true},
		{"/shop.v1.Carts/CreateCart", "shop.v1.Carts", "CreateCart", true},
		{"shop.v1.Carts.CreateCart", "shop.v1.Carts", "CreateCart", true},
		{"Carts/CreateCart", "Carts", "CreateCart", true},
		{"Carts", "", "", false},
		{"", "", "", false},
		{"/CreateCart", "", "", false},
		{"shop.v1.Carts/", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			service, method, ok := SplitFullMethod(tt.in)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.service, service)
			assert.Equal(t, tt.method, method)
		})
	}
}
