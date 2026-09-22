package plan

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gburgyan/aat/internal/yamlx"
	"gopkg.in/yaml.v3"
)

// dateLayout is how a plan writes a calendar date. It matches the form
// {{today}} expands to and applyOffset parses, so every date an author reads
// or writes in a plan looks the same.
const dateLayout = "2006-01-02"

// Date is a calendar date written as YYYY-MM-DD. It is a date rather than a
// timestamp: an expiry is a day, not an instant, and a plan author should not
// have to think about which zone a deadline falls in.
type Date struct {
	time.Time
}

// ParseDate reads a date written as YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("%q is not a valid date (use YYYY-MM-DD)", s)
	}
	return Date{Time: t}, nil
}

// String renders the date as YYYY-MM-DD.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.Format(dateLayout)
}

// Expired reports whether now falls after the date. The date itself is
// included, so an entry marked until 2026-10-06 is still in force all through
// the sixth and lapses on the seventh.
func (d Date) Expired(now time.Time) bool {
	if d.IsZero() {
		return false
	}
	y, m, day := now.Date()
	today := time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
	return today.After(d.Time)
}

// DaysUntil returns whole days from now until the date: 0 on the day itself,
// negative once it has passed.
func (d Date) DaysUntil(now time.Time) int {
	if d.IsZero() {
		return 0
	}
	y, m, day := now.Date()
	today := time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
	return int(d.Time.Sub(today).Hours() / 24)
}

// UnmarshalYAML reads the scalar text rather than decoding into a string,
// because YAML resolves an unquoted 2026-10-06 to a timestamp tag and a
// quoted one to a string. The node's value is the same either way.
func (d *Date) UnmarshalYAML(unmarshal func(any) error) error {
	n, err := yamlx.Node(unmarshal)
	if err != nil {
		return err
	}
	if n.Kind != yaml.ScalarNode {
		return yamlx.KindError(n, "date", "a date written as YYYY-MM-DD")
	}
	parsed, err := ParseDate(n.Value)
	if err != nil {
		return &yaml.TypeError{Errors: []string{fmt.Sprintf("line %d: %s", n.Line, err)}}
	}
	*d = parsed
	return nil
}

// MarshalYAML writes the date back in the form it was read.
func (d Date) MarshalYAML() (any, error) {
	return d.String(), nil
}

// MarshalJSON writes the date as YYYY-MM-DD rather than a timestamp, so an
// archived plan reads the way the plan file does.
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON reads the date written by MarshalJSON.
func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*d = Date{}
		return nil
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// KnownIssue tolerates a failure until a date, without hiding it.
//
// A step that fails while a known issue is in force still runs, still checks
// its assertions, and still reads as failed everywhere a run is displayed —
// but it does not turn the run red, so CI stays green on a fact somebody has
// written down. Once Until passes the entry stops applying and the failure
// counts again, which is the point: switching a test off is easy, and this
// makes forgetting to switch it back on impossible.
//
// It is not a way to quiet a flaky step. A failure that comes and goes is a
// retry rule; a known issue is for a defect that is understood, has an owner
// somewhere else, and is expected to be fixed by a date.
type KnownIssue struct {
	// Until is the last day the entry applies, written YYYY-MM-DD.
	Until Date `yaml:"until" json:"until"`
	// Reason says what the defect is and why waiting is the right response.
	// It is required: an entry nobody explained is an entry nobody can judge.
	Reason string `yaml:"reason" json:"reason"`
	// URL optionally points at the ticket, changelog, or write-up that tracks it.
	URL string `yaml:"url,omitempty" json:"url,omitempty"`
}

// Clone returns a deep copy, or nil when the receiver is nil.
func (k *KnownIssue) Clone() *KnownIssue {
	if k == nil {
		return nil
	}
	c := *k
	return &c
}

// Active reports whether the entry still applies at now.
func (k *KnownIssue) Active(now time.Time) bool {
	return k != nil && !k.Until.Expired(now)
}
