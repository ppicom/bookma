package aimharder

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"time"
)

type loggingRoundTripper struct {
	rt     http.RoundTripper
	active bool
}

func (lrt *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Log the request
	if lrt.active {
		logRequest(req)
	}

	// Perform the request
	start := time.Now()
	resp, err := lrt.rt.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		log.Printf("Request failed: %v", err)
		return nil, err
	}

	// Log the response
	if lrt.active {
		logResponse(resp, duration)
	}
	return resp, nil
}

func logRequest(req *http.Request) {
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	log.Printf("Request: %s %s\nHeaders: %v\nBody: %s\n",
		req.Method, req.URL, req.Header, string(bodyBytes))
}

func logResponse(resp *http.Response, duration time.Duration) {
	var bodyBytes []byte
	if resp.Body != nil {
		bodyBytes, _ = io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	log.Printf("Response: %s\nDuration: %v\nStatus: %d\nHeaders: %v\nBody: %s\n",
		resp.Request.URL, duration, resp.StatusCode, resp.Header, string(bodyBytes))
}
