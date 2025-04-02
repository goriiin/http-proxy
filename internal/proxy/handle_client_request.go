package proxy

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

func (p *Proxy) HandleClientRequest(clientConn net.Conn) {
	defer func(clientConn net.Conn) {
		err := clientConn.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}(clientConn)

	reader := bufio.NewReader(clientConn)
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			log.Printf("Failed to read request line: %v", err)
		} else {
			return
		}
	}

	requestLine = strings.TrimSpace(requestLine)
	if requestLine == "" {
		log.Println("Empty request line")
		return
	}
	log.Printf("Received request: %s", requestLine)

	parts := strings.Split(requestLine, " ")
	if len(parts) != 3 {
		log.Printf("Invalid request line format: %s", requestLine)
		fmt.Fprintf(clientConn, "HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\n")

		return
	}

	method, target, _ := parts[0], parts[1], parts[2]
	if method == http.MethodConnect {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				log.Printf("Error reading headers for CONNECT: %v", err)
				return
			}
			if line == "\r\n" {
				break
			}
		}
		p.handleHTTPSConnect(clientConn, target)
	} else {
		log.Printf("Handling non-CONNECT (%s) request for %s", method, target)

		rebuiltRequestReader := io.MultiReader(strings.NewReader(requestLine+"\r\n"), reader)
		req, err := http.ReadRequest(bufio.NewReader(rebuiltRequestReader)) // Use new bufio reader on combined stream
		if err != nil {
			log.Printf("Failed to read full HTTP request for %s: %v", target, err)
			fmt.Fprintf(clientConn, "HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\nCould not parse request.\r\n")
			return
		}

		for name, headers := range req.Header {
			for _, h := range headers {
				log.Printf("  Header: %v: %v", name, h)
			}
		}

		req.Header.Del("Proxy-Connection")
		req.Header.Del("Proxy-Authorization")
		if prior, ok := req.Header["X-Forwarded-For"]; ok {
			req.Header.Set("X-Forwarded-For", strings.Join(prior, ", ")+", "+clientConn.RemoteAddr().String())
		} else {
			req.Header.Set("X-Forwarded-For", clientConn.RemoteAddr().String())
		}

		req.RequestURI = ""

		if req.Host == "" && req.URL != nil {
			req.Host = req.URL.Host
		}
		if req.Host == "" {
			log.Printf("Could not determine host for request: %s", target)
			fmt.Fprintf(clientConn, "HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\nInvalid target host.\r\n")
			return
		}
		log.Printf("Forwarding %s request to host: %s, URL: %s", req.Method, req.Host, req.URL.String())

		transport := &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			Proxy:                 nil,
		}

		resp, err := transport.RoundTrip(req)
		if err != nil {
			log.Printf("Failed to forward request to %s: %v", req.Host, err)
			errMsg := fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nProxy failed to connect to target server: %v\r\n", err)
			fmt.Fprint(clientConn, errMsg)
			return
		}
		defer resp.Body.Close()

		log.Printf("Received response %s for %s %s", resp.Status, req.Method, target)

		err = resp.Write(clientConn)
		if err != nil {
			log.Printf("Failed to write response back to client for %s: %v", target, err)
		} else {
			log.Printf("Successfully relayed response for %s %s", req.Method, target)
		}
	}
}
