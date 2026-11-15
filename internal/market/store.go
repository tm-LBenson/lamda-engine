package market

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/spellbook"
	"io"
	"math/big"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func missingItemName(i Item) bool {
	if i.Name == "" || i.Name == i.ID {
		return true
	}
	parts := strings.Split(i.ID, ":")
	return len(parts) > 1 && (parts[0] == "item" || parts[0] == "browse") && i.Name == parts[1]
}

type Store struct {
	publisher *Publisher
	icons     *spellbook.IconCache
	db        *sql.DB
	client    string
	mu        sync.Mutex
	transfer  *Assembler
	captureMu sync.Mutex
	capture   CaptureStatus
	revision  atomic.Uint64
}

func Open(path, client string) (*Store, error) {
	if client != "forever" && client != "retail" {
		return nil, fmt.Errorf("invalid client")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	var schema int
	if err = db.QueryRow(`PRAGMA user_version`).Scan(&schema); err != nil || schema > 1 {
		db.Close()
		return nil, fmt.Errorf("unsupported market database schema: %d (%v)", schema, err)
	}
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;
 CREATE TABLE IF NOT EXISTS events (market TEXT,id TEXT,hash TEXT,body BLOB,PRIMARY KEY(market,id));
 CREATE TABLE IF NOT EXISTS scans (market TEXT,id TEXT,at INTEGER,pages INTEGER,coverage TEXT,complete INTEGER DEFAULT 0,PRIMARY KEY(market,id));
 CREATE TABLE IF NOT EXISTS pages (market TEXT,scan TEXT,page INTEGER,body BLOB,PRIMARY KEY(market,scan,page));
 CREATE TABLE IF NOT EXISTS items (market TEXT,id TEXT,body BLOB,at INTEGER,PRIMARY KEY(market,id));
 CREATE TABLE IF NOT EXISTS quotes (market TEXT,scan TEXT,item TEXT,at INTEGER,price REAL,quantity INTEGER,coverage TEXT,lots BLOB,PRIMARY KEY(market,scan,item));
 CREATE INDEX IF NOT EXISTS quote_history ON quotes(market,item,at);
 CREATE TABLE IF NOT EXISTS documents (market TEXT,kind TEXT,id TEXT,at INTEGER,body BLOB,PRIMARY KEY(market,kind,id));
 CREATE TABLE IF NOT EXISTS watches (market TEXT,item TEXT,PRIMARY KEY(market,item));
 CREATE TABLE IF NOT EXISTS favorite_lists (market TEXT,owner TEXT,id TEXT,name TEXT,created INTEGER,PRIMARY KEY(market,owner,id));
 CREATE TABLE IF NOT EXISTS favorite_list_items (market TEXT,owner TEXT,list_id TEXT,item TEXT,position INTEGER,PRIMARY KEY(market,owner,list_id,item));
 CREATE TABLE IF NOT EXISTS trades (market TEXT,id TEXT,item TEXT,side TEXT,quantity INTEGER,total INTEGER,at INTEGER,PRIMARY KEY(market,id));
 CREATE INDEX IF NOT EXISTS trade_item ON trades(market,item,at);
 PRAGMA user_version=1;`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, client: client, transfer: NewAssembler(), icons: spellbook.NewItemIconCache(filepath.Join(filepath.Dir(path), "icons"))}, nil
}
func (s *Store) Close() error {
	if s.publisher != nil {
		s.publisher.Close()
	}
	return s.db.Close()
}
func strictJSON(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var x any
	if d.Decode(&x) != io.EOF {
		return fmt.Errorf("one JSON object required")
	}
	return nil
}
func encoded(v any) []byte { b, _ := json.Marshal(v); return b }
func (s *Store) Ingest(data []byte, now time.Time) error {
	if len(data) > MaxEventBytes {
		return fmt.Errorf("event exceeds size limit")
	}
	var e Event
	if err := strictJSON(data, &e); err != nil {
		return err
	}
	if err := e.Validate(now); err != nil {
		return err
	}
	if e.Market.Client != s.client {
		return fmt.Errorf("wrong client")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	key := e.Market.Key()
	canonical := encoded(e)
	sum := sha256.Sum256(canonical)
	hash := hex.EncodeToString(sum[:])
	var previous string
	err = tx.QueryRow(`SELECT hash FROM events WHERE market=? AND id=?`, key, e.ID).Scan(&previous)
	if err == nil {
		if previous != hash {
			return fmt.Errorf("event ID reused with different content")
		}
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO events VALUES(?,?,?,?)`, key, e.ID, hash, canonical); err != nil {
		return err
	}
	for _, i := range e.Items {
		// Cache misses are not new evidence about an item's name or binding.
		if missingItemName(i) || !i.BindingKnown || i.ClassID == nil || i.SubclassID == nil || i.ClassName == "" || i.SubclassName == "" {
			var body []byte
			var prior Item
			if tx.QueryRow(`SELECT body FROM items WHERE market=? AND id=?`, key, i.ID).Scan(&body) == nil && json.Unmarshal(body, &prior) == nil {
				if missingItemName(i) {
					i.Name = prior.Name
				}
				if !i.BindingKnown && prior.BindingKnown {
					i.BindingKnown, i.Transferable = true, prior.Transferable
				}
				if i.ClassID == nil {
					i.ClassID, i.SubclassID = prior.ClassID, prior.SubclassID
					i.ClassName, i.SubclassName = prior.ClassName, prior.SubclassName
				} else if prior.ClassID != nil && *i.ClassID == *prior.ClassID {
					if i.ClassName == "" {
						i.ClassName = prior.ClassName
					}
					if i.SubclassID == nil {
						i.SubclassID, i.SubclassName = prior.SubclassID, prior.SubclassName
					} else if prior.SubclassID != nil && *i.SubclassID == *prior.SubclassID && i.SubclassName == "" {
						i.SubclassName = prior.SubclassName
					}
				}
			}
		}
		if _, err = tx.Exec(`INSERT INTO items VALUES(?,?,?,?) ON CONFLICT(market,id) DO UPDATE SET body=excluded.body,at=excluded.at WHERE excluded.at>=items.at`, key, i.ID, encoded(i), e.At); err != nil {
			return err
		}
	}
	save := func(kind, id string, v any) error {
		_, x := tx.Exec(`INSERT INTO documents VALUES(?,?,?,?,?) ON CONFLICT(market,kind,id) DO UPDATE SET at=excluded.at,body=excluded.body WHERE excluded.at>=documents.at`, key, kind, id, e.At, encoded(v))
		return x
	}
	for _, v := range e.Vendors {
		if err = save("vendor", v.ID, v); err != nil {
			return err
		}
	}
	for _, r := range e.Recipes {
		if err = save("recipe", r.ID, r); err != nil {
			return err
		}
	}
	if c := e.Character; c != nil {
		// Closing a profession window must not erase the last observed recipe list.
		var old []byte
		var prior Character
		if tx.QueryRow(`SELECT body FROM documents WHERE market=? AND kind='character' AND id=?`, key, c.ID).Scan(&old) == nil {
			json.Unmarshal(old, &prior)
			if c.Surname == "" {
				c.Surname = prior.Surname
			}
			if c.Class == "" {
				c.Class = prior.Class
			}
		}
		if !c.RecipesObserved {
			c.Known = prior.Known
			c.KnownByProfession = prior.KnownByProfession
			c.RecipeTimes = prior.RecipeTimes
		} else if c.KnownByProfession != nil {
			if c.RecipeTimes == nil {
				c.RecipeTimes = map[string]int64{}
			}
			active := map[string]bool{}
			for _, p := range c.Professions {
				active[p.ID] = true
			}
			for id, known := range prior.KnownByProfession {
				if active[id] {
					if _, ok := c.KnownByProfession[id]; !ok {
						c.KnownByProfession[id] = known
						c.RecipeTimes[id] = prior.RecipeTimes[id]
					}
				}
			}
			set := map[string]bool{}
			for id, known := range c.KnownByProfession {
				if !active[id] {
					delete(c.KnownByProfession, id)
					delete(c.RecipeTimes, id)
					continue
				}
				for _, recipe := range known {
					set[recipe] = true
				}
			}
			c.Known = []string{}
			for id := range set {
				c.Known = append(c.Known, id)
			}
			sort.Strings(c.Known)
		}
		c.At = e.At
		if err = save("character", c.ID, c); err != nil {
			return err
		}
	}
	if e.Kind == "auction" {
		if _, err = tx.Exec(`INSERT INTO scans(market,id,at,pages,coverage) VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING`, key, e.Scan, e.At, e.Pages, e.Coverage); err != nil {
			return err
		}
		var at int64
		var pages, complete int
		var coverage string
		if err = tx.QueryRow(`SELECT at,pages,coverage,complete FROM scans WHERE market=? AND id=?`, key, e.Scan).Scan(&at, &pages, &coverage, &complete); err != nil {
			return err
		}
		if at != e.At || pages != e.Pages || coverage != e.Coverage {
			return fmt.Errorf("scan metadata conflict")
		}
		var old []byte
		err = tx.QueryRow(`SELECT body FROM pages WHERE market=? AND scan=? AND page=?`, key, e.Scan, e.Page).Scan(&old)
		if err == nil && !bytes.Equal(old, encoded(e.Lots)) {
			return fmt.Errorf("scan page conflict")
		}
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO pages VALUES(?,?,?,?) ON CONFLICT DO NOTHING`, key, e.Scan, e.Page, encoded(e.Lots)); err != nil {
			return err
		}
		var count int
		if err = tx.QueryRow(`SELECT count(*) FROM pages WHERE market=? AND scan=?`, key, e.Scan).Scan(&count); err != nil {
			return err
		}
		if count == pages && complete == 0 {
			rows, x := tx.Query(`SELECT body FROM pages WHERE market=? AND scan=? ORDER BY page`, key, e.Scan)
			if x != nil {
				return x
			}
			byItem := map[string][]Lot{}
			total := 0
			for rows.Next() {
				var b []byte
				if x = rows.Scan(&b); x != nil {
					rows.Close()
					return x
				}
				var lots []Lot
				if x = json.Unmarshal(b, &lots); x != nil {
					rows.Close()
					return x
				}
				total += len(lots)
				if total > 200000 {
					rows.Close()
					return fmt.Errorf("scan exceeds 200000 price levels")
				}
				for _, l := range lots {
					byItem[l.Item] = append(byItem[l.Item], l)
				}
			}
			x = rows.Err()
			rows.Close()
			if x != nil {
				return x
			}
			for id, lots := range byItem {
				var qty int64
				price := float64(lots[0].Price) / float64(lots[0].Quantity)
				for _, l := range lots {
					if n := l.Quantity * l.Count; n > 1_000_000_000_000 || qty > 1_000_000_000_000-n {
						return fmt.Errorf("auction quantity exceeds supported bound")
					}
					qty += l.Quantity * l.Count
					if p := float64(l.Price) / float64(l.Quantity); p < price {
						price = p
					}
				}
				if _, err = tx.Exec(`INSERT INTO quotes VALUES(?,?,?,?,?,?,?,?)`, key, e.Scan, id, e.At, price, qty, e.Coverage, encoded(lots)); err != nil {
					return err
				}
			}
			if _, err = tx.Exec(`UPDATE scans SET complete=1 WHERE market=? AND id=?`, key, e.Scan); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.revision.Add(1)
	return nil
}
func (s *Store) Markets() ([]Identity, error) {
	rows, err := s.db.Query(`SELECT DISTINCT market FROM events ORDER BY market`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Identity{}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return nil, err
		}
		// Keys have four validated components and cannot contain '/'.
		parts := splitKey(key)
		out = append(out, Identity{parts[0], parts[1], parts[2], parts[3]})
	}
	return out, rows.Err()
}
func splitKey(k string) []string {
	var p []string
	start := 0
	for i, c := range k {
		if c == '/' {
			p = append(p, k[start:i])
			start = i + 1
		}
	}
	return append(p, k[start:])
}
func (s *Store) documents(key, kind string, v any) error {
	rows, err := s.db.Query(`SELECT body FROM documents WHERE market=? AND kind=? ORDER BY id`, key, kind)
	if err != nil {
		return err
	}
	defer rows.Close()
	raw := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return err
		}
		raw = append(raw, b)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return json.Unmarshal(encoded(raw), v)
}
func (s *Store) Settings(key string) Settings {
	v := DefaultSettings()
	var b []byte
	if s.db.QueryRow(`SELECT body FROM documents WHERE market=? AND kind='settings' AND id='user'`, key).Scan(&b) == nil {
		json.Unmarshal(b, &v)
	}
	return v
}
func (s *Store) SaveSettings(key string, v Settings) error {
	if v.Batch == 0 {
		v.Batch = 1
	}
	if v.Batch < 1 || v.Batch > 10000 || !validMoney(v.TransferCost) {
		return fmt.Errorf("invalid batch or transfer cost")
	}
	if v.CutBPS < 0 || v.CutBPS > 10000 || !validMoney(v.LostDeposit) || v.MaxAgeHours < 1 || v.MaxAgeHours > 8760 || (v.Mode != "character" && v.Mode != "characters" && v.Mode != "theory") || len(v.Theory) > 40 {
		return fmt.Errorf("invalid settings")
	}
	for id, r := range v.Theory {
		if !validID(id) || r < 0 || r > 10000 {
			return fmt.Errorf("invalid profession rank")
		}
	}
	_, err := s.db.Exec(`INSERT INTO documents VALUES(?,'settings','user',?,?) ON CONFLICT DO UPDATE SET at=excluded.at,body=excluded.body`, key, time.Now().Unix(), encoded(v))
	return err
}
func (s *Store) Watch(key, id string, on bool) error {
	mode := "remove"
	if on {
		mode = "add"
	}
	return s.ListItems(key, localListOwner, []string{DefaultListID}, []string{id}, mode)
}
func (s *Store) Trade(key string, t Trade) error {
	if !validTrade(t) {
		return fmt.Errorf("invalid transaction")
	}
	return s.mutateTrade(key, t, "add")
}
func (s *Store) Trades(key string) ([]Trade, error) {
	rows, err := s.db.Query(`SELECT id,item,side,quantity,total,at FROM trades WHERE market=? ORDER BY at,rowid`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Trade{}
	for rows.Next() {
		var t Trade
		if err = rows.Scan(&t.ID, &t.Item, &t.Side, &t.Quantity, &t.Total, &t.At); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Store) Cards(key string, since int64) ([]Card, error) {
	return s.cardsFor(s, key, since)
}

// cardsFor combines shared observations with only this profile's ledger.
func (s *Store) cardsFor(profile *Store, key string, since int64) ([]Card, error) {
	rows, err := s.db.Query(`SELECT q.item,q.at,q.price,q.quantity,q.coverage,i.body FROM quotes q LEFT JOIN items i ON i.market=q.market AND i.id=q.item WHERE q.market=? AND (q.at>=? OR q.at=(SELECT MAX(z.at) FROM quotes z WHERE z.market=q.market AND z.item=q.item)) ORDER BY q.item,q.at,q.scan`, key, since)
	if err != nil {
		return nil, err
	}
	cards := map[string]*Card{}
	for rows.Next() {
		var id, coverage string
		var at, qty int64
		var price float64
		var b []byte
		if err = rows.Scan(&id, &at, &price, &qty, &coverage, &b); err != nil {
			rows.Close()
			return nil, err
		}
		c := cards[id]
		if c == nil {
			c = &Card{Item: Item{ID: id, Name: id}, History: []Point{}}
			if len(b) > 0 {
				json.Unmarshal(b, &c.Item)
			}
			cards[id] = c
		}
		c.Price, c.Quantity, c.At, c.Coverage = price, qty, at, coverage
		if at >= since {
			c.History = append(c.History, Point{at, price, qty, coverage})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	watches, err := profile.db.Query(`SELECT item FROM watches WHERE market=?`, key)
	if err != nil {
		return nil, err
	}
	for watches.Next() {
		var id string
		watches.Scan(&id)
		if c := cards[id]; c != nil {
			c.Watch = true
		}
	}
	err = watches.Err()
	watches.Close()
	if err != nil {
		return nil, err
	}
	trades, err := profile.Trades(key)
	if err != nil {
		return nil, err
	}
	for _, t := range trades {
		if cards[t.Item] == nil {
			cards[t.Item] = &Card{Item: Item{ID: t.Item, Name: t.Item}, History: []Point{}}
		}
		if c := cards[t.Item]; c != nil {
			if t.Side == "buy" {
				c.Basis += t.Total
				c.Holding += t.Quantity
			} else {
				if c.Holding > 0 {
					sold := t.Quantity
					if sold > c.Holding {
						sold = c.Holding
					}
					removed := new(big.Int).Quo(new(big.Int).Mul(big.NewInt(c.Basis), big.NewInt(sold)), big.NewInt(c.Holding)).Int64()
					c.Basis -= removed
					c.Realized += t.Total - removed
				}
				c.Holding -= t.Quantity
			}
		}
	}
	out := []Card{}
	var fullAt int64
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(at),0) FROM scans WHERE market=? AND complete=1 AND coverage IN ('full','summary')`, key).Scan(&fullAt)
	for _, c := range cards {
		if c.At < fullAt {
			c.Quantity = 0
		}
		if len(c.History) > 1 && c.History[0].Price > 0 {
			v := (c.Price/c.History[0].Price - 1) * 100
			c.Change = &v
		}
		if len(c.History) > 240 {
			v := make([]Point, 240)
			for i := range v {
				v[i] = c.History[i*(len(c.History)-1)/239]
			}
			c.History = v
		}
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
