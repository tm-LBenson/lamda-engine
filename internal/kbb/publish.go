package kbb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const Marker = "-- LamdaUI managed KBB data v1"
const Seed = Marker + "\nLamdaKBBData = {}\n"

func writeLuaString(out *bytes.Buffer, text string) error {
	if len(text) > 64*1024 {
		return fmt.Errorf("planner string exceeds the limit")
	}
	out.WriteByte('"')
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '"' || c == '\\':
			out.WriteByte('\\')
			out.WriteByte(c)
		case c < 32 || c == 127:
			fmt.Fprintf(out, "\\%03d", c)
		default:
			out.WriteByte(c)
		}
	}
	out.WriteByte('"')
	return nil
}

func writeLuaData(out *bytes.Buffer, value any, depth int, nodes *int) error {
	*nodes++
	if out.Len() > MaxFile || *nodes > 2000000 {
		return fmt.Errorf("planner export exceeds its limit")
	}
	if depth > 32 {
		return fmt.Errorf("planner nesting exceeds the limit")
	}
	switch v := value.(type) {
	case nil:
		out.WriteString("nil")
	case bool:
		out.WriteString(strconv.FormatBool(v))
	case string:
		return writeLuaString(out, v)
	case json.Number:
		n, err := strconv.ParseFloat(string(v), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return fmt.Errorf("planner contains an invalid number")
		}
		out.WriteString(string(v)) // json.Marshal already produced valid decimal syntax.
	case []any:
		out.WriteString("{\n")
		for _, item := range v {
			if err := writeLuaData(out, item, depth+1, nodes); err != nil {
				return err
			}
			out.WriteString(",\n")
		}
		out.WriteByte('}')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out.WriteString("{\n")
		for _, key := range keys {
			out.WriteByte('[')
			if err := writeLuaString(out, key); err != nil {
				return err
			}
			out.WriteString("]=")
			if err := writeLuaData(out, v[key], depth+1, nodes); err != nil {
				return err
			}
			out.WriteString(",\n")
		}
		out.WriteByte('}')
	default:
		return fmt.Errorf("unsupported in-game data type")
	}
	return nil
}

func checkGamePath(path string) error {
	for p := path; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("planners destination is linked")
		}
		if filepath.Dir(p) == p {
			return nil
		}
	}
}

// Only the record envelope enters Lua; engine history and receipts stay on disk.
func BuildData(state State) ([]byte, error) {
	chars := map[string]Snapshot{}
	for guid, c := range state.Characters {
		chars[guid] = Snapshot{1, state.Client, guid, c.Name, c.Realm, records(c)}
	}
	raw, err := json.Marshal(struct {
		Schema     int                 `json:"schema"`
		Client     string              `json:"client"`
		Characters map[string]Snapshot `json:"characters"`
	}{1, state.Client, chars})
	if err != nil {
		return nil, err
	}
	var tree any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err = d.Decode(&tree); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString("LamdaKBBData = ")
	nodes := 0
	if err = writeLuaData(&out, tree, 0, &nodes); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	digest := sha256.Sum256(out.Bytes())
	header := Marker + "\n-- Payload digest: " + hex.EncodeToString(digest[:]) + "\n"
	if len(header)+out.Len() > MaxFile {
		return nil, fmt.Errorf("planner export is too large")
	}
	return append([]byte(header), out.Bytes()...), nil
}
func destination(addons, client string) (string, []byte, error) {
	if !filepath.IsAbs(addons) {
		return "", nil, fmt.Errorf("planner requires an absolute AddOns path")
	}
	folder := filepath.Join(addons, "lamdaUI")
	toc := filepath.Join(folder, "lamdaUI.toc")
	path := filepath.Join(folder, "KBBData.lua")
	for _, p := range []string{toc, path} {
		if err := checkGamePath(p); err != nil {
			return "", nil, err
		}
	}
	data, err := readFile(toc, 65536)
	if err != nil {
		return "", nil, err
	}
	enabled, matching, count, di, mi := false, false, 0, -1, -1
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		line = strings.ReplaceAll(line, "\\", "/")
		if line == "## X-Lamda-KBB: 1" {
			enabled = true
		}
		if line == "## X-Lamda-KBB-Client: "+client {
			matching = true
		}
		if strings.HasPrefix(line, "## X-Lamda-KBB-Client:") {
			count++
		}
		if line == "KBBData.lua" {
			if di >= 0 {
				return "", nil, fmt.Errorf("duplicate planner data file")
			}
			di = i
		}
		if line == "Modules/KBBSync.lua" {
			mi = i
		}
	}
	if !enabled || !matching || count != 1 || di < 0 || mi <= di {
		return "", nil, fmt.Errorf("reload-ready planner sync requires the updated %s addon", client)
	}
	previous, err := readFile(path, MaxFile)
	if err != nil {
		return "", nil, err
	}
	if string(previous) != Seed {
		parts := bytes.SplitN(previous, []byte("\n"), 3)
		if len(parts) != 3 || string(parts[0]) != Marker {
			return "", nil, fmt.Errorf("unmanaged planner data file")
		}
		sum := sha256.Sum256(parts[2])
		if string(parts[1]) != "-- Payload digest: "+hex.EncodeToString(sum[:]) {
			return "", nil, fmt.Errorf("planner data file was modified outside Lamda Engine")
		}
	}
	return path, previous, nil
}
func (s *Store) Publish(addons string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := func() error {
		data := s.publication
		var err error
		if data == nil {
			data, err = BuildData(s.state)
			if err == nil {
				s.publication = data
			}
		}
		if err != nil {
			return err
		}
		path, old, err := destination(addons, s.state.Client)
		if err != nil {
			return err
		}
		if bytes.Equal(old, data) {
			return nil
		}
		return atomicWrite(path, ".kbb-data-*", data)
	}()
	s.publishError = ""
	s.published = err == nil
	if err != nil {
		s.publishError = err.Error()
	}
	return err
}
func (s *Store) RunPublisher(ctx context.Context, addons string, enabled ...func() bool) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		if len(enabled) == 0 || enabled[0] == nil || enabled[0]() {
			_ = s.Publish(addons)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
