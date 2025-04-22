package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/goriiin/go-proxy/internal/scanner"
	"github.com/goriiin/go-proxy/internal/store"
)

func Start(s *store.Store, p *scanner.Scanner) {
	r := mux.NewRouter()

	r.HandleFunc("/requests", func(w http.ResponseWriter, r *http.Request) {
		list, _ := s.List()
		_ = json.NewEncoder(w).Encode(list)
	}).Methods(http.MethodGet)

	r.HandleFunc("/requests/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseUint(mux.Vars(r)["id"], 10, 64)
		item, _ := s.Get(id)
		_ = json.NewEncoder(w).Encode(item)
	}).Methods(http.MethodGet)

	r.HandleFunc("/repeat/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseUint(mux.Vars(r)["id"], 10, 64)
		res, _ := p.Repeat(id)
		_ = json.NewEncoder(w).Encode(res)
	}).Methods(http.MethodPost)

	r.HandleFunc("/scan/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseUint(mux.Vars(r)["id"], 10, 64)
		res, _ := p.DirBuster(id)
		_ = json.NewEncoder(w).Encode(res)
	}).Methods(http.MethodPost)

	http.ListenAndServe(":8000", r)
}
