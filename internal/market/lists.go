package market

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const DefaultListID = "favorites"
const localListOwner = "local"
const maxListItems = 2000

type FavoriteList struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Items   []string `json:"items"`
	Default bool     `json:"default"`
}

type principalContextKey struct{}

// WithUser binds ownership to a verified server identity, never request JSON.
func WithUser(r *http.Request, user string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), principalContextKey{}, user))
}
func requestUser(r *http.Request) string {
	if user, ok := r.Context().Value(principalContextKey{}).(string); ok && user != "" {
		return user
	}
	return localListOwner
}
func validListName(name string) bool {
	return strings.TrimSpace(name) != "" && utf8.RuneCountInString(name) <= 80 && !strings.ContainsAny(name, "\x00\r\n")
}

func (s *Store) Lists(key, owner string) ([]FavoriteList, error) {
	lists := []FavoriteList{}
	rows, err := s.db.Query(`SELECT id,name FROM favorite_lists WHERE market=? AND owner=? ORDER BY created,id`, key, owner)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v FavoriteList
		if err = rows.Scan(&v.ID, &v.Name); err != nil {
			rows.Close()
			return nil, err
		}
		v.Items = []string{}
		v.Default = v.ID == DefaultListID
		lists = append(lists, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	found := false
	for _, v := range lists {
		if v.Default {
			found = true
		}
	}
	if !found {
		lists = append([]FavoriteList{{ID: DefaultListID, Name: "Favorites", Items: []string{}, Default: true}}, lists...)
	}
	for n := range lists {
		rows, err = s.db.Query(`SELECT item FROM favorite_list_items WHERE market=? AND owner=? AND list_id=? ORDER BY position,item`, key, owner, lists[n].ID)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			lists[n].Items = append(lists[n].Items, id)
			seen[id] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if lists[n].Default && owner == localListOwner {
			rows, err = s.db.Query(`SELECT item FROM watches WHERE market=? ORDER BY item`, key)
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var id string
				rows.Scan(&id)
				if !seen[id] {
					lists[n].Items = append(lists[n].Items, id)
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return nil, err
			}
		}
	}
	sort.SliceStable(lists, func(i, j int) bool { return lists[i].Default && !lists[j].Default })
	return lists, nil
}
func ensureDefaultList(tx *sql.Tx, key, owner string) error {
	_, err := tx.Exec(`INSERT OR IGNORE INTO favorite_lists VALUES(?,?,?,?,?)`, key, owner, DefaultListID, "Favorites", time.Now().UnixNano())
	if err != nil {
		return err
	}
	if owner == localListOwner {
		_, err = tx.Exec(`INSERT OR IGNORE INTO favorite_list_items(market,owner,list_id,item,position) SELECT ?,?,?,item,COALESCE((SELECT MAX(position) FROM favorite_list_items WHERE market=? AND owner=? AND list_id=?),0)+ROW_NUMBER() OVER(ORDER BY item) FROM watches WHERE market=?`, key, owner, DefaultListID, key, owner, DefaultListID, key)
	}
	return err
}
func (s *Store) SaveList(key, owner, id, name string, create bool) error {
	name = strings.TrimSpace(name)
	if !validID(id) || len(id) > 80 || !validListName(name) || (create && id == DefaultListID) {
		return fmt.Errorf("use a list name of 1–80 characters")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = ensureDefaultList(tx, key, owner); err != nil {
		return err
	}
	var duplicate int
	err = tx.QueryRow(`SELECT count(*) FROM favorite_lists WHERE market=? AND owner=? AND name=? COLLATE NOCASE AND id<>?`, key, owner, name, id).Scan(&duplicate)
	if err != nil {
		return err
	}
	if duplicate > 0 {
		return fmt.Errorf("a list with that name already exists")
	}
	if create {
		var count int
		if err = tx.QueryRow(`SELECT count(*) FROM favorite_lists WHERE market=? AND owner=?`, key, owner).Scan(&count); err != nil {
			return err
		}
		if count >= 128 {
			return fmt.Errorf("the limit is 128 lists")
		}
		_, err = tx.Exec(`INSERT INTO favorite_lists VALUES(?,?,?,?,?)`, key, owner, id, name, time.Now().UnixNano())
	} else {
		var result sql.Result
		result, err = tx.Exec(`UPDATE favorite_lists SET name=? WHERE market=? AND owner=? AND id=?`, name, key, owner, id)
		if err == nil {
			if n, _ := result.RowsAffected(); n == 0 {
				return fmt.Errorf("list not found")
			}
		}
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(); err == nil {
		s.revision.Add(1)
	}
	return err
}
func (s *Store) DeleteList(key, owner, id string) error {
	if id == DefaultListID {
		return fmt.Errorf("the default list can be renamed but cannot be deleted")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`DELETE FROM favorite_lists WHERE market=? AND owner=? AND id=?`, key, owner, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("list not found")
	}
	if _, err = tx.Exec(`DELETE FROM favorite_list_items WHERE market=? AND owner=? AND list_id=?`, key, owner, id); err != nil {
		return err
	}
	if err = tx.Commit(); err == nil {
		s.revision.Add(1)
	}
	return err
}
func (s *Store) ListItems(key, owner string, listIDs, items []string, mode string) error {
	if len(items) < 1 || len(items) > maxListItems || len(listIDs) > 128 || (mode != "add" && mode != "remove" && mode != "assign") || (mode == "assign" && len(items) != 1) {
		return fmt.Errorf("choose 1–2000 items and valid lists")
	}
	seen := map[string]bool{}
	for _, id := range items {
		if !validID(id) || seen[id] {
			return fmt.Errorf("invalid or repeated item")
		}
		seen[id] = true
	}
	selected := map[string]bool{}
	for _, id := range listIDs {
		if !validID(id) || selected[id] {
			return fmt.Errorf("invalid or repeated list")
		}
		selected[id] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = ensureDefaultList(tx, key, owner); err != nil {
		return err
	}
	for _, id := range listIDs {
		var exists int
		if err = tx.QueryRow(`SELECT count(*) FROM favorite_lists WHERE market=? AND owner=? AND id=?`, key, owner, id).Scan(&exists); err != nil {
			return err
		}
		if exists != 1 {
			return fmt.Errorf("list not found")
		}
	}
	if mode == "assign" {
		args := []any{key, owner, items[0]}
		query := `DELETE FROM favorite_list_items WHERE market=? AND owner=? AND item=?`
		if len(listIDs) > 0 {
			query += ` AND list_id NOT IN (` + strings.TrimSuffix(strings.Repeat("?,", len(listIDs)), ",") + `)`
			for _, id := range listIDs {
				args = append(args, id)
			}
		}
		if _, err = tx.Exec(query, args...); err != nil {
			return err
		}
		if owner == localListOwner {
			if _, err = tx.Exec(`DELETE FROM watches WHERE market=? AND item=?`, key, items[0]); err != nil {
				return err
			}
		}
	}
	for _, list := range listIDs {
		var position int
		if err = tx.QueryRow(`SELECT COALESCE(MAX(position),0) FROM favorite_list_items WHERE market=? AND owner=? AND list_id=?`, key, owner, list).Scan(&position); err != nil {
			return err
		}
		for _, item := range items {
			if mode == "remove" {
				_, err = tx.Exec(`DELETE FROM favorite_list_items WHERE market=? AND owner=? AND list_id=? AND item=?`, key, owner, list, item)
			} else {
				position++
				_, err = tx.Exec(`INSERT OR IGNORE INTO favorite_list_items VALUES(?,?,?,?,?)`, key, owner, list, item, position)
			}
			if err != nil {
				return err
			}
			if list == DefaultListID && owner == localListOwner {
				if mode == "remove" {
					_, err = tx.Exec(`DELETE FROM watches WHERE market=? AND item=?`, key, item)
				} else {
					_, err = tx.Exec(`INSERT OR IGNORE INTO watches VALUES(?,?)`, key, item)
				}
				if err != nil {
					return err
				}
			}
		}
		var count int
		if err = tx.QueryRow(`SELECT count(*) FROM favorite_list_items WHERE market=? AND owner=? AND list_id=?`, key, owner, list).Scan(&count); err != nil {
			return err
		}
		if count > maxListItems {
			return fmt.Errorf("a list can contain at most 2000 items")
		}
	}
	if err = tx.Commit(); err == nil {
		s.revision.Add(1)
	}
	return err
}
func (s *Store) MoveListItem(key, owner, list, item, before string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = ensureDefaultList(tx, key, owner); err != nil {
		return err
	}
	rows, err := tx.Query(`SELECT item FROM favorite_list_items WHERE market=? AND owner=? AND list_id=? ORDER BY position,item`, key, owner, list)
	if err != nil {
		return err
	}
	ids := []string{}
	found, target := false, before == ""
	for rows.Next() {
		var id string
		rows.Scan(&id)
		if id == item {
			found = true
		}
		if id == before {
			target = true
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !found || !target {
		return fmt.Errorf("list item not found")
	}
	if item == before {
		return nil
	}
	order := []string{}
	for _, id := range ids {
		if id == item {
			continue
		}
		if id == before {
			order = append(order, item)
		}
		order = append(order, id)
	}
	if before == "" {
		order = append(order, item)
	}
	for position, id := range order {
		if _, err = tx.Exec(`UPDATE favorite_list_items SET position=? WHERE market=? AND owner=? AND list_id=? AND item=?`, position+1, key, owner, list, id); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err == nil {
		s.revision.Add(1)
	}
	return err
}
