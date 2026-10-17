package sonos

import (
	"net"
	"net/http"
	"time"
)

// sharedTransport is reused by all Sonos HTTP clients so that connections to
// the players are pooled instead of creating a new transport per call.
var sharedTransport = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ResponseHeaderTimeout: 5 * time.Second,
	MaxIdleConnsPerHost:   4,
	IdleConnTimeout:       30 * time.Second,
}

// soapClient is the shared HTTP client for SOAP requests to Sonos players.
var soapClient = &http.Client{
	Timeout:   DefaultTimeout,
	Transport: sharedTransport,
}
