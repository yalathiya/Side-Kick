package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// New creates a reverse proxy that forwards requests to the upstream URL.
// It enriches requests with X-Forwarded-* and X-Sidekick headers.
func New(upstream string) (http.Handler, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Sidekick", "true")
		req.Host = target.Host
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("X-Proxied-By", "sidekick")
		return nil
	}

	return proxy, nil
}
