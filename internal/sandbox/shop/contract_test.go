package shop

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	valerrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/helpers"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/require"
)

// contract validates served request/response pairs against
// examples/shop/openapi.yaml. It mirrors the engine's runtime validation
// (graph/oas/runtime.go): the path item is located first, then the
// WithPathItem validators run, so the region prefix never has to match.
type contract struct {
	model *v3.Document
	v     validator.Validator
}

var (
	contractOnce sync.Once
	contractInst *contract
	contractErr  error
)

func loadContract(t *testing.T) *contract {
	t.Helper()
	contractOnce.Do(func() {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "shop", "openapi.yaml"))
		if err != nil {
			contractErr = err
			return
		}
		doc, err := libopenapi.NewDocument(data)
		if err != nil {
			contractErr = fmt.Errorf("parsing openapi.yaml: %w", err)
			return
		}
		model, err := doc.BuildV3Model()
		if err != nil {
			contractErr = fmt.Errorf("building v3 model: %w", err)
			return
		}
		contractInst = &contract{model: &model.Model, v: validator.NewValidatorFromV3Model(&model.Model)}
	})
	require.NoError(t, contractErr)
	return contractInst
}

// specPathFor maps a served URL path (/{region}/v1/...) to the spec path
// template and its PathItem by comparing literal segments.
func (c *contract) specPathFor(urlPath string) (string, *v3.PathItem) {
	segs := strings.Split(strings.Trim(urlPath, "/"), "/")
	if len(segs) < 3 {
		return "", nil
	}
	rel := segs[2:]
	for specPath, item := range c.model.Paths.PathItems.FromOldest() {
		ss := strings.Split(strings.Trim(specPath, "/"), "/")
		if len(ss) != len(rel) {
			continue
		}
		match := true
		for i := range ss {
			if strings.HasPrefix(ss[i], "{") {
				continue
			}
			if ss[i] != rel[i] {
				match = false
				break
			}
		}
		if match {
			return specPath, item
		}
	}
	return "", nil
}

// wrap returns a handler that serves inner and then validates the exchange
// against the contract. Requests are validated only when the server accepted
// them (2xx): tests deliberately send malformed bodies to provoke 4xx.
// Responses are always validated, including error envelopes.
func (c *contract) wrap(t *testing.T, inner http.Handler) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBody, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(reqBody))

		rec := httptest.NewRecorder()
		inner.ServeHTTP(rec, r)
		res := rec.Result()
		respBody, _ := io.ReadAll(res.Body)
		for k, vs := range res.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(res.StatusCode)
		_, _ = w.Write(respBody)

		specPath, item := c.specPathFor(r.URL.Path)
		if item == nil {
			// Only versioned routes are part of the contract; unknown routes
			// must at least be 404s.
			if strings.Contains(r.URL.Path, "/v1/") && res.StatusCode != http.StatusNotFound {
				t.Errorf("contract: no spec path for %s %s (status %d)", r.Method, r.URL.Path, res.StatusCode)
			}
			return
		}

		rel := "/" + strings.Join(strings.Split(strings.Trim(r.URL.Path, "/"), "/")[2:], "/")
		vreq := r.Clone(r.Context())
		vreq.URL = &url.URL{Path: rel, RawQuery: r.URL.RawQuery}
		vreq.RequestURI = ""
		vreq.Body = io.NopCloser(bytes.NewReader(reqBody))

		label := fmt.Sprintf("%s %s -> %d", r.Method, r.URL.Path, res.StatusCode)
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			_, errs := c.v.ValidateHttpRequestSyncWithPathItem(vreq, item, specPath)
			reportContractErrors(t, "request "+label, errs)
		}
		vres := &http.Response{
			StatusCode: res.StatusCode,
			Header:     res.Header,
			Body:       io.NopCloser(bytes.NewReader(respBody)),
			Request:    vreq,
		}
		_, errs := c.v.GetResponseBodyValidator().ValidateResponseBodyWithPathItem(vreq, vres, item, specPath)
		reportContractErrors(t, "response "+label, errs)
	})
}

func reportContractErrors(t *testing.T, label string, errs []*valerrors.ValidationError) {
	t.Helper()
	for _, e := range errs {
		if e.ValidationType == helpers.SecurityValidation {
			continue // auth is exercised by dedicated tests, without headers
		}
		var details []string
		for _, se := range e.SchemaValidationErrors {
			details = append(details, fmt.Sprintf("%s (%s)", se.Reason, se.FieldPath))
		}
		t.Errorf("contract violation on %s: %s: %s %s", label, e.Message, e.Reason, strings.Join(details, "; "))
	}
}
