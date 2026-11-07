// Package kbb synchronizes player-authored planner records. It never writes
// SavedVariables or game input; WoW consumes the published data on /reload.
package kbb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxTransfer = 128 * 1024
const MaxFile = 16 * 1024 * 1024
const MaxRecords = 21000

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var guidPattern = regexp.MustCompile(`^Player-[A-Za-z0-9-]{1,100}$`)

type Stamp struct {
	At    int64  `json:"at"`
	Seq   int64  `json:"seq"`
	Actor string `json:"actor"`
}

func (a Stamp) Compare(b Stamp) int {
	if a.At < b.At {
		return -1
	}
	if a.At > b.At {
		return 1
	}
	if a.Seq < b.Seq {
		return -1
	}
	if a.Seq > b.Seq {
		return 1
	}
	return strings.Compare(a.Actor, b.Actor)
}

type Record struct {
	Key     string          `json:"key"`
	Stamp   Stamp           `json:"stamp"`
	Deleted bool            `json:"deleted,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}
type Snapshot struct {
	Schema  int      `json:"schema"`
	Client  string   `json:"client"`
	GUID    string   `json:"guid"`
	Name    string   `json:"name"`
	Realm   string   `json:"realm"`
	Records []Record `json:"records"`
}
type Column struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ShowOnUI    bool   `json:"showOnUI,omitempty"`
	Locked      bool   `json:"uiLocked,omitempty"`
	Transparent bool   `json:"uiTransparent,omitempty"`
	Compact     bool   `json:"uiCompact,omitempty"`
	Collapsed   bool   `json:"uiCollapsed,omitempty"`
	Placement   string `json:"uiPlacement,omitempty"`
	Position    *struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"uiPosition,omitempty"`
	Size *struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"uiSize,omitempty"`
	Visibility map[string]bool `json:"uiVisibility,omitempty"`
}
type Field struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Hidden  bool     `json:"hidden,omitempty"`
	Options []string `json:"options,omitempty"`
}
type Board struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Order            float64  `json:"order"`
	Columns          []Column `json:"columns"`
	Fields           []Field  `json:"fields"`
	QuestColumn      string   `json:"questColumn"`
	DoneColumn       string   `json:"doneColumn"`
	AutoWatch        bool     `json:"autoWatch,omitempty"`
	AutoDone         bool     `json:"autoDone,omitempty"`
	HideQuestTracker bool     `json:"hideQuestTracker,omitempty"`
}
type Card struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Notes         string         `json:"notes"`
	Column        string         `json:"column"`
	Kind          string         `json:"kind"`
	Priority      string         `json:"priority"`
	Values        map[string]any `json:"values"`
	Order         float64        `json:"order"`
	Archived      bool           `json:"archived,omitempty"`
	Created       int64          `json:"created,omitempty"`
	Updated       int64          `json:"updated,omitempty"`
	QuestID       int64          `json:"questID,omitempty"`
	QuestStatus   string         `json:"questStatus,omitempty"`
	QuestProgress string         `json:"questProgress,omitempty"`
}

func Decode(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}
func text(s string, n int, blank bool) bool {
	if !utf8.ValidString(s) || len(s) > n || (!blank && strings.TrimSpace(s) == "") {
		return false
	}
	for _, c := range s {
		if c < 32 && c != '\n' && c != '\r' && c != '\t' {
			return false
		}
	}
	return true
}
func one(s string, choices ...string) bool {
	for _, v := range choices {
		if s == v {
			return true
		}
	}
	return false
}
func validOrder(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1e9 }
func (r *Record) Validate() error {
	p := strings.Split(r.Key, "/")
	if (len(p) != 2 || p[0] != "b") && (len(p) != 3 || p[0] != "c") {
		return fmt.Errorf("invalid planner record key")
	}
	for _, s := range p[1:] {
		if !idPattern.MatchString(s) {
			return fmt.Errorf("invalid planner ID")
		}
	}
	if r.Stamp.At < 0 || r.Stamp.At > 4102444800000 || r.Stamp.Seq < 0 || r.Stamp.Seq > 2147483647 || !one(r.Stamp.Actor, "game", "desktop") {
		return fmt.Errorf("invalid change timestamp")
	}
	if r.Deleted {
		if len(r.Data) > 0 && string(r.Data) != "null" {
			return fmt.Errorf("deleted record contains content")
		}
		r.Data = nil
		return nil
	}
	if len(r.Data) == 0 || len(r.Data) > 64*1024 {
		return fmt.Errorf("invalid planner record size")
	}
	if p[0] == "b" {
		var b Board
		if err := Decode(r.Data, &b); err != nil {
			return err
		}
		if b.ID != p[1] || !text(b.Name, 60, false) || !validOrder(b.Order) || len(b.Columns) < 1 || len(b.Columns) > 12 || len(b.Fields) > 16 {
			return fmt.Errorf("invalid board")
		}
		ids, names := map[string]bool{}, map[string]bool{}
		for _, c := range b.Columns {
			if !idPattern.MatchString(c.ID) || ids[c.ID] || !text(c.Name, 40, false) || names[strings.ToLower(c.Name)] || !one(c.Placement, "", "floating", "tracker", "both") {
				return fmt.Errorf("invalid or duplicate column")
			}
			ids[c.ID] = true
			names[strings.ToLower(c.Name)] = true
			if c.Size != nil && (c.Size.Width < 220 || c.Size.Width > 640 || c.Size.Height < 150 || c.Size.Height > 4096) {
				return fmt.Errorf("invalid list size")
			}
			if c.Position != nil && (c.Position.X < 0 || c.Position.X > 1 || c.Position.Y < 0 || c.Position.Y > 1) {
				return fmt.Errorf("invalid list position")
			}
			for k := range c.Visibility {
				if !one(k, "world", "party", "key", "raid", "scenario", "arena", "pvp") {
					return fmt.Errorf("invalid display context")
				}
			}
		}
		if !ids[b.QuestColumn] || !ids[b.DoneColumn] {
			return fmt.Errorf("invalid quest destination")
		}
		ids, names = map[string]bool{}, map[string]bool{}
		for _, f := range b.Fields {
			if !idPattern.MatchString(f.ID) || ids[f.ID] || !text(f.Name, 40, false) || names[strings.ToLower(f.Name)] || !one(f.Type, "Text", "Number", "Checkbox", "Choice") {
				return fmt.Errorf("invalid custom field")
			}
			ids[f.ID] = true
			names[strings.ToLower(f.Name)] = true
			if (f.Type == "Choice" && (len(f.Options) < 2 || len(f.Options) > 12)) || (f.Type != "Choice" && len(f.Options) > 0) {
				return fmt.Errorf("invalid field choices")
			}
			seen := map[string]bool{}
			for _, v := range f.Options {
				if !text(v, 60, false) || seen[strings.ToLower(v)] {
					return fmt.Errorf("invalid field choice")
				}
				seen[strings.ToLower(v)] = true
			}
		}
		if b.Fields == nil {
			b.Fields = []Field{}
		}
		r.Data, _ = json.Marshal(b)
	} else {
		var c Card
		if err := Decode(r.Data, &c); err != nil {
			return err
		}
		if c.ID != p[2] || !text(c.Title, 160, false) || !text(c.Notes, 8000, true) || !idPattern.MatchString(c.Column) || !validOrder(c.Order) || !one(c.Kind, "Note", "Dungeon", "Achievement", "Recipe", "Rotation", "Quest") || !one(c.Priority, "Normal", "High", "Low") || len(c.Values) > 16 || c.QuestID < 0 || c.QuestID > 2147483647 || !text(c.QuestStatus, 80, true) || !text(c.QuestProgress, 16000, true) || c.Created < 0 || c.Updated < 0 {
			return fmt.Errorf("invalid card")
		}
		for k, v := range c.Values {
			if !idPattern.MatchString(k) {
				return fmt.Errorf("invalid custom field ID")
			}
			switch x := v.(type) {
			case string:
				if !text(x, 500, true) {
					return fmt.Errorf("invalid field text")
				}
			case float64:
				if math.IsInf(x, 0) || math.IsNaN(x) {
					return fmt.Errorf("invalid field number")
				}
			case bool:
			default:
				return fmt.Errorf("invalid field value")
			}
		}
		if c.Values == nil {
			c.Values = map[string]any{}
		}
		r.Data, _ = json.Marshal(c)
	}
	var data any
	if err := json.Unmarshal(r.Data, &data); err != nil {
		return err
	}
	r.Data, _ = json.Marshal(data)
	return nil
}
func (s *Snapshot) Validate(client string, now time.Time) error {
	if s.Schema != 1 || s.Client != client || !guidPattern.MatchString(s.GUID) || !text(s.Name, 128, true) || !text(s.Realm, 128, true) || len(s.Records) > MaxRecords {
		return fmt.Errorf("invalid planner identity or size")
	}
	seen := map[string]bool{}
	for i := range s.Records {
		r := &s.Records[i]
		if seen[r.Key] {
			return fmt.Errorf("duplicate planner record")
		}
		seen[r.Key] = true
		if err := r.Validate(); err != nil {
			return err
		}
		if r.Stamp.At > now.Add(5*time.Minute).UnixMilli() {
			return fmt.Errorf("planner clock is ahead; check the computer clock")
		}
	}
	return nil
}
