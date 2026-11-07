package overlays

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"github.com/tm-LBenson/lamda-engine/internal/spellbook"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

//go:embed web/*
var assets embed.FS

func Files() http.Handler { f, _ := fs.Sub(assets, "web"); return http.FileServer(http.FS(f)) }

var ErrConflict = errors.New("The rules changed in another window. Reload this page before saving")

const Marker = "-- LamdaUI managed Overlays data v1"
const MaxRulesBytes = 4 * 1024 * 1024

type Document struct {
	Version   int    `json:"version"`
	Revision  int64  `json:"revision"`
	UpdatedAt string `json:"updatedAt"`
	Rules     []Rule `json:"rules"`
}
type Store struct {
	captureDisabled  bool
	combatRequested  bool
	history          packHistoryFile
	historyErr       error
	memoryIO         sync.Mutex
	execution        []Rule
	executionSource  []Rule
	mediaUnavailable map[string]bool
	media            MediaLibrary
	mediaErr         error
	combat           combatFeed
	icons            *spellbook.IconCache
	backgroundMu     sync.Mutex
	mu               sync.Mutex
	dir              string
	doc              Document
	runtime          *Runtime
	gameFile         string
	publishError     string
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, doc: Document{Version: 1, Rules: []Rule{}}, runtime: NewRuntime(), icons: spellbook.NewIconCache(filepath.Join(dir, "icons"))}
	b, e := os.ReadFile(filepath.Join(dir, "rules.json"))
	if e == nil {
		if len(b) > MaxRulesBytes {
			return nil, fmt.Errorf("Overlay file exceeds limit")
		}
		if e = json.Unmarshal(b, &s.doc); e != nil {
			return nil, e
		}
		if s.doc.Version != 1 {
			return nil, fmt.Errorf("Unsupported overlay version")
		}
		if e = Validate(s.doc.Rules); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	s.loadMedia()
	s.loadMemory()
	s.loadPackHistory()
	return s, nil
}
func (s *Store) Icon(ctx context.Context, id int) string { return s.icons.Get(ctx, id) }
func (s *Store) State() any {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshMediaLocked()
	return struct {
		Document
		Live         any         `json:"live"`
		WatchHash    string      `json:"watchHash"`
		CooldownHash string      `json:"cooldownHash"`
		PublishError string      `json:"publishError,omitempty"`
		Sonar        bool        `json:"sonar"`
		Media        MediaStatus `json:"media"`
		MemoryError  string      `json:"memoryError,omitempty"`
		HistoryError string      `json:"historyError,omitempty"`
	}{s.doc, s.runtime.Status(time.Now()), WatchHash(Watches(s.doc.Rules)), CooldownHash(CooldownWatches(s.doc.Rules)), s.publishError, s.SoundPath("sonar") != "", s.mediaStatusLocked(), s.runtime.savedMemory.problem(), s.packHistoryProblem()}
}
func WatchHash(w []Watch) string {
	b, _ := json.Marshal(w)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16]
}
func atomic(path string, data []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".overlays-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(data); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Rename(name, path)
}
func (s *Store) Save(revision int64, rules []Rule) error {
	return s.save(revision, rules, nil)
}
func (s *Store) SavePackUpdates(revision int64, rules []Rule, instances []string) error {
	return s.save(revision, rules, instances)
}
func (s *Store) save(revision int64, rules []Rule, updates []string) error {
	if rules == nil {
		rules = []Rule{}
	}
	if err := Validate(rules); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if revision != s.doc.Revision {
		return ErrConflict
	}
	d := Document{1, s.doc.Revision + 1, time.Now().UTC().Format(time.RFC3339Nano), rules}
	b, e := json.MarshalIndent(d, "", "  ")
	if e != nil {
		return e
	}
	if len(b) > MaxRulesBytes {
		return fmt.Errorf("Keep saved overlays under 4 MB")
	}
	if s.doc.Revision > 0 {
		old, _ := json.MarshalIndent(s.doc, "", "  ")
		if e = atomic(filepath.Join(s.dir, "rules.previous.json"), old); e != nil {
			return e
		}
	}
	if e = s.backupPackUpdatesLocked(rules, updates); e != nil {
		return e
	}
	if e = atomic(filepath.Join(s.dir, "rules.json"), b); e != nil {
		return e
	}
	s.doc = d
	s.runtime.savedMemory.prune(rules)
	s.execution = nil
	s.executionSource = nil
	s.runtime.Reset()
	s.publishLocked()
	return nil
}
func (s *Store) SetGameFile(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gameFile = path
	s.publishLocked()
}
func (s *Store) publishLocked() {
	s.publishError = ""
	if s.gameFile == "" {
		return
	}
	for p := s.gameFile; ; p = filepath.Dir(p) {
		info, e := os.Lstat(p)
		if e != nil || info.Mode()&os.ModeSymlink != 0 {
			s.publishError = "Addon data path unavailable"
			return
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	b, e := os.ReadFile(s.gameFile)
	if e != nil || !bytes.HasPrefix(b, []byte(Marker+"\n")) {
		s.publishError = "Addon update required"
		return
	}
	w := Watches(s.doc.Rules)
	var out bytes.Buffer
	fmt.Fprintf(&out, "%s\nLamdaOverlaysData = {[\"hash\"]=%q,[\"watches\"]={\n", Marker, WatchHash(w))
	for _, v := range w {
		fmt.Fprintf(&out, "{[\"spell\"]=%d,[\"kind\"]=%q,[\"unit\"]=%q,},\n", v.Spell, v.Kind, v.Unit)
	}
	cd := CooldownWatches(s.doc.Rules)
	fmt.Fprintf(&out, "},[\"cooldownHash\"]=%q,[\"cooldowns\"]={\n", CooldownHash(cd))
	for _, v := range cd {
		fmt.Fprintf(&out, "{[\"kind\"]=%q,[\"id\"]=%d,},\n", v.Kind, v.ID)
	}
	talents := TalentWatches(s.doc.Rules)
	fmt.Fprintf(&out, "},[\"talentHash\"]=%q,[\"talents\"]={\n", TalentHash(talents))
	for _, w := range talents {
		fmt.Fprintf(&out, "{[\"kind\"]=%q,[\"id\"]=%d,},\n", w.Kind, w.ID)
	}
	fmt.Fprintf(&out, "},[\"talentChanges\"]=%t,[\"equipmentChanges\"]=%t,[\"equipment\"]=%t,[\"equipmentSets\"]={\n", needsLoadoutChange(s.doc.Rules, "talents"), needsLoadoutChange(s.doc.Rules, "equipment"), needsEquipment(s.doc.Rules))
	for _, w := range EquipmentSets(s.doc.Rules) {
		fmt.Fprintf(&out, "{[\"key\"]=%q,[\"name\"]=%q,},\n", w.Key, w.Name)
	}
	casts := CastWatches(s.doc.Rules)
	fmt.Fprintf(&out, "},[\"castHash\"]=%q,[\"casts\"]={\n", CastHash(casts))
	for _, u := range casts {
		fmt.Fprintf(&out, "%q,\n", u)
	}
	out.WriteString("},[\"anchors\"]={\n")
	for _, a := range Anchors(s.doc.Rules) {
		fmt.Fprintf(&out, "{[\"kind\"]=%q,[\"target\"]=%q,},\n", a.Kind, a.Target)
	}
	mw := MeasurementWatches(s.doc.Rules)
	fmt.Fprintf(&out, "},[\"measurementHash\"]=%q,[\"measurements\"]={\n", MeasurementHash(mw))
	for _, w := range mw {
		fmt.Fprintf(&out, "{[\"unit\"]=%q,[\"kind\"]=%q,[\"resource\"]=%q,},\n", w.Unit, w.Kind, w.Resource)
	}
	identities := CombatWatches(s.doc.Rules)
	fmt.Fprintf(&out, "},[\"combat\"]=%t,[\"identityHash\"]=%q,[\"identities\"]={\n", len(identities) > 0, CastHash(identities))
	for _, u := range identities {
		fmt.Fprintf(&out, "%q,\n", u)
	}
	tokens := UnitTokens(s.doc.Rules)
	out.WriteString("},[\"unitTokens\"]={\n")
	for _, unit := range tokens {
		fmt.Fprintf(&out, "%q,\n", unit)
	}
	fmt.Fprintf(&out, "},[\"unitHash\"]=%q,[\"bosses\"]=%t,[\"encounters\"]=%t,[\"units\"]=%t,}\n", lookup.OverlayUnitHash(tokens), needsBosses(s.doc.Rules), needsEncounters(s.doc.Rules), len(tokens) > 0)
	if bytes.Equal(b, out.Bytes()) {
		return
	}
	if e = atomic(s.gameFile, out.Bytes()); e != nil {
		s.publishError = e.Error()
	}
}

// SoundPath only serves a bounded, locally imported PCM file with a fixed name.
func (s *Store) SoundPath(name string) string {
	if name != "sonar" {
		return ""
	}
	p := filepath.Join(s.dir, "sounds", "sonar.wav")
	info, e := os.Lstat(p)
	if e != nil || !info.Mode().IsRegular() || info.Size() > 4*1024*1024 {
		return ""
	}
	return p
}
