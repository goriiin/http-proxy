package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

type Proxy struct{}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Получен запрос на", r.Host)
	fmt.Printf("\t Входящие заголовки: %v\n", r.Header)
	fmt.Printf("\t URL запроса: %v\n", r.URL)
	fmt.Printf("\t Хост: %v\n", r.Host)
	fmt.Printf("\t User-Agent: %v\n", r.UserAgent())
	fmt.Printf("\t Метод: %v\n", r.Method)

	r.Header.Del("Proxy-Connection")

	log.Println("Отправляем запрос на целевой сервер:")
	fmt.Printf("\t Метод: %v, URL: %v\n", r.Method, r.URL.String())
	fmt.Printf("\t Заголовки, отправляемые на сервер: %v\n", r.Header)
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)

		return
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(resp.Body)

	log.Println("Получен ответ от целевого сервера")
	for key, values := range resp.Header {
		fmt.Printf("\t %v: %v\n", key, values)
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	fmt.Printf("\t Статус: %v\n", resp.Status)

	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("[ ERROR ] io.Copy %v \n", err)
		return
	}
}

func main() {
	proxy := &Proxy{}
	log.Println("HTTP-прокси запущен на порту 8080")
	log.Fatal(http.ListenAndServe(":8080", proxy))
}
