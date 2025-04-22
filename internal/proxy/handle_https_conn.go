package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

func (p *Proxy) handleHTTPSConnect(clientConn net.Conn, targetHost string) {
	host, port, err := net.SplitHostPort(targetHost)
	originalTargetHost := targetHost
	if err != nil {
		host = targetHost
		port = "443"
		targetHost = net.JoinHostPort(host, port)
		log.Printf("Port missing in CONNECT target '%s', assuming 443: %s", originalTargetHost, targetHost)
	}

	log.Printf("Handling CONNECT for %s", targetHost)

	generatedCert, err := p.GetCertificate(host)
	if err != nil {
		generatedCert, err = p.generateCert(host)
		if err != nil {
			log.Printf("Failed to generate certificate for %s: %v", host, err)
			fmt.Fprintf(clientConn, "HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\n\r\n")
			return
		}
	}

	_, err = fmt.Fprintf(clientConn, "HTTP/1.0 200 Connection established\r\nProxy-agent: GoProxy/1.0\r\n\r\n")
	if err != nil {
		log.Printf("Failed to send '200 Connection established' to client: %v", err)
		return
	}
	log.Printf("Sent '200 Connection established' to client for %s", targetHost)

	tlsClientConfig := &tls.Config{
		Certificates: []tls.Certificate{*generatedCert},
		MinVersion:   tls.VersionTLS12,
	}

	tlsClientConn := tls.Server(clientConn, tlsClientConfig)
	err = tlsClientConn.Handshake()
	if err != nil {
		log.Printf("TLS handshake with client failed for %s: %v", host, err)

		return
	}
	defer func(tlsClientConn *tls.Conn) {
		err = tlsClientConn.Close()
		if err != nil {
			log.Printf("Failed to close TLS connection: %v", err)
		}
	}(tlsClientConn)
	log.Printf("TLS handshake with client successful for %s", host)

	targetServerConn, err := net.DialTimeout("tcp", targetHost, 15*time.Second)
	if err != nil {
		log.Printf("Failed to connect to target server %s: %v", targetHost, err)

		return
	}
	defer func(targetServerConn net.Conn) {
		err = targetServerConn.Close()
		if err != nil {
			log.Printf("Failed to close target server: %v", err)
		}
	}(targetServerConn)
	log.Printf("Established TCP connection to target server %s", targetHost)

	tlsServerConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}
	tlsServerConn := tls.Client(targetServerConn, tlsServerConfig)
	err = tlsServerConn.Handshake()
	if err != nil {
		log.Printf("TLS handshake with target server %s failed: %v", targetHost, err)
		return
	}
	defer func(tlsServerConn *tls.Conn) {
		err = tlsServerConn.Close()
		if err != nil {
			log.Printf("Failed to close TLS connection for %s: %v", host, err)
		}
	}(tlsServerConn)
	log.Printf("TLS handshake with target server %s successful", targetHost)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		bytesCopied, err := io.Copy(tlsServerConn, tlsClientConn)
		if err != nil && err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
			log.Printf("Error copying client to server (%s): %v (%d bytes)", targetHost, err, bytesCopied)
		}

		if err = tlsServerConn.CloseWrite(); err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				log.Printf("Error calling CloseWrite on server connection (%s): %v", targetHost, err)
			}
		}

		log.Printf("Finished client -> server copy for %s (%d bytes)", targetHost, bytesCopied)
	}()

	go func() {
		defer wg.Done()
		bytesCopied, err := io.Copy(tlsClientConn, tlsServerConn)
		if err != nil && err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
			log.Printf("Error copying server to client (%s): %v (%d bytes)", targetHost, err, bytesCopied)
		}

		if err = tlsClientConn.CloseWrite(); err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				log.Printf("Error calling CloseWrite on client connection (%s): %v", targetHost, err)
			}
		}
		log.Printf("Finished server -> client copy for %s (%d bytes)", targetHost, bytesCopied)
	}()

	wg.Wait()
	log.Printf("Data relay finished for %s", targetHost)
}
