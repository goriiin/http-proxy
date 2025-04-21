package scanner

import (
	"bufio"
	"net/http"
	"os"
	"strings"

	"github.com/goriiin/go-proxy/internal/store"
)

type Scanner struct {
	s     *store.Store
	words []string
}

func New(s *store.Store, wordlist string) (*Scanner, error) {
	fd, err := os.Open(wordlist)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	sc := bufio.NewScanner(fd)
	var w []string
	for sc.Scan() {
		w = append(w, strings.TrimSpace(sc.Text()))
	}
	return &Scanner{s: s, words: w}, nil
}

func (sc *Scanner) Repeat(id uint64) (*http.Response, error) {
	item, _ := sc.s.Get(id)
	reqMap := item["data"].(map[string]interface{})["request"].(map[string]interface{})
	raw := reqMap["raw_request"].(string)

	// отправляем «сырым» текстом через локальный прокси
	resp, err := http.DefaultTransport.RoundTrip(parseRaw(raw))
	return resp, err
}

func (sc *Scanner) DirBuster(id uint64) ([]map[string]interface{}, error) {
	item, _ := sc.s.Get(id)
	reqMap := item["data"].(map[string]interface{})["request"].(map[string]interface{})
	host := item["host"].(string)
	origPath := reqMap["path"].(string)

	var findings []map[string]interface{}
	for _, w := range sc.words {
		p := "/" + strings.TrimLeft(w, "/")
		req, _ := http.NewRequest(reqMap["method"].(string), "http://"+host+p, nil)
		resp, err := http.DefaultTransport.RoundTrip(req)
		if err == nil && resp.StatusCode != http.StatusNotFound {
			findings = append(findings, map[string]interface{}{
				"path":      p,
				"status":    resp.StatusCode,
				"orig_path": origPath,
			})
		}
	}
	return findings, nil
}

// parseRaw превращает «сырой» HTTP‑текст в *http.Request,
// достаточный для http.Transport.RoundTrip().
func parseRaw(raw string) *http.Request {
	// 1) распарсить как обычный HTTP‑запрос
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		return nil // лучше обработать ошибку наверху — тут кратко
	}

	// 2) Сформировать полноценный URL (scheme + host):
	if req.URL.Scheme == "" {
		req.URL.Scheme = "http" // по умолчанию; при желании можно хранить Scheme отдельно
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}

	// 3) Для RoundTrip() RequestURI должен быть пустым.
	req.RequestURI = ""

	return req
}
