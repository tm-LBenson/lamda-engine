package market

import "encoding/json"

func (s *Store) Items(key string) ([]Item, error) {
	rows, err := s.db.Query(`SELECT body FROM items WHERE market=? ORDER BY id`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var item Item
		if err = json.Unmarshal(b, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
