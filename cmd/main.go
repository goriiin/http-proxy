package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"github.com/goriiin/go-proxy/internal/proxy"
	"log"
	"net"
)

func main() {
	caCertFile := "ca.crt"
	caKeyFile := "ca.key"
	flag.Parse()

	var err error
	caCert, err := tls.LoadX509KeyPair(caCertFile, caKeyFile)
	if err != nil {
		log.Fatalf("Failed to load CA certificate/key: %v", err)
	}

	caCert.Leaf, err = x509.ParseCertificate(caCert.Certificate[0])
	if err != nil {
		log.Fatalf("Failed to parse CA certificate: %v", err)
	}

	listenAddr := "0.0.0.0:8080"
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", listenAddr, err)
	}
	defer listener.Close()
	log.Printf("Proxy listening on %s", listenAddr)

	myProxy := proxy.New(caCert)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go myProxy.HandleClientRequest(clientConn)
	}
}
