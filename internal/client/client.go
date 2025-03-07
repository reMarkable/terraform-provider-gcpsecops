package client

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type Client struct {
	instanceURL string
	token       string
	httpClient  *http.Client
}

func New(secopsInstancePath, accesstoken string) (*Client, error) {
	return &Client{
		instanceURL: secopsInstancePath,
		token:       accesstoken,
		httpClient:  http.DefaultClient,
	}, nil
}

func (c *Client) doRequest(ctx context.Context, req *http.Request) ([]byte, error) {
	tflog.Info(ctx, "Performing request")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w failed to do request", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			tflog.Debug(ctx, "got error closing body", map[string]any{"err": err})
		}
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("%w failed to read response body", err)
	}

	if res.StatusCode != http.StatusOK {
		return body, fmt.Errorf("got error response from server. url: %s, status: %d, body: %s", req.URL, res.StatusCode, body)
	}

	return body, err
}
