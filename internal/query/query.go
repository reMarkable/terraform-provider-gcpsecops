// Query provides a convenience wrapper around the standard http library.
// The main method you likely want to use is Query
package query

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var ErrReadBody = errors.New("error in readBody")

// ReadBody takes a requesst, and attempts to parse the response. It will attempt to map.
// the response to a `*T`. If unmarshalling or reading fails, it errors.
func ReadBody[T any](ctx context.Context, resp *http.Response) (*T, error) {
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w failed to read body", err)
	}
	defer resp.Body.Close()

	var b T
	if len(bytes) == 0 {
		return nil, nil
	}

	if err := json.Unmarshal(bytes, &b); err != nil {
		return nil, fmt.Errorf("%w failed to unmarshal body %s", err, string(bytes))
	}

	tflog.Debug(ctx, "got responsebody:", map[string]any{
		"string_body": string(bytes),
		"obj":         b,
	})

	return &b, nil
}

// WithBearer populates the auth header with a value.
func WithBearer(val string) func(*http.Request) *http.Request {
	return func(r *http.Request) *http.Request {
		r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", val))
		return r
	}
}

// CreateBody marshals the input, and puts it in a reader. Returns an emptystring reader if body is nil.
func CreateBody(body any) (io.ReadCloser, error) {
	if body == nil {
		return io.NopCloser(strings.NewReader("")), nil
	}
	bb, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(bb)), nil
}

type QueryOpt = func(*http.Request) *http.Request

// NotOKError is conditionally returned by Query, if it gets a non-OK response.
type NotOKError struct {
	Status int
	err    error
}

func (e *NotOKError) Unwrap() error {
	return e.err
}

func (e *NotOKError) Error() string {
	return fmt.Sprintf("got non-ok status %d", e.Status)
}

// Query wraps a bunch of boilerplate for your convenience. It's intended to help you express that you
// want to call endpoint x, passing body y, and expecting response z.
func Query[ReqBody any, RespBody any](ctx context.Context, method, url string, body ReqBody, opts ...QueryOpt) (*RespBody, error) {
	tflog.Debug(ctx, fmt.Sprintf("%s on %s with ReqBody of type %T, RespBody of type %T", method, url, *new(ReqBody), *new(RespBody)))
	tflog.Debug(ctx, "createBody")
	b, err := CreateBody(body)
	if err != nil {
		return nil, err
	}

	tflog.Debug(ctx, "createRequest")
	req, err := http.NewRequest(method, url, b)
	if err != nil {
		return nil, err
	}

	tflog.Debug(ctx, "applyOpts")
	for _, opt := range opts {
		req = opt(req)
	}

	tflog.Debug(ctx, "doRequest")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	tflog.Debug(ctx, "readBody")
	res, err := ReadBody[RespBody](ctx, resp)
	if err != nil {
		return res, err
	}

	tflog.Debug(ctx, "parseStatus")
	if resp.StatusCode != http.StatusOK {
		return res, &NotOKError{Status: resp.StatusCode, err: nil}
	}

	tflog.Debug(ctx, "return")
	return res, nil
}

// PatchM wraps Patch, but makes the request body and response types the same, which is a frequent usecase.
func PatchM[DTO any](ctx context.Context, url string, body DTO, opts ...QueryOpt) (*DTO, error) {
	return Patch[DTO, DTO](ctx, url, body, opts...)
}

// PostM wraps Patch, but makes the request body and response types the same, which is a frequent usecase.
func PostM[DTO any](ctx context.Context, url string, body DTO, opts ...QueryOpt) (*DTO, error) {
	return Post[DTO, DTO](ctx, url, body, opts...)
}

// Patch performs a Patch request.
func Patch[ReqBody any, RespBody any](ctx context.Context, url string, body ReqBody, opts ...QueryOpt) (*RespBody, error) {
	return Query[ReqBody, RespBody](ctx, http.MethodPatch, url, body, opts...)
}

// Post performs a Post request.
func Post[ReqBody any, RespBody any](ctx context.Context, url string, body ReqBody, opts ...QueryOpt) (*RespBody, error) {
	return Query[ReqBody, RespBody](ctx, http.MethodPost, url, body, opts...)
}

// Get performs a Get request.
func Get[RespBody any](ctx context.Context, url string, opts ...QueryOpt) (*RespBody, error) {
	return Query[[]byte, RespBody](ctx, http.MethodGet, url, nil, opts...)
}

func Delete(ctx context.Context, url string, opts ...QueryOpt) error {
	if _, err := Query[[]byte, []byte](ctx, http.MethodDelete, url, nil, opts...); err != nil {
		return err
	}
	return nil
}
