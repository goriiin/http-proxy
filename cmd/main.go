// cmd/main.go
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"log"
	"net"
	"os"

	"github.com/goriiin/go-proxy/internal/api"
	"github.com/goriiin/go-proxy/internal/proxy"
	"github.com/goriiin/go-proxy/internal/scanner"
	"github.com/goriiin/go-proxy/internal/store"
)

func main() {
	// ---- флаги/параметры ----------------------------------------------------
	caCertPath := flag.String("ca-cert", "ca.crt", "CA certificate file")
	caKeyPath := flag.String("ca-key", "ca.key", "CA private key file")
	proxyAddr := flag.String("proxy-addr", "0.0.0.0:8080", "Address for the HTTP‑proxy")
	apiAddr := flag.String("api-addr", ":8000", "Address for the REST API")
	wordlist := flag.String("wordlist", "db/dicc.txt", "Wordlist for DirBuster scan")
	flag.Parse()

	// ---- CA сертификат ------------------------------------------------------
	caPair, err := tls.LoadX509KeyPair(*caCertPath, *caKeyPath)
	if err != nil {
		log.Fatalf("cannot load CA pair: %v", err)
	}
	caPair.Leaf, err = x509.ParseCertificate(caPair.Certificate[0])
	if err != nil {
		log.Fatalf("cannot parse CA cert: %v", err)
	}

	// ---- Tarantool ----------------------------------------------------------
	tntURI := os.Getenv("TARANTOOL_ADDR")
	if tntURI == "" {
		tntURI = "tarantool:3301" // для docker‑compose по умолчанию
	}
	st, err := store.New(tntURI) // uses go‑tarantool v1
	if err != nil {
		log.Fatalf("tarantool connection error: %v", err)
	}

	// ---- сканер (DirBuster + повтор запросов) ------------------------------
	sc, err := scanner.New(st, *wordlist)
	if err != nil {
		log.Fatalf("cannot init scanner: %v", err)
	}

	// ---- REST‑API -----------------------------------------------------------
	go api.Start(st, sc) // неблокирующий

	// ---- сам HTTP/HTTPS‑прокси ---------------------------------------------
	pr := proxy.New(caPair, st) // наш расширенный прокси с БД

	listener, err := net.Listen("tcp", *proxyAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", *proxyAddr, err)
	}
	defer listener.Close()
	log.Printf("Proxy listening on %s; API on %s", *proxyAddr, *apiAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go pr.HandleClientRequest(conn)
	}
}
