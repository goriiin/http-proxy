package proxy

import (
	"crypto/tls"
	"sync"
)

type Proxy struct {
	certCache map[string]*tls.Certificate
	mu        sync.Mutex
	caCert    tls.Certificate
}

func New(cert tls.Certificate) *Proxy {
	return &Proxy{
		certCache: make(map[string]*tls.Certificate),
		mu:        sync.Mutex{},
		caCert:    cert,
	}
}
