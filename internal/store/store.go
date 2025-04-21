package store

import (
	"context"
	"fmt"
	"github.com/goriiin/go-proxy/internal/domain"
	"time"

	tarantool "github.com/tarantool/go-tarantool/v2"
)

type Store struct{ conn *tarantool.Connection }

func New(addr string) (*Store, error) {
	dialer := tarantool.NetDialer{
		Address: addr,
		User:    "guest", // ← поле User живёт в dialer
	}
	opts := tarantool.Opts{Timeout: 3 * time.Second}

	conn, err := tarantool.Connect(context.Background(), dialer, opts)
	if err != nil {
		return nil, err
	}
	return &Store{conn: conn}, nil
}

// --- сохраняем -------------------------------------------------------------

func (s *Store) Save(req domain.ParsedRequest, resp domain.ParsedResponse) (uint64, error) {
	tuple := []interface{}{
		nil, // auto‑inc id (sequence)
		req.Host,
		req.Method,
		req.Path,
		map[string]interface{}{"request": req, "response": resp},
		uint64(time.Now().Unix()),
	}

	// v2 — только через Do(...)
	data, err := s.conn.Do(
		tarantool.NewInsertRequest("requests").Tuple(tuple),
	).Get()
	if err != nil {
		return 0, err
	}

	// Get() вернёт [][]interface{}

	id := data[0].(uint64)
	return id, nil
}

// --- выборки ---------------------------------------------------------------

func (s *Store) Get(id uint64) (map[string]interface{}, error) {
	data, err := s.conn.Do(
		tarantool.NewSelectRequest("requests").
			Index("primary").
			Iterator(tarantool.IterEq).
			Key([]interface{}{id}).
			Limit(1),
	).Get()
	if err != nil {
		return nil, err
	}
	rows := data
	if len(rows) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return rows[0].(map[string]interface{}), nil
}

func (s *Store) List() ([]map[string]interface{}, error) {
	raw, err := s.conn.Do(
		tarantool.NewSelectRequest("requests").
			Iterator(tarantool.IterAll),
	).Get()
	if err != nil {
		return nil, err
	}
	arr := raw
	out := make([]map[string]interface{}, len(arr))
	for i, v := range arr {
		out[i] = v.(map[string]interface{})
	}
	return out, nil
}
