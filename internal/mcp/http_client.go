package mcp

import (
	"net/http"
	"time"

	"github.com/SolaTyolo/httpclient"
	"github.com/SolaTyolo/httpclient/retry"
	"github.com/hashicorp/go-retryablehttp"
)

const mcpHTTPTimeout = 60 * time.Second

func httpClientFromEnv(headers map[string]string) *http.Client {
	cfg := retry.DefaultConfig()
	cfg.Timeout = mcpHTTPTimeout

	rc := retry.NewClient(cfg)
	rc.HTTPClient.Transport = headerTransport(headers)

	hc := httpclient.New(httpclient.WithRetryableClient(rc))
	if c, ok := hc.Requester().(*retryablehttp.Client); ok {
		return c.StandardClient()
	}
	return rc.StandardClient()
}

func headerTransport(headers map[string]string) http.RoundTripper {
	if len(headers) == 0 {
		return http.DefaultTransport
	}
	return &headerRoundTripper{
		base:    http.DefaultTransport,
		headers: headers,
	}
}

type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	return h.base.RoundTrip(req)
}
