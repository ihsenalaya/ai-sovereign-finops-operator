package sidecarproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	HeaderNamespace = "x-greenops-namespace"
	HeaderApp       = "x-greenops-app"
)

// Config defines how the sidecar proxy enriches outbound HTTP requests.
type Config struct {
	Namespace   string
	Application string
	Targets     []string
	Transport   http.RoundTripper
	DialContext func(context.Context, string, string) (net.Conn, error)
}

// New returns a forward proxy that injects namespace/app headers on matching
// outbound HTTP requests.
func New(cfg Config) http.Handler {
	normalizedTargets := make([]string, 0, len(cfg.Targets))
	for _, t := range cfg.Targets {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" {
			normalizedTargets = append(normalizedTargets, t)
		}
	}
	transport := cfg.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	dialContext := cfg.DialContext
	if dialContext == nil {
		dialer := &net.Dialer{Timeout: 30 * time.Second}
		dialContext = dialer.DialContext
	}
	return &proxy{
		namespace:   cfg.Namespace,
		application: cfg.Application,
		targets:     normalizedTargets,
		transport:   transport,
		dialContext: dialContext,
	}
}

type proxy struct {
	namespace   string
	application string
	targets     []string
	transport   http.RoundTripper
	dialContext func(context.Context, string, string) (net.Conn, error)
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}

	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	outReq.URL = cloneURL(r.URL)
	if outReq.URL == nil {
		http.Error(w, "missing request URL", http.StatusBadRequest)
		return
	}
	if outReq.URL.Scheme == "" {
		outReq.URL.Scheme = "http"
	}
	if outReq.URL.Host == "" {
		outReq.URL.Host = r.Host
	}

	outReq.Header = r.Header.Clone()
	outReq.Header.Del("Proxy-Connection")
	outReq.Header.Del("Connection")
	if p.shouldInject(outReq.URL.Hostname()) {
		if p.namespace != "" {
			outReq.Header.Set(HeaderNamespace, p.namespace)
		}
		if p.application != "" {
			outReq.Header.Set(HeaderApp, p.application)
		}
	}

	resp, err := p.transport.RoundTrip(outReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("proxy upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (p *proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "proxy does not support hijacking", http.StatusInternalServerError)
		return
	}
	targetConn, err := p.dialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, fmt.Sprintf("proxy connect error: %v", err), http.StatusBadGateway)
		return
	}

	clientConn, buf, err := hijacker.Hijack()
	if err != nil {
		_ = targetConn.Close()
		http.Error(w, "proxy hijack error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = buf.Flush() }()

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		_ = clientConn.Close()
		_ = targetConn.Close()
		return
	}

	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = clientConn.Close()
			_ = targetConn.Close()
		})
	}
	go func() {
		_, _ = io.Copy(targetConn, clientConn)
		closeBoth()
	}()
	go func() {
		_, _ = io.Copy(clientConn, targetConn)
		closeBoth()
	}()
}

func (p *proxy) shouldInject(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	if len(p.targets) == 0 {
		return true
	}
	for _, target := range p.targets {
		if host == target || strings.HasSuffix(host, "."+target) {
			return true
		}
	}
	return false
}

func cloneURL(in *url.URL) *url.URL {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func copyHeader(dst, src http.Header) {
	for k, values := range src {
		for _, v := range values {
			dst.Add(k, v)
		}
	}
}
