package proxy

import (
	"crypto/tls"
	"github.com/goriiin/go-proxy/internal/errs"
	"log"
	"time"
)

func (p *Proxy) GetCertificate(host string) (*tls.Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cert, ok := p.certCache[host]; ok {
		if time.Now().Before(cert.Leaf.NotAfter) {
			log.Printf("Using cached certificate for %s", host)

			return cert, nil
		}
		log.Printf("certificate expired at %v for %s", cert.Leaf.NotAfter, host)

		delete(p.certCache, host)
		return nil, errs.DeprecatedCert
	}

	return nil, errs.NoCert
}
