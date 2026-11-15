package market

import (
	"fmt"
	"sort"
	"time"
)

func validTrade(t Trade) bool {
	return validID(t.ID) && validID(t.Item) && (t.Side == "buy" || t.Side == "sell") && validQuantity(t.Quantity) && validMoney(t.Total) && t.At >= 946684800 && t.At <= time.Now().Add(10*time.Minute).Unix()
}
func (s *Store) mutateTrade(key string, t Trade, action string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	trades, err := s.Trades(key)
	if err != nil {
		return err
	}
	candidate := make([]Trade, 0, len(trades)+1)
	found := false
	for _, old := range trades {
		if old.ID == t.ID {
			found = true
			if action == "add" {
				return fmt.Errorf("this transaction is already recorded")
			}
			if action == "edit" {
				if old.Item != t.Item {
					return fmt.Errorf("a correction must keep the original item")
				}
				candidate = append(candidate, t)
			}
		} else {
			candidate = append(candidate, old)
		}
	}
	if action != "add" && !found {
		return fmt.Errorf("transaction no longer exists")
	}
	if action == "add" {
		candidate = append(candidate, t)
	}
	sort.SliceStable(candidate, func(i, j int) bool { return candidate[i].At < candidate[j].At })
	balances := map[string]int64{}
	costs := map[string]int64{}
	for _, v := range candidate {
		if v.Side == "buy" {
			balances[v.Item] += v.Quantity
			costs[v.Item] = safeAdd(costs[v.Item], v.Total)
			if balances[v.Item] > 1_000_000_000_000 || costs[v.Item] >= maxCost {
				return fmt.Errorf("ledger limit exceeded")
			}
		} else {
			balances[v.Item] -= v.Quantity
			if balances[v.Item] < 0 {
				return fmt.Errorf("this would sell more than the recorded holding; correct its purchases first")
			}
		}
	}
	switch action {
	case "add":
		_, err = s.db.Exec(`INSERT INTO trades VALUES(?,?,?,?,?,?,?)`, key, t.ID, t.Item, t.Side, t.Quantity, t.Total, t.At)
	case "edit":
		_, err = s.db.Exec(`UPDATE trades SET side=?,quantity=?,total=?,at=? WHERE market=? AND id=?`, t.Side, t.Quantity, t.Total, t.At, key, t.ID)
	case "remove":
		_, err = s.db.Exec(`DELETE FROM trades WHERE market=? AND id=?`, key, t.ID)
	}
	return err
}
func (s *Store) EditTrade(key string, t Trade) error {
	if !validTrade(t) {
		return fmt.Errorf("invalid transaction")
	}
	return s.mutateTrade(key, t, "edit")
}
func (s *Store) RemoveTrade(key, id string) error {
	if !validID(id) {
		return fmt.Errorf("invalid transaction ID")
	}
	return s.mutateTrade(key, Trade{ID: id}, "remove")
}
