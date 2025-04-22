package proxy

import (
	"crypto/tls"
	"github.com/goriiin/go-proxy/internal/store"
	"sync"
)

type Proxy struct {
	certCache map[string]*tls.Certificate
	mu        sync.Mutex
	caCert    tls.Certificate
	store     *store.Store
}

func New(cert tls.Certificate, s *store.Store) *Proxy {
	return &Proxy{
		certCache: make(map[string]*tls.Certificate),
		mu:        sync.Mutex{},
		caCert:    cert,
		store:     s,
	}
}
