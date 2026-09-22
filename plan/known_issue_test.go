package plan

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gburgyan/aat/internal/yamlx"
)

func TestKnownIssue_DateParsing(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    string
		wantErr string
	}{
		{
			name: "unquoted, which YAML resolves as a timestamp",
			yaml: "until: 2026-10-06\nreason: a defect\n",
			want: "2026-10-06",
		},
		{
			name: "quoted, which YAML resolves as a string",
			yaml: "until: \"2026-10-06\"\nreason: a defect\n",
			want: "2026-10-06",
		},
		{
			name:    "a timestamp is not a date",
			yaml:    "until: 2026-10-06T12:00:00Z\nreason: a defect\n",
			wantErr: "not a valid date",
		},
		{
			name:    "prose is not a date",
			yaml:    "until: next Tuesday\nreason: a defect\n",
			wantErr: "not a valid date",
		},
		{
			name:    "a list is not a date",
			yaml:    "until: [2026-10-06]\nreason: a defect\n",
			wantErr: "date",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ki KnownIssue
			err := yamlx.Decode([]byte(tt.yaml), &ki)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, ki.Until.String())
		})
	}
}

// An unknown key inside knownIssue is an error, as everywhere else in a
// project file: a misspelled "reson" must not silently leave the entry
// unexplained.
func TestKnownIssue_UnknownKeyRejected(t *testing.T) {
	var ki KnownIssue
	err := yamlx.Decode([]byte("until: 2026-10-06\nreson: a typo\n"), &ki)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown key")
	assert.Contains(t, err.Error(), "reson")
}

func TestKnownIssue_Validation(t *testing.T) {
	until, err := ParseDate("2026-10-06")
	require.NoError(t, err)

	tests := []struct {
		name string
		ki   *KnownIssue
		want []string
	}{
		{name: "absent is fine", ki: nil},
		{name: "complete is fine", ki: &KnownIssue{Until: until, Reason: "a defect"}},
		{
			name: "a reason is required",
			ki:   &KnownIssue{Until: until},
			want: []string{"knownIssue.reason is required"},
		},
		{
			name: "whitespace is not a reason",
			ki:   &KnownIssue{Until: until, Reason: "   "},
			want: []string{"knownIssue.reason is required"},
		},
		{
			name: "an expiry is required — it is the whole point",
			ki:   &KnownIssue{Reason: "a defect"},
			want: []string{"knownIssue.until is required"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateKnownIssue("step 0 (refund)", tt.ki)
			require.Len(t, errs, len(tt.want))
			for i, want := range tt.want {
				assert.Contains(t, errs[i], want)
				assert.Contains(t, errs[i], "step 0 (refund)")
			}
		})
	}
}

// Validate does not judge the date against today: it has no clock, and an
// entry that has lapsed is the run's business, not an offline check's.
func TestKnownIssue_ValidateIgnoresAPastDate(t *testing.T) {
	past, err := ParseDate("2020-01-01")
	require.NoError(t, err)
	assert.Empty(t, validateKnownIssue("step 0 (refund)", &KnownIssue{Until: past, Reason: "long gone"}))
}

func TestKnownIssue_ExpiryBoundaries(t *testing.T) {
	until, err := ParseDate("2026-10-06")
	require.NoError(t, err)
	at := func(s string) time.Time {
		d, perr := time.Parse("2006-01-02", s)
		require.NoError(t, perr)
		return d
	}

	assert.False(t, until.Expired(at("2026-10-05")), "the day before")
	assert.False(t, until.Expired(at("2026-10-06")), "the day itself is included")
	assert.True(t, until.Expired(at("2026-10-07")), "the day after")

	assert.Equal(t, 1, until.DaysUntil(at("2026-10-05")))
	assert.Equal(t, 0, until.DaysUntil(at("2026-10-06")))
	assert.Equal(t, -1, until.DaysUntil(at("2026-10-07")))
}

// A step's entry survives instantiation, which deep-copies every step.
func TestKnownIssue_SurvivesDeepCopy(t *testing.T) {
	until, err := ParseDate("2026-10-06")
	require.NoError(t, err)
	orig := Step{Node: "refund", KnownIssue: &KnownIssue{Until: until, Reason: "a defect"}}

	cp := deepCopyStep(orig)
	require.NotNil(t, cp.KnownIssue)
	assert.Equal(t, "a defect", cp.KnownIssue.Reason)

	cp.KnownIssue.Reason = "changed"
	assert.Equal(t, "a defect", orig.KnownIssue.Reason, "the copy must not share the original's entry")
}

func TestKnownIssue_RoundTripsThroughYAMLAndJSON(t *testing.T) {
	until, err := ParseDate("2026-10-06")
	require.NoError(t, err)
	ki := KnownIssue{Until: until, Reason: "a defect", URL: "https://example.com/1"}

	// The date is written back as a date, not as a timestamp.
	out, err := ki.Until.MarshalYAML()
	require.NoError(t, err)
	assert.Equal(t, "2026-10-06", out)

	data, err := ki.Until.MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `"2026-10-06"`, string(data))

	var back Date
	require.NoError(t, back.UnmarshalJSON(data))
	assert.Equal(t, "2026-10-06", back.String())
}
