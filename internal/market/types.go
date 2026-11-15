// Package market stores observations, rather than inferred auction sales.
package market

import (
	"fmt"
	"strings"
	"time"
)

const MaxEventBytes = 256 << 10

type Identity struct {
	Client  string `json:"client"`
	Region  string `json:"region"`
	Realm   string `json:"realm"`
	Faction string `json:"faction"`
}

func (m Identity) Key() string {
	return strings.Join([]string{m.Client, m.Region, m.Realm, m.Faction}, "/")
}
func (m Identity) Validate() error {
	if m.Client != "forever" && m.Client != "retail" {
		return fmt.Errorf("unsupported client")
	}
	for _, v := range []string{m.Region, m.Realm, m.Faction} {
		if v == "" || len(v) > 128 || strings.ContainsAny(v, "/\x00\n\r") {
			return fmt.Errorf("invalid market identity")
		}
	}
	return nil
}

type Item struct {
	BindingKnown bool   `json:"bindingKnown,omitempty"`
	ID           string `json:"id"`
	Name         string `json:"name"`
	Transferable bool   `json:"transferable"`
	ClassID      *int   `json:"classID,omitempty"`
	SubclassID   *int   `json:"subclassID,omitempty"`
	ClassName    string `json:"className,omitempty"`
	SubclassName string `json:"subclassName,omitempty"`
}

// Price is the total buyout for one lot; Count identical lots are offered.
// Divisible is true only for commodity search results that allow partial purchases.
type Lot struct {
	Item      string `json:"item"`
	Price     int64  `json:"price"`
	Quantity  int64  `json:"quantity"`
	Count     int64  `json:"count"`
	Divisible bool   `json:"divisible"`
}
type Vendor struct {
	ID          string `json:"id"`
	Item        string `json:"item"`
	Name        string `json:"name"`
	Buy         int64  `json:"buy"`
	Sell        int64  `json:"sell"`
	Bundle      int64  `json:"bundle"`
	Stock       int64  `json:"stock"`
	Requirement string `json:"requirement"`
	Location    string `json:"location"`
}
type Profession struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Rank int    `json:"rank"`
	Max  int    `json:"max"`
}
type Character struct {
	Surname           string              `json:"surname,omitempty"`
	Class             string              `json:"class,omitempty"`
	KnownByProfession map[string][]string `json:"knownByProfession,omitempty"`
	RecipeTimes       map[string]int64    `json:"recipeTimes,omitempty"`
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Professions       []Profession        `json:"professions"`
	Known             []string            `json:"known"`
	Inventory         map[string]int64    `json:"inventory"`
	At                int64               `json:"at"`
	RecipesObserved   bool                `json:"recipesObserved"`
}

func (c Character) DisplayName() string {
	if c.Surname != "" && !strings.HasSuffix(c.Name, "-"+c.Surname) {
		return c.Name + "-" + c.Surname
	}
	return c.Name
}

type Reagent struct {
	Item     string `json:"item"`
	Quantity int64  `json:"quantity"`
}
type Recipe struct {
	Custom      bool      `json:"custom,omitempty"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Profession  string    `json:"profession"`
	Rank        int       `json:"rank"`
	RankKnown   bool      `json:"rankKnown"`
	Inputs      []Reagent `json:"inputs"`
	Output      Reagent   `json:"output"`
	Cooldown    string    `json:"cooldown"`
	Requirement string    `json:"requirement"`
	Source      string    `json:"source"`
}
type Event struct {
	Version   int        `json:"version"`
	ID        string     `json:"id"`
	Market    Identity   `json:"market"`
	At        int64      `json:"at"`
	Kind      string     `json:"kind"`
	Scan      string     `json:"scan,omitempty"`
	Page      int        `json:"page,omitempty"`
	Pages     int        `json:"pages,omitempty"`
	Coverage  string     `json:"coverage,omitempty"`
	Items     []Item     `json:"items,omitempty"`
	Lots      []Lot      `json:"lots,omitempty"`
	Vendors   []Vendor   `json:"vendors,omitempty"`
	Character *Character `json:"character,omitempty"`
	Recipes   []Recipe   `json:"recipes,omitempty"`
}

func validID(s string) bool {
	return len(s) > 0 && len(s) <= 200 && !strings.ContainsAny(s, "\x00\n\r")
}
func validMoney(n int64) bool    { return n >= 0 && n <= 1_000_000_000_000 }
func validQuantity(n int64) bool { return n > 0 && n <= 10_000_000 }
func (e Event) Validate(now time.Time) error {
	if err := e.Market.Validate(); err != nil {
		return err
	}
	if e.Version != 1 || !validID(e.ID) || e.At < 946684800 || e.At > now.Add(10*time.Minute).Unix() {
		return fmt.Errorf("invalid event version, id or timestamp")
	}
	if len(e.Items) > 4000 || len(e.Lots) > 4000 || len(e.Recipes) > 1000 || len(e.Vendors) > 1000 {
		return fmt.Errorf("event too large; split into pages")
	}
	for _, i := range e.Items {
		if !validID(i.ID) || len(i.Name) > 500 {
			return fmt.Errorf("invalid item")
		}
		if (i.ClassID != nil && (*i.ClassID < 0 || *i.ClassID > 255)) ||
			(i.SubclassID != nil && (i.ClassID == nil || *i.SubclassID < 0 || *i.SubclassID > 255)) ||
			len(i.ClassName) > 200 || len(i.SubclassName) > 200 {
			return fmt.Errorf("invalid item category")
		}
	}
	for _, l := range e.Lots {
		if !validID(l.Item) || !validQuantity(l.Quantity) || !validQuantity(l.Count) || !validMoney(l.Price) || l.Price == 0 {
			return fmt.Errorf("invalid lot")
		}
	}
	for _, v := range e.Vendors {
		if !validID(v.ID) || !validID(v.Item) || !validQuantity(v.Bundle) || v.Stock < -1 || v.Stock > 10_000_000 || !validMoney(v.Buy) || !validMoney(v.Sell) || len(v.Requirement) > 1000 || len(v.Location) > 500 {
			return fmt.Errorf("invalid vendor offer")
		}
	}
	for _, r := range e.Recipes {
		if !validID(r.ID) || !validID(r.Profession) || len(r.Inputs) == 0 || len(r.Inputs) > 32 || !validID(r.Output.Item) || !validQuantity(r.Output.Quantity) || r.Rank < 0 || r.Rank > 10000 || len(r.Source) > 500 {
			return fmt.Errorf("invalid recipe")
		}
		for _, x := range r.Inputs {
			if !validID(x.Item) || !validQuantity(x.Quantity) {
				return fmt.Errorf("invalid reagent")
			}
		}
	}
	if c := e.Character; c != nil {
		if len(c.Surname) > 120 || len(c.Class) > 30 || strings.ContainsAny(c.Surname+c.Class, "\x00\n\r") {
			return fmt.Errorf("invalid character surname or class")
		}
		if !validID(c.ID) || len(c.Name) > 200 || len(c.Known) > 50000 || len(c.Inventory) > 5000 || len(c.Professions) > 32 {
			return fmt.Errorf("invalid character")
		}
		if len(c.KnownByProfession) > 32 || len(c.RecipeTimes) > 32 {
			return fmt.Errorf("too many profession exports")
		}
		totalKnown := 0
		for id, known := range c.KnownByProfession {
			if !validID(id) {
				return fmt.Errorf("invalid profession export")
			}
			totalKnown += len(known)
			for _, recipe := range known {
				if !validID(recipe) {
					return fmt.Errorf("invalid recipe ID")
				}
			}
		}
		if totalKnown > 50000 {
			return fmt.Errorf("too many known recipes")
		}
		for id, at := range c.RecipeTimes {
			if !validID(id) || at < 0 || at > now.Add(10*time.Minute).Unix() {
				return fmt.Errorf("invalid recipe export timestamp")
			}
		}
		for _, p := range c.Professions {
			if !validID(p.ID) || p.Rank < 0 || p.Max < 0 || p.Rank > 10000 || p.Max > 10000 {
				return fmt.Errorf("invalid profession")
			}
		}
		for _, id := range c.Known {
			if !validID(id) {
				return fmt.Errorf("invalid known recipe")
			}
		}
		for id, q := range c.Inventory {
			if !validID(id) || q < 0 || q > 10_000_000 {
				return fmt.Errorf("invalid inventory")
			}
		}
	}
	switch e.Kind {
	case "auction":
		if !validID(e.Scan) || e.Pages < 1 || e.Pages > 2048 || e.Page < 1 || e.Page > e.Pages || (e.Coverage != "full" && e.Coverage != "partial" && e.Coverage != "summary") {
			return fmt.Errorf("invalid scan page")
		}
	case "character":
		if e.Character == nil {
			return fmt.Errorf("character required")
		}
	case "vendor", "recipes", "items":
	default:
		return fmt.Errorf("unsupported event kind")
	}
	return nil
}

type Point struct {
	At       int64   `json:"at"`
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
	Coverage string  `json:"coverage"`
}
type Card struct {
	Item
	Coverage string   `json:"coverage,omitempty"`
	Price    float64  `json:"price"`
	Quantity int64    `json:"quantity"`
	At       int64    `json:"at"`
	Change   *float64 `json:"change"`
	History  []Point  `json:"history"`
	Watch    bool     `json:"watch"`
	Holding  int64    `json:"holding"`
	Basis    int64    `json:"basis"`
	Realized int64    `json:"realized"`
}
type Settings struct {
	AllowUnknownRanks bool           `json:"allowUnknownRanks"`
	Batch             int64          `json:"batch"`
	TransferCost      int64          `json:"transferCost"`
	CutBPS            int64          `json:"cutBps"`
	LostDeposit       int64          `json:"lostDeposit"`
	MaxAgeHours       int            `json:"maxAgeHours"`
	Mode              string         `json:"mode"`
	Character         string         `json:"character"`
	Theory            map[string]int `json:"theory"`
}

func DefaultSettings() Settings {
	return Settings{Batch: 1, CutBPS: 500, MaxAgeHours: 24, Mode: "characters", Theory: map[string]int{}}
}

type Trade struct {
	ID       string `json:"id"`
	Item     string `json:"item"`
	Side     string `json:"side"`
	Quantity int64  `json:"quantity"`
	Total    int64  `json:"total"`
	At       int64  `json:"at"`
}
