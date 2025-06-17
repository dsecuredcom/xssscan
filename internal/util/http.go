// internal/util/http.go
package util

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dsecuredcom/xssscan/internal/payload"
	"github.com/valyala/fasthttp"
)

type HTTPConfig struct {
	Timeout  time.Duration
	Proxy    string
	Insecure bool
	MaxConns int
}

type HTTPClient struct {
	client   *fasthttp.Client
	proxyURL *url.URL
	useProxy bool
}

type Response struct {
	StatusCode  int
	Body        []byte
	ContentType string
}

func NewHTTPClient(config HTTPConfig) *HTTPClient {
	// Automatically enable insecure mode when using proxy (common for testing tools like Burp)
	insecureMode := config.Insecure
	if config.Proxy != "" {
		insecureMode = true
		if !config.Insecure {
			fmt.Printf("[+] Auto-enabling insecure mode for proxy (required for intercepting proxies like Burp)\n")
		}
	}

	client := &fasthttp.Client{
		ReadTimeout:     config.Timeout,
		WriteTimeout:    config.Timeout,
		MaxConnsPerHost: config.MaxConns,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: insecureMode,
		},
		MaxIdleConnDuration: 2 * time.Second,
	}

	httpClient := &HTTPClient{
		client:   client,
		useProxy: false,
	}

	// Set up proxy if provided
	if config.Proxy != "" {
		proxyStr := config.Proxy
		if !strings.Contains(proxyStr, "://") {
			proxyStr = "http://" + proxyStr
		}

		proxyURL, err := url.Parse(proxyStr)
		if err != nil {
			fmt.Printf("[!] Invalid proxy URL %q: %v\n", config.Proxy, err)
			return httpClient
		}

		httpClient.proxyURL = proxyURL
		httpClient.useProxy = true

		// Set custom dialer for proxy
		client.Dial = httpClient.dialThroughProxy

		fmt.Printf("[+] Using proxy: %s\n", proxyStr)
	}

	return httpClient
}

func (c *HTTPClient) dialThroughProxy(addr string) (net.Conn, error) {
	// Connect to proxy server
	proxyConn, err := net.DialTimeout("tcp", c.proxyURL.Host, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}

	// For HTTPS connections, send CONNECT request
	if strings.HasSuffix(addr, ":443") {
		connectRequest := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Connection: keep-alive\r\n\r\n", addr, addr)

		_, err = proxyConn.Write([]byte(connectRequest))
		if err != nil {
			proxyConn.Close()
			return nil, fmt.Errorf("failed to write CONNECT request: %w", err)
		}

		// Read the response
		reader := bufio.NewReader(proxyConn)
		resp, err := http.ReadResponse(reader, nil)
		if err != nil {
			proxyConn.Close()
			return nil, fmt.Errorf("failed to read CONNECT response: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode != 200 {
			proxyConn.Close()
			return nil, fmt.Errorf("proxy CONNECT failed with status: %d %s", resp.StatusCode, resp.Status)
		}
	}

	return proxyConn, nil
}

func (c *HTTPClient) Request(ctx context.Context, method, targetURL string, payloads []payload.Payload) (*Response, error) {
	// If using proxy and it's HTTP, modify the request URL to be absolute
	if c.useProxy && !strings.HasPrefix(targetURL, "https://") {
		// For HTTP requests through proxy, we need to use absolute URLs
		return c.requestThroughHTTPProxy(ctx, method, targetURL, payloads)
	}

	// Standard FastHTTP request (for HTTPS or no proxy)
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	// Set User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:15.0) Gecko/20100101 Firefox/15.0.1")

	if method == "GET" {
		if err := c.buildGETRequest(req, targetURL, payloads); err != nil {
			return nil, fmt.Errorf("building GET request: %w", err)
		}
	} else {
		if err := c.buildPOSTRequest(req, targetURL, payloads); err != nil {
			return nil, fmt.Errorf("building POST request: %w", err)
		}
	}

	// Execute request
	err := c.client.DoTimeout(req, resp, c.client.ReadTimeout)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	// Copy response body
	b := resp.Body()
	bodyCopy := append([]byte(nil), b...)

	contentType := string(resp.Header.ContentType())

	return &Response{
		StatusCode:  resp.StatusCode(),
		Body:        bodyCopy,
		ContentType: contentType,
	}, nil
}

func (c *HTTPClient) requestThroughHTTPProxy(ctx context.Context, method, targetURL string, payloads []payload.Payload) (*Response, error) {
	// For HTTP requests through proxy, we need to handle this differently
	// Connect directly to proxy and send absolute URL
	conn, err := net.DialTimeout("tcp", c.proxyURL.Host, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}
	defer conn.Close()

	var requestLine string
	if method == "GET" {
		u, err := url.Parse(targetURL)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		for _, p := range payloads {
			q.Set(p.Parameter, p.Value)
		}
		u.RawQuery = q.Encode()
		requestLine = fmt.Sprintf("GET %s HTTP/1.1\r\n", u.String())
	} else {
		var formData []string
		for _, p := range payloads {
			formData = append(formData, url.QueryEscape(p.Parameter)+"="+url.QueryEscape(p.Value))
		}
		body := strings.Join(formData, "&")

		requestLine = fmt.Sprintf("POST %s HTTP/1.1\r\n", targetURL)
		requestLine += "Content-Type: application/x-www-form-urlencoded\r\n"
		requestLine += fmt.Sprintf("Content-Length: %d\r\n", len(body))
		requestLine += "User-Agent: Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:15.0) Gecko/20100101 Firefox/15.0.1\r\n"
		requestLine += "\r\n"
		requestLine += body
	}

	if method == "GET" {
		requestLine += "User-Agent: Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:15.0) Gecko/20100101 Firefox/15.0.1\r\n"
		requestLine += "\r\n"
	}

	// Send request
	_, err = conn.Write([]byte(requestLine))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	defer resp.Body.Close()

	// Read body with limit
	bodyBytes := make([]byte, 2*1024*1024) // 2MB limit
	n, _ := resp.Body.Read(bodyBytes)

	contentType := resp.Header.Get("Content-Type")

	return &Response{
		StatusCode:  resp.StatusCode,
		Body:        bodyBytes[:n],
		ContentType: contentType,
	}, nil
}

func (c *HTTPClient) buildGETRequest(req *fasthttp.Request, targetURL string, payloads []payload.Payload) error {
	u, err := url.Parse(targetURL)
	if err != nil {
		return err
	}

	q := u.Query()
	for _, p := range payloads {
		q.Set(p.Parameter, p.Value)
	}
	u.RawQuery = q.Encode()

	req.SetRequestURI(u.String())
	req.Header.SetMethod("GET")

	return nil
}

func (c *HTTPClient) buildPOSTRequest(req *fasthttp.Request, targetURL string, payloads []payload.Payload) error {
	req.SetRequestURI(targetURL)
	req.Header.SetMethod("POST")
	req.Header.SetContentType("application/x-www-form-urlencoded")

	var formData []string
	for _, p := range payloads {
		formData = append(formData, url.QueryEscape(p.Parameter)+"="+url.QueryEscape(p.Value))
	}

	req.SetBodyString(strings.Join(formData, "&"))

	return nil
}
