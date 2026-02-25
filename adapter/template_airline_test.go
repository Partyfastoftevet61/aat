package adapter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedAirlineAdapters lists all adapter names that must be loaded from
// the airline template directory. This matches the 7 graph nodes in
// graph/testdata/valid/airline_booking.yaml.
var expectedAirlineAdapters = []string{
	"airline.searchFlights",
	"airline.priceOffer",
	"airline.createItinerary",
	"airline.addOffer",
	"airline.addTraveler",
	"airline.commitBooking",
	"airline.ignoreItinerary",
}

func loadAirlineRegistry(t *testing.T) *Registry {
	t.Helper()
	registry := NewRegistry()
	count, err := LoadTemplates("testdata/templates/airline", registry)
	require.NoError(t, err)
	require.Equal(t, 7, count)
	return registry
}

func TestAirlineTemplates_Load(t *testing.T) {
	registry := loadAirlineRegistry(t)

	names := registry.Names()
	assert.Len(t, names, 7)
	for _, name := range expectedAirlineAdapters {
		_, err := registry.Get(name)
		assert.NoError(t, err, "adapter %q should be registered", name)
	}
}

func TestAirlineTemplates_BuildRequest(t *testing.T) {
	registry := loadAirlineRegistry(t)

	tests := []struct {
		name        string
		adapterName string
		inputs      map[string]any
		wantMethod  string
		wantPath    string
		// bodyContains lists strings that must appear in the request body.
		bodyContains []string
		// bodyIsEmpty is true for requests with no body (DELETE).
		bodyIsEmpty bool
	}{
		{
			name:        "searchFlights",
			adapterName: "airline.searchFlights",
			inputs: map[string]any{
				"origin":        "JFK",
				"destination":   "LAX",
				"departureDate": "2025-09-15",
				"passengers":    2,
			},
			wantMethod: "POST",
			wantPath:   "/v1/air/catalog/search/flightoffers",
			bodyContains: []string{
				`"@type": "FlightOffersQueryRequest"`,
				`"@type": "FlightOffersRequest"`,
				`"@type": "PassengerCriteria"`,
				`"value": "JFK"`,
				`"value": "LAX"`,
				`"departureDate": "2025-09-15"`,
				`"number": 2`,
			},
		},
		{
			name:        "priceOffer",
			adapterName: "airline.priceOffer",
			inputs: map[string]any{
				"catalogOfferingsId": "cat-offer-123",
				"offeringId":         "o1",
				"productRef":         "p0",
			},
			wantMethod: "POST",
			wantPath:   "/v1/air/price/offers/fromflightoffers",
			bodyContains: []string{
				`"@type": "OfferQueryFromFlightOffers"`,
				`"@type": "FromFlightOffersRequest"`,
				`"value": "cat-offer-123"`,
				`"value": "o1"`,
				`"value": "p0"`,
			},
		},
		{
			name:        "createItinerary",
			adapterName: "airline.createItinerary",
			inputs:      map[string]any{},
			wantMethod:  "POST",
			wantPath:    "/v1/air/book/session/reservationitinerary",
			bodyContains: []string{
				`"@type": "ReservationID"`,
			},
		},
		{
			name:        "addOffer",
			adapterName: "airline.addOffer",
			inputs: map[string]any{
				"itineraryId":        "wb-789",
				"catalogOfferingsId": "cat-session-123",
				"offeringId":         "o1",
				"productRef":         "p0",
			},
			wantMethod: "POST",
			wantPath:   "/v1/air/book/airoffer/reservationitinerary/wb-789/offers/fromflightoffers",
			bodyContains: []string{
				`"@type": "OfferQueryFromFlightOffers"`,
				`"@type": "FromFlightOffersRequest"`,
				`"value": "cat-session-123"`,
				`"value": "o1"`,
				`"value": "p0"`,
			},
		},
		{
			name:        "addTraveler",
			adapterName: "airline.addTraveler",
			inputs: map[string]any{
				"itineraryId":       "wb-789",
				"surname":          "Smith",
				"givenName":        "John",
				"birthDate":        "1990-01-15",
				"gender":           "Male",
				"passengerTypeCode": "ADT",
			},
			wantMethod: "POST",
			wantPath:   "/v1/air/book/traveler/reservationitinerary/wb-789/travelers",
			bodyContains: []string{
				`"@type": "Traveler"`,
				`"@type": "PersonNameDetail"`,
				`"Surname": "Smith"`,
				`"Given": "John"`,
				`"birthDate": "1990-01-15"`,
				`"gender": "Male"`,
				`"passengerTypeCode": "ADT"`,
				`"@type": "Telephone"`,
			},
		},
		{
			name:        "commitBooking",
			adapterName: "airline.commitBooking",
			inputs: map[string]any{
				"itineraryId": "wb-789",
			},
			wantMethod: "POST",
			wantPath:   "/v1/air/book/reservation/reservations/wb-789",
			bodyContains: []string{
				`"@type": "ReservationQueryConfirmItinerary"`,
			},
		},
		{
			name:        "ignoreItinerary",
			adapterName: "airline.ignoreItinerary",
			inputs: map[string]any{
				"itineraryId": "wb-789",
			},
			wantMethod:  "DELETE",
			wantPath:    "/v1/air/book/session/reservationitinerary/wb-789",
			bodyIsEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := registry.Get(tt.adapterName)
			require.NoError(t, err)

			req, err := adapter.BuildRequest(tt.inputs, &EnvironmentConfig{})
			require.NoError(t, err)

			assert.Equal(t, tt.wantMethod, req.Method)
			assert.Equal(t, tt.wantPath, req.Path)

			if tt.bodyIsEmpty {
				assert.Nil(t, req.Body)
				return
			}

			assert.Equal(t, "application/json", req.Headers["Content-Type"])
			require.NotNil(t, req.Body, "expected body for %s", tt.adapterName)
			// Verify body is valid JSON.
			assert.True(t, json.Valid(req.Body), "body should be valid JSON: %s", string(req.Body))
			bodyStr := string(req.Body)
			for _, want := range tt.bodyContains {
				assert.True(t, strings.Contains(bodyStr, want),
					"body should contain %q, got:\n%s", want, bodyStr)
			}
		})
	}
}

func TestAirlineTemplates_ExtractOutputs(t *testing.T) {
	registry := loadAirlineRegistry(t)

	tests := []struct {
		name        string
		adapterName string
		responseJSON string
		wantOutputs map[string]any
	}{
		{
			name:        "searchFlights extracts offerings and ID",
			adapterName: "airline.searchFlights",
			responseJSON: `{
				"FlightOffersResponse": {
					"FlightOffers": {
						"Identifier": {"value": "cat-session-abc"},
						"FlightOffer": [
							{"id": "offering-1"},
							{"id": "offering-2"}
						]
					}
				}
			}`,
			wantOutputs: map[string]any{
				"catalogOfferingsId": "cat-session-abc",
				"catalogOfferings": []any{
					map[string]any{"id": "offering-1"},
					map[string]any{"id": "offering-2"},
				},
			},
		},
		{
			name:        "priceOffer extracts offerListId, offerId, price and currency",
			adapterName: "airline.priceOffer",
			responseJSON: `{
				"OfferListResponse": {
					"Identifier": {"value": "e491538a-0c64-4804-ba44-bf9a8e1d8604_PC"},
					"OfferID": [
						{
							"id": "o0",
							"Price": {
								"TotalPrice": 542.50,
								"CurrencyCode": {"value": "USD"}
							}
						}
					]
				}
			}`,
			wantOutputs: map[string]any{
				"offerListId":  "e491538a-0c64-4804-ba44-bf9a8e1d8604_PC",
				"offerId":      "o0",
				"totalPrice":   json.Number("542.50"),
				"currencyCode": "USD",
			},
		},
		{
			name:        "createItinerary extracts itineraryId",
			adapterName: "airline.createItinerary",
			responseJSON: `{
				"ReservationResponse": {
					"Reservation": {
						"Identifier": {"value": "wb-new-456"}
					}
				}
			}`,
			wantOutputs: map[string]any{
				"itineraryId": "wb-new-456",
			},
		},
		{
			name:        "addOffer extracts offer identifier",
			adapterName: "airline.addOffer",
			responseJSON: `{
				"OfferListResponse": {
					"OfferID": [
						{
							"Identifier": {
								"authority": "Airline",
								"value": "223ca57d-2744-467b-b3a8-22ab0f120e0f"
							}
						}
					]
				}
			}`,
			wantOutputs: map[string]any{
				"offerStatus": "223ca57d-2744-467b-b3a8-22ab0f120e0f",
			},
		},
		{
			name:        "addTraveler extracts travelerId",
			adapterName: "airline.addTraveler",
			responseJSON: `{
				"TravelerResponse": {
					"Traveler": {
						"Identifier": {"value": "tvl-789"}
					}
				}
			}`,
			wantOutputs: map[string]any{
				"travelerId": "tvl-789",
			},
		},
		{
			name:        "commitBooking extracts locator",
			adapterName: "airline.commitBooking",
			responseJSON: `{
				"ReservationResponse": {
					"Reservation": {
						"Receipt": [
							{
								"Confirmation": {
									"Locator": {"source": "1G", "value": "ABC123"}
								}
							}
						]
					}
				}
			}`,
			wantOutputs: map[string]any{
				"locator":       "ABC123",
				"locatorSource": "1G",
			},
		},
		{
			name:        "ignoreItinerary extracts nothing",
			adapterName: "airline.ignoreItinerary",
			responseJSON: `{}`,
			wantOutputs: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := registry.Get(tt.adapterName)
			require.NoError(t, err)

			resp := &Response{
				StatusCode: 200,
				Body:       []byte(tt.responseJSON),
			}

			outputs, err := adapter.ExtractOutputs(resp)
			require.NoError(t, err)

			for key, want := range tt.wantOutputs {
				assert.Equal(t, want, outputs[key], "output %q", key)
			}
			assert.Len(t, outputs, len(tt.wantOutputs))
		})
	}
}
