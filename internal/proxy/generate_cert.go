package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

func (p *Proxy) generateCert(host string) (*tls.Certificate, error) {
	hostPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate host private key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"koshenkov"},
			CommonName:   host,
		},
		Issuer:    p.caCert.Leaf.Subject,
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(8760 * time.Hour),

		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, // Required for server TLS

		BasicConstraintsValid: true,
	}

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, p.caCert.Leaf, &hostPrivKey.PublicKey, p.caCert.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(hostPrivKey)})

	generatedCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create tls key pair: %w", err)
	}

	generatedCert.Leaf, err = x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated certificate: %w", err)
	}

	p.certCache[host] = &generatedCert

	return &generatedCert, nil
}
