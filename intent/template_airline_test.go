package intent

import (
	"strings"
	"testing"

	"github.com/gburgyan/aat/graph"
	"github.com/gburgyan/aat/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tpGraphPath = "../airline/graph.yaml"
const tpGraphDir = "../airline"

// TestAirlineTemplates_AllValidate loads every workflow template in the
// airline graph and validates it against the graph. This catches regressions
// when graph nodes, edges, or outputs change.
func TestAirlineTemplates_AllValidate(t *testing.T) {
	g, err := graph.ParseFile(tpGraphPath)
	require.NoError(t, err)

	templatedWorkflows := 0
	for _, wf := range g.Workflows {
		if wf.Template == "" {
			continue
		}
		templatedWorkflows++
		t.Run(wf.Name, func(t *testing.T) {
			p, err := LoadWorkflowTemplate(wf.Template, tpGraphDir, g)
			require.NoError(t, err, "failed to load template for workflow %q", wf.Name)

			// Addon templates are fragments that can't validate standalone —
			// they have intentionally unfed required inputs that the LLM fills
			// at composition time. Only validate standalone workflows.
			if !wf.IsAddon() {
				err = plan.Validate(p, g)
				assert.NoError(t, err, "template validation failed for workflow %q", wf.Name)
			}
		})
	}

	// Ensure we found all 33 templated workflows (10 standalone + 4 multi-city + 6 post-commit + 6 pre-commit addons + 7 post-commit addons).
	assert.Equal(t, 33, templatedWorkflows, "expected 33 workflows with templates")
}

// TestAirlineTemplates_UnfedInputs verifies the unfed inputs for each
// workflow template. Unfed inputs are values the LLM/user must provide.
func TestAirlineTemplates_UnfedInputs(t *testing.T) {
	g, err := graph.ParseFile(tpGraphPath)
	require.NoError(t, err)

	// Unfed inputs are those not structurally wired in the plan template (no from,
	// fromSelection, select ref). Literal template defaults (e.g., origin: DEN)
	// are considered overrideable and thus "unfed" — the LLM should provide values
	// that match the user's intent. Optional inputs and graph-level defaults are
	// excluded (fed). Auto-wired graph edges don't count as plan wiring.
	tests := []struct {
		workflow string
		contains []string // substrings that should appear in unfed list
		exact    int      // exact expected count (-1 = don't check)
	}{
		{
			workflow:  "Full Booking",
			contains:  []string{"searchFlights.origin", "searchFlights.destination", "searchFlights.departureDate", "addTraveler.givenName", "addTraveler.surname"},
			exact:     5,
		},
		{
			workflow:  "1-Leg Reference Booking",
			contains:  []string{"searchFlights.origin", "searchFlights.destination", "addTraveler.givenName", "addTraveler.surname"},
			exact:     5,
		},
		{
			workflow:  "Post-Commit Ticketing",
			contains:  []string{"createItineraryFromLocator.locator", "addPayment.amount", "addPayment.currencyCode"},
			exact:     3,
		},
		{
			workflow:  "Exchange",
			contains:  []string{"getExchangeEligibility.locator", "searchExchange.origin"},
			exact:     5,
		},
		{
			workflow:  "Ancillary Booking",
			contains:  []string{"searchAncillaries.itineraryId", "addAncillaryOffer.itineraryId"},
			exact:     4,
		},
		{
			workflow:  "Seat Selection",
			contains:  []string{"searchSeatMap.offerId", "searchSeatMap.productId"},
			exact:     -1,
		},
		{
			workflow:  "Traveler Modification",
			contains:  []string{"updateTraveler.updateValue"},
			exact:     -1,
		},
		{
			workflow:  "3-City Multi-City Booking",
			contains:  []string{"searchFlights2Leg.leg1Origin", "searchFlights2Leg.leg1Destination", "searchFlights2Leg.leg1DepartureDate", "searchFlights2Leg.leg2Origin", "searchFlights2Leg.leg2Destination", "searchFlights2Leg.leg2DepartureDate", "addTraveler.givenName", "addTraveler.surname"},
			exact:     8,
		},
		{
			workflow:  "4-City Multi-City Booking",
			contains:  []string{"searchFlights3Leg.leg1Origin", "searchFlights3Leg.leg1Destination", "searchFlights3Leg.leg1DepartureDate", "searchFlights3Leg.leg2Origin", "searchFlights3Leg.leg2Destination", "searchFlights3Leg.leg2DepartureDate", "searchFlights3Leg.leg3Origin", "searchFlights3Leg.leg3Destination", "searchFlights3Leg.leg3DepartureDate", "addTraveler.givenName", "addTraveler.surname"},
			exact:     11,
		},
		{
			workflow:  "2-Leg Reference Booking",
			contains:  []string{"searchFlights2Leg.leg1Origin", "searchFlights2Leg.leg1Destination", "searchFlights2Leg.leg1DepartureDate", "searchFlights2Leg.leg2Origin", "searchFlights2Leg.leg2Destination", "searchFlights2Leg.leg2DepartureDate", "addTraveler.givenName", "addTraveler.surname"},
			exact:     8,
		},
		{
			workflow:  "3-Leg Reference Booking",
			contains:  []string{"searchFlights3Leg.leg1Origin", "searchFlights3Leg.leg1Destination", "searchFlights3Leg.leg1DepartureDate", "searchFlights3Leg.leg2Origin", "searchFlights3Leg.leg2Destination", "searchFlights3Leg.leg2DepartureDate", "searchFlights3Leg.leg3Origin", "searchFlights3Leg.leg3Destination", "searchFlights3Leg.leg3DepartureDate", "addTraveler.givenName", "addTraveler.surname"},
			exact:     11,
		},
		{
			workflow:  "4-Leg Reference Booking",
			contains:  []string{"searchFlights4Leg.leg1Origin", "searchFlights4Leg.leg1Destination", "searchFlights4Leg.leg1DepartureDate", "searchFlights4Leg.leg2Origin", "searchFlights4Leg.leg2Destination", "searchFlights4Leg.leg2DepartureDate", "searchFlights4Leg.leg3Origin", "searchFlights4Leg.leg3Destination", "searchFlights4Leg.leg3DepartureDate", "searchFlights4Leg.leg4Origin", "searchFlights4Leg.leg4Destination", "searchFlights4Leg.leg4DepartureDate", "addTraveler.givenName", "addTraveler.surname"},
			exact:     14,
		},
		{
			workflow:  "Round-Trip Full Booking",
			contains:  []string{"searchFlights.origin", "searchFlights.destination", "addTraveler.givenName"},
			exact:     -1,
		},
		{
			workflow:  "Round-Trip Leg-Based Booking",
			contains:  []string{"searchFlights.origin", "searchFlights.destination", "addTraveler.givenName"},
			exact:     -1,
		},
		{
			workflow:  "Post-Commit Seat Selection",
			contains:  nil,
			exact:     -1,
		},
		{
			workflow:  "Post-Commit Ancillary",
			contains:  nil,
			exact:     -1,
		},
		{
			workflow:  "Involuntary Schedule Change",
			contains:  nil,
			exact:     -1,
		},
		{
			workflow:  "Ticket Void",
			contains:  nil,
			exact:     -1,
		},
		{
			workflow:  "Cancel Reservation",
			contains:  nil,
			exact:     -1,
		},
		{
			workflow:  "Order Divide",
			contains:  nil,
			exact:     -1,
		},
		{
			workflow:  "Special Services",
			contains:  []string{"addSpecialService.ssrCode"},
			exact:     -1,
		},
		{
			workflow:  "Reservation Comments",
			contains:  []string{"addReservationComment.commentValue", "addReservationComment.commentName"},
			exact:     -1,
		},
		{
			workflow:  "Accounting Remarks",
			contains:  []string{"addAccountingRemark.accountingValue", "addAccountingRemark.accountingName"},
			exact:     -1,
		},
		{
			workflow:  "Document Overrides",
			contains:  []string{"addDocumentOverrides.itineraryId"},
			exact:     -1,
		},
		{
			workflow:  "Primary Contact",
			contains:  []string{"addPrimaryContact.email", "addPrimaryContact.phoneNumber", "addPrimaryContact.itineraryId"},
			exact:     3,
		},
		{
			workflow:  "Travel Agency",
			contains:  []string{"addTravelAgency.corporateCode", "addTravelAgency.email", "addTravelAgency.phoneNumber", "addTravelAgency.itineraryId"},
			exact:     4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.workflow, func(t *testing.T) {
			tmplPath, found := FindWorkflowTemplate(g, tt.workflow)
			require.True(t, found, "workflow %q should have a template", tt.workflow)

			p, err := LoadWorkflowTemplate(tmplPath, tpGraphDir, g)
			require.NoError(t, err)

			unfed := UnfedInputsFromTemplate(p, g)
			for _, expected := range tt.contains {
				hasMatch := false
				for _, u := range unfed {
					if strings.Contains(u, expected) {
						hasMatch = true
						break
					}
				}
				assert.True(t, hasMatch, "expected unfed input containing %q for workflow %q, got: %v", expected, tt.workflow, unfed)
			}
			if tt.exact >= 0 {
				assert.Equal(t, tt.exact, len(unfed), "unfed count mismatch for workflow %q: %v", tt.workflow, unfed)
			}
		})
	}
}

// TestAirlineTemplates_GoalConsistency verifies that booking workflows
// have the correct goal step and sub-workflows handle goals appropriately.
func TestAirlineTemplates_GoalConsistency(t *testing.T) {
	g, err := graph.ParseFile(tpGraphPath)
	require.NoError(t, err)

	// Workflows that should have isGoal set on their final step.
	goalWorkflows := map[string]string{
		"Full Booking":           "confirmItinerary",
		"1-Leg Reference Booking":        "confirmItinerary",
		"Post-Commit Ticketing":          "commitTicket",
		"Exchange":                        "commitExchangeTicket",
		"Round-Trip Full Booking": "confirmItinerary",
		"3-City Multi-City Booking":       "confirmItinerary",
		"4-City Multi-City Booking":       "confirmItinerary",
		"Round-Trip Leg-Based Booking":           "confirmItinerary",
		"2-Leg Reference Booking":         "confirmItinerary",
		"3-Leg Reference Booking":         "confirmItinerary",
		"4-Leg Reference Booking":         "confirmItinerary",
		"Post-Commit Seat Selection":            "confirmItinerary",
		"Post-Commit Ancillary":           "confirmItinerary",
		"Involuntary Schedule Change":     "confirmItinerary",
		"Ticket Void":                     "voidTicket",
		"Cancel Reservation":              "cancelReservation",
		"Order Divide":                    "divideReservation",
	}

	for wfName, expectedGoal := range goalWorkflows {
		t.Run(wfName, func(t *testing.T) {
			tmplPath, found := FindWorkflowTemplate(g, wfName)
			require.True(t, found)

			p, err := LoadWorkflowTemplate(tmplPath, tpGraphDir, g)
			require.NoError(t, err)

			// Find the goal step.
			var goalNode string
			goalCount := 0
			for _, step := range p.Execution.Steps {
				if step.IsGoal {
					goalNode = step.Node
					goalCount++
				}
			}
			assert.Equal(t, 1, goalCount, "workflow %q should have exactly one goal step", wfName)
			assert.Equal(t, expectedGoal, goalNode, "workflow %q goal mismatch", wfName)

			// Intent goal should match if set.
			if p.Intent.Goal != "" {
				assert.Equal(t, expectedGoal, p.Intent.Goal, "intent.goal mismatch for %q", wfName)
			}
		})
	}

	// Sub-workflows (no goal step expected).
	subWorkflows := []string{
		"Ancillary Booking",
		"Seat Selection",
		"Traveler Modification",
		"Special Services",
		"Reservation Comments",
		"Accounting Remarks",
		"Document Overrides",
		"Primary Contact",
		"Travel Agency",
		"Post-Commit Seat Selection Addon",
		"Post-Commit Ancillary Addon",
		"Cancel Booking",
		"Void Ticket",
		"Post-Commit Ticketing Addon",
		"Retrieve Booking",
		"Fare Rules Check",
	}

	for _, wfName := range subWorkflows {
		t.Run(wfName+"_NoGoal", func(t *testing.T) {
			tmplPath, found := FindWorkflowTemplate(g, wfName)
			require.True(t, found)

			p, err := LoadWorkflowTemplate(tmplPath, tpGraphDir, g)
			require.NoError(t, err)

			for _, step := range p.Execution.Steps {
				assert.False(t, step.IsGoal, "sub-workflow %q should not have isGoal on step %q", wfName, step.Node)
			}
		})
	}
}

// TestAirlineTemplates_CleanupPresent verifies that workflows that create
// itineraries have ignoreItinerary cleanup, and sub-workflows do not.
func TestAirlineTemplates_CleanupPresent(t *testing.T) {
	g, err := graph.ParseFile(tpGraphPath)
	require.NoError(t, err)

	// Workflows that should have ignoreItinerary cleanup.
	itineraryWorkflows := []string{
		"Full Booking",
		"1-Leg Reference Booking",
		"Post-Commit Ticketing",
		"Exchange",
		"Round-Trip Full Booking",
		"3-City Multi-City Booking",
		"4-City Multi-City Booking",
		"Round-Trip Leg-Based Booking",
		"2-Leg Reference Booking",
		"3-Leg Reference Booking",
		"4-Leg Reference Booking",
		"Post-Commit Seat Selection",
		"Post-Commit Ancillary",
		"Involuntary Schedule Change",
		"Order Divide",
		"Post-Commit Seat Selection Addon",
		"Post-Commit Ancillary Addon",
		"Post-Commit Ticketing Addon",
	}

	for _, wfName := range itineraryWorkflows {
		t.Run(wfName+"_HasCleanup", func(t *testing.T) {
			tmplPath, found := FindWorkflowTemplate(g, wfName)
			require.True(t, found)

			p, err := LoadWorkflowTemplate(tmplPath, tpGraphDir, g)
			require.NoError(t, err)

			hasIgnoreItinerary := false
			for _, cs := range p.Execution.Cleanup {
				if cs.Node == "ignoreItinerary" {
					hasIgnoreItinerary = true
					assert.Equal(t, "always", cs.RunOn, "ignoreItinerary should run on 'always' for %q", wfName)
				}
			}
			assert.True(t, hasIgnoreItinerary, "workflow %q should have ignoreItinerary cleanup", wfName)
		})
	}

	// Sub-workflows that should NOT have cleanup.
	noCleanupWorkflows := []string{
		"Ancillary Booking",
		"Seat Selection",
		"Traveler Modification",
		"Ticket Void",
		"Cancel Reservation",
		"Special Services",
		"Reservation Comments",
		"Accounting Remarks",
		"Document Overrides",
		"Primary Contact",
		"Travel Agency",
		"Cancel Booking",
		"Void Ticket",
		"Retrieve Booking",
		"Fare Rules Check",
	}

	for _, wfName := range noCleanupWorkflows {
		t.Run(wfName+"_NoCleanup", func(t *testing.T) {
			tmplPath, found := FindWorkflowTemplate(g, wfName)
			require.True(t, found)

			p, err := LoadWorkflowTemplate(tmplPath, tpGraphDir, g)
			require.NoError(t, err)

			assert.Empty(t, p.Execution.Cleanup, "sub-workflow %q should not have cleanup steps", wfName)
		})
	}
}
