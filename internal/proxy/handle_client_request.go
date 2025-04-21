package proxy

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"github.com/goriiin/go-proxy/internal/domain"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
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

		parsedReq := parseHTTPRequest(req) // helper‑функции ниже
		parsedResp := parseHTTPResponse(resp)
		id, _ := p.store.Save(parsedReq, parsedResp)
		log.Printf("Saved request with id=%d", id)

		err = resp.Write(clientConn)
		if err != nil {
			log.Printf("Failed to write response back to client for %s: %v", target, err)
		} else {
			log.Printf("Successfully relayed response for %s %s", req.Method, target)
		}
	}
}

func flattener(v url.Values) map[string]interface{} {
	out := make(map[string]interface{}, len(v))
	for k, arr := range v {
		if len(arr) == 1 {
			out[k] = arr[0]
		} else {
			out[k] = arr
		}
	}
	return out
}

// ----------- запрос ---------------------------------------------------------

func parseHTTPRequest(r *http.Request) domain.ParsedRequest {
	// GET‑параметры
	getParams := flattener(r.URL.Query())

	// Cookie‑заголовок в map
	cookies := map[string]string{}
	for _, c := range r.Cookies() {
		cookies[c.Name] = c.Value
	}

	// Заголовки
	hdrs := map[string]string{}
	for k, v := range r.Header {
		hdrs[k] = strings.Join(v, ", ")
	}

	// Тело
	var bodyCopy strings.Builder
	if r.Body != nil {
		raw, _ := io.ReadAll(r.Body)
		bodyCopy.Write(raw)
		// вернём тело, чтобы последующая отправка сохранилась
		r.Body = io.NopCloser(strings.NewReader(bodyCopy.String()))
	}

	// POST‑/PUT‑параметры (если это form)
	postParams := map[string]interface{}{}
	if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct == "application/x-www-form-urlencoded" {
		_ = r.ParseForm()
		postParams = flattener(r.PostForm)
	}

	return domain.ParsedRequest{
		Method:     r.Method,
		Path:       r.URL.Path,
		GetParams:  getParams,
		PostParams: postParams,
		Headers:    hdrs,
		Cookies:    cookies,
		Body:       bodyCopy.String(),
		Host:       r.Host,
		RawRequest: rawRequestDump(r, bodyCopy.String()),
	}
}

// rawRequestDump строит текст вида:
//
//	GET /path?a=1 HTTP/1.1\r\nHdr: v\r\n\r\nBODY…
func rawRequestDump(r *http.Request, body string) string {
	var b strings.Builder
	b.WriteString(r.Method + " " + r.URL.RequestURI() + " " + r.Proto + "\r\n")
	for k, v := range r.Header {
		for _, vv := range v {
			b.WriteString(k + ": " + vv + "\r\n")
		}
	}
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.String()
}

// ----------- ответ ----------------------------------------------------------

func parseHTTPResponse(resp *http.Response) domain.ParsedResponse {
	// Декодируем gzip, чтобы в БД хранился настоящий html/json
	var reader io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		if gr, err := gzip.NewReader(resp.Body); err == nil {
			reader = gr
			defer gr.Close()
		}
	}

	raw, _ := io.ReadAll(reader)
	resp.Body = io.NopCloser(strings.NewReader(string(raw))) // возвращаем, чтобы прокси мог отправить клиенту

	// Заголовки
	hdrs := map[string]string{}
	for k, v := range resp.Header {
		hdrs[k] = strings.Join(v, ", ")
	}

	return domain.ParsedResponse{
		Code:    resp.StatusCode,
		Message: resp.Status,
		Headers: hdrs,
		Body:    string(raw),
	}
}
