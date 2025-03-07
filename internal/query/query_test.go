package query_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"terraform-provider-secops/internal/query"
	"testing"
)

type Body struct {
	StrField   string `json:"str_field,omitzero"`
	IntField   int    `json:"int_field,omitzero"`
	BoolField  bool   `json:"bool_field,omitzero"`
	SliceField []struct {
		NestedField string `json:"nested_field,omitzero"`
	} `json:"slice_field,omitzero"`
	StructField struct {
		NestedField string `json:"nested_field,omitzero"`
	} `json:"struct_field,omitzero"`
}

func echoserver(w http.ResponseWriter, r *http.Request) {
	bb, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	//nolint:errcheck
	w.Write(bb)
}

func giverserver[T any](body T) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		bytes, err := json.Marshal(body)
		if err != nil {
			// This can only happen if the test is broken beyond belief
			panic("giverserver is broken wtf")
		}

		//nolint:errcheck
		w.Write(bytes)
	}
}

func modifyserver[T any](m func(T) T) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		bb, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var b T
		err = json.Unmarshal(bb, &b)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resp := m(b)

		respb, err := json.Marshal(resp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		//nolint:errcheck
		w.Write(respb)
	}
}

func statusserver(code int) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
	}
}

func emptyserver(w http.ResponseWriter, r *http.Request) {
	//nolint:errcheck
	w.Write([]byte{})
}

func TestDelete(t *testing.T) {
	cases := []struct {
		name   string
		server http.HandlerFunc
		expErr error
	}{
		{
			name:   "emptyserver",
			server: emptyserver,
		},
		{
			name:   "echoserver",
			server: echoserver,
		},
		{
			name:   "statusserver",
			server: statusserver(429),
			expErr: &query.NotOKError{
				Status: 429,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(tc.server)

			err := query.Delete(context.Background(), ts.URL)
			var jerr *query.NotOKError
			if errors.As(err, &jerr) {
				expErr, ok := tc.expErr.(*query.NotOKError)
				if !ok {
					t.Fatal("failed to convert error to NotOKError")
				}
				if jerr.Status != expErr.Status {
					t.Fatalf("expected error with status %d but got %d", expErr.Status, jerr.Status)
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	in := Body{
		StrField:  "you've been given",
		IntField:  1337,
		BoolField: false,
		SliceField: []struct {
			NestedField string `json:"nested_field,omitzero"`
		}{},
		StructField: struct {
			NestedField string `json:"nested_field,omitzero"`
		}{},
	}
	ts := httptest.NewServer(http.HandlerFunc(giverserver(in)))
	resp, err := query.Get[Body](context.Background(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(in, *resp) {
		t.Fatalf("expected %+#v but got %+#v", in, *resp)
	}
}

func TestQuery(t *testing.T) {
	cases := []struct {
		name    string
		in      Body
		expOut  Body
		expErr  query.NotOKError
		handler http.HandlerFunc
		method  string
	}{
		{
			name:    "handles nil body on GET",
			handler: echoserver,
			in: Body{
				StrField: "ayy",
				IntField: 42,
			},
			expOut: Body{
				StrField: "ayy",
				IntField: 42,
			},
			method: http.MethodPost,
		},
		{
			name:    "unchanged body is equal",
			handler: echoserver,
			in: Body{
				StrField: "ayy",
				IntField: 42,
			},
			expOut: Body{
				StrField: "ayy",
				IntField: 42,
			},
			method: http.MethodPost,
		},
		{
			name: "added body field should appear",
			handler: modifyserver(func(b Body) Body {
				b.BoolField = true
				return b
			}),
			in: Body{
				StrField: "ayy",
				IntField: 42,
			},
			expOut: Body{
				StrField:  "ayy",
				IntField:  42,
				BoolField: true,
			},
			method: http.MethodPost,
		},
		{
			name: "removed body field should disappear",
			handler: modifyserver(func(b Body) Body {
				b.IntField = 0
				return b
			}),
			in: Body{
				StrField: "ayy",
				IntField: 42,
			},
			expOut: Body{
				StrField: "ayy",
			},
			method: http.MethodPost,
		},
		{
			name:    "should handle struct fields",
			handler: echoserver,
			in: Body{
				StructField: struct {
					NestedField string `json:"nested_field,omitzero"`
				}{
					NestedField: "nested-field",
				},
			},
			expOut: Body{
				StructField: struct {
					NestedField string `json:"nested_field,omitzero"`
				}{
					NestedField: "nested-field",
				},
			},

			method: http.MethodPost,
		},
		{
			name:    "should return NotOKError on GET for 400",
			handler: statusserver(http.StatusBadRequest),
			expErr: query.NotOKError{
				Status: http.StatusBadRequest,
			},
			method: http.MethodGet,
		},
		{
			name:    "should return NotOKError on GET for 401",
			handler: statusserver(http.StatusUnauthorized),
			expErr: query.NotOKError{
				Status: http.StatusUnauthorized,
			},
			method: http.MethodGet,
		},
		{
			name:    "should return NotOKError on GET for 403",
			handler: statusserver(http.StatusForbidden),
			expErr: query.NotOKError{
				Status: http.StatusForbidden,
			},
			method: http.MethodGet,
		},
		{
			name:    "should return NotOKError on GET for 429",
			handler: statusserver(http.StatusTooManyRequests),
			expErr: query.NotOKError{
				Status: http.StatusTooManyRequests,
			},
			method: http.MethodGet,
		},
		{
			name:    "should return NotOKError on GET for 500",
			handler: statusserver(http.StatusInternalServerError),
			expErr: query.NotOKError{
				Status: http.StatusInternalServerError,
			},
			method: http.MethodGet,
		},
		{
			name:    "should return NotOKError on GET for 502",
			handler: statusserver(http.StatusBadGateway),
			expErr: query.NotOKError{
				Status: http.StatusBadGateway,
			},
			method: http.MethodGet,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts := httptest.NewServer(tc.handler)
			out, err := query.Query[Body, Body](context.Background(), tc.method, ts.URL, tc.in)
			if err != nil {
				jerr := &query.NotOKError{
					Status: tc.expErr.Status,
				}
				if errors.As(err, &jerr) {
					err, ok := err.(*query.NotOKError)
					if !ok {
						t.Fatalf("could not map err to expected type. err was %+#v", err)
					}
					if jerr.Status != tc.expErr.Status {
						t.Fatalf("expected status %d, got %d", tc.expErr.Status, err.Status)
					}
					return
				}
				t.Fatal(err)
			}

			if !reflect.DeepEqual(tc.expOut, *out) {
				t.Fatalf("got unexpected result. \n\n In was %+#v \n\n Out was %+#v", tc.in, *out)
			}
		})
	}
}
