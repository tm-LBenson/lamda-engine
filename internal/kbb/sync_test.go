package kbb

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/lookup"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixtureTime = time.Unix(1791680000, 0)

func fixture(t *testing.T) Snapshot {
	t.Helper()
	var s Snapshot
	err := Decode([]byte(`{"schema":1,"client":"retail","guid":"Player-1-first","name":"Fixture","realm":"Test Realm","records":[{"key":"b/b1","stamp":{"at":1,"seq":0,"actor":"game"},"data":{"id":"b1","name":"Farm","order":1,"columns":[{"id":"c1","name":"Want to do"}],"fields":[],"questColumn":"c1","doneColumn":"c1"}},{"key":"c/b1/t2","stamp":{"at":2,"seq":0,"actor":"game"},"data":{"id":"t2","title":"Original","notes":"- Loot","column":"c1","kind":"Dungeon","priority":"Normal","values":{},"order":1}}]}`), &s)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func opened(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir(), "retail")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Merge(fixture(t), fixtureTime); err != nil {
		t.Fatal(err)
	}
	return s
}
func editTitle(t *testing.T, s *Store, title string) (Record, Edit) {
	t.Helper()
	r := s.State().Characters["Player-1-first"].Records["c/b1/t2"]
	var c Card
	_ = json.Unmarshal(r.Data, &c)
	c.Title = title
	b, _ := json.Marshal(c)
	e := Edit{"Player-1-first", []Change{{Key: r.Key, Base: &r.Stamp, Data: b}}}
	return r, e
}
func TestMergeEditReplayHistoryRestart(t *testing.T) {
	s := opened(t)
	old, e := editTitle(t, s, "Desktop")
	if err := s.Edit(e, fixtureTime); err != nil {
		t.Fatal(err)
	}
	if err := s.Edit(e, fixtureTime); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale editor: %v", err)
	}
	if err := s.Merge(fixture(t), fixtureTime); err != nil {
		t.Fatal(err)
	}
	c := s.State().Characters[e.GUID]
	if !strings.Contains(string(c.Records[old.Key].Data), "Desktop") || len(c.History) != 1 {
		t.Fatalf("replay overwrote current or duplicated history: %+v", c)
	}
	snap := fixture(t)
	snap.Records = snap.Records[:1]
	snap.Records[0].Stamp = Stamp{fixtureTime.UnixMilli() + 100, 0, "game"}
	snap.Records[0].Data = []byte(strings.Replace(string(snap.Records[0].Data), "Farm", "Another board name", 1))
	if err := s.Merge(snap, fixtureTime); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(filepath.Dir(s.path), "retail")
	if err != nil {
		t.Fatal(err)
	}
	c = reopened.State().Characters[e.GUID]
	if !strings.Contains(string(c.Records[old.Key].Data), "Desktop") || !strings.Contains(string(c.Records["b/b1"].Data), "Another board name") {
		t.Fatal("independent edits lost")
	}
	if err = os.WriteFile(s.path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(filepath.Dir(s.path), "retail"); err == nil {
		t.Fatal("corruption silently replaced")
	}
}
func TestTombstoneAtomicEditsAndDiskFailure(t *testing.T) {
	s := opened(t)
	old, e := editTitle(t, s, "Lost")
	e.Changes = append(e.Changes, Change{Key: "invalid", Data: []byte(`{}`)})
	if s.Edit(e, fixtureTime) == nil {
		t.Fatal("accepted invalid batch")
	}
	if !equal(s.State().Characters[e.GUID].Records[old.Key], old) {
		t.Fatal("partial batch committed")
	}
	e.Changes = []Change{{Key: old.Key, Base: &old.Stamp, Deleted: true}}
	if err := s.Edit(e, fixtureTime); err != nil {
		t.Fatal(err)
	}
	if err := s.Merge(fixture(t), fixtureTime); err != nil {
		t.Fatal(err)
	}
	if !s.State().Characters[e.GUID].Records[old.Key].Deleted {
		t.Fatal("resurrected deleted card")
	}
	s.path = filepath.Join(t.TempDir(), "absent", "planner.json")
	current := s.State().Characters[e.GUID].Records[old.Key]
	e.Changes[0] = Change{Key: old.Key, Base: &current.Stamp, Data: old.Data}
	if s.Edit(e, fixtureTime) == nil {
		t.Fatal("expected storage failure")
	}
	if !s.State().Characters[e.GUID].Records[old.Key].Deleted {
		t.Fatal("memory committed before durable write")
	}
}
func TestStampConflictFutureAndWrongClient(t *testing.T) {
	s := opened(t)
	snap := fixture(t)
	snap.Records[1].Data = []byte(strings.Replace(string(snap.Records[1].Data), "Original", "Equivocation", 1))
	if s.Merge(snap, fixtureTime) == nil {
		t.Fatal("equal stamp different data accepted")
	}
	snap = fixture(t)
	snap.Client = "forever"
	if s.Merge(snap, fixtureTime) == nil {
		t.Fatal("wrong client accepted")
	}
	snap = fixture(t)
	snap.Records[1].Stamp.At = fixtureTime.Add(time.Hour).UnixMilli()
	if s.Merge(snap, fixtureTime) == nil {
		t.Fatal("future clock accepted")
	}
}
func TestTransferOutOfOrderCRCAndWire(t *testing.T) {
	snap := fixture(t)
	data, _ := json.Marshal(snap)
	data = append(data, []byte(strings.Repeat(" ", 6000))...)
	count := (len(data) + 5599) / 5600
	a := Assembler{Client: "retail"}
	var got *Snapshot
	for i := count; i >= 1; i-- {
		end := min(i*5600, len(data))
		body := []byte(fmt.Sprintf("2|test|retail|%d|%d|%d|%d|%d\n%s", fixtureTime.Unix(), len(data), i, count, crc32.ChecksumIEEE(data), data[(i-1)*5600:end]))
		var err error
		got, err = a.Accept(body, fixtureTime)
		if err != nil {
			t.Fatal(err)
		}
		if i > 1 && got != nil {
			t.Fatal("partial snapshot applied")
		}
	}
	if got == nil || len(got.Records) != 2 {
		t.Fatal("transfer did not assemble")
	}
	if _, err := a.Accept([]byte("2|bad|forever|1|1|1|1|1\nx"), fixtureTime); err == nil {
		t.Fatal("cross-client transfer")
	}
}
func TestPublicationSafety(t *testing.T) {
	s := opened(t)
	root := t.TempDir()
	folder := filepath.Join(root, "lamdaUI")
	_ = os.Mkdir(folder, 0700)
	_ = os.WriteFile(filepath.Join(folder, "KBBData.lua"), []byte(Seed), 0600)
	if s.Publish(root) == nil {
		t.Fatal("published without opt-in")
	}
	toc := "## X-LamdaFrames-Managed: 1\n## X-Lamda-KBB: 1\n## X-Lamda-KBB-Client: retail\nKBBData.lua\nModules/KBBSync.lua\n"
	_ = os.WriteFile(filepath.Join(folder, "lamdaUI.toc"), []byte(toc), 0600)
	_, e := editTitle(t, s, `quotes " \ ]=]; _G.BAD=true; --`)
	if err := s.Edit(e, fixtureTime); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(folder, "KBBData.lua"))
	if !strings.Contains(string(data), `\"`) {
		t.Fatal("quotes were not escaped")
	}
	if !s.State().Published {
		t.Fatal("publish status missing")
	}
	_ = os.WriteFile(filepath.Join(folder, "KBBData.lua"), []byte("unmanaged"), 0600)
	if s.Publish(root) == nil {
		t.Fatal("overwrote unmanaged file")
	}
}
func TestRealLuaJSONPacketAndDesktopReload(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	python := filepath.Join(root, ".venv/bin/python")
	if _, err = os.Stat(python); err != nil {
		t.Skip("project Lua test environment unavailable")
	}
	script := `import sys,json,base64
sys.path.insert(0,'Tools/ui-tests')
from pathlib import Path
import test_addon;test_addon.ADDON=Path('Lamda/UIAddon')
from test_kbb_sync import synced
from test_addon import ADDON
lua=synced()
lua.execute('function UIParent:GetEffectiveScale()return 1 end;PixelUtil={GetPixelToUIUnitFactor=function()return 1 end}')
lua.execute('assert(loadstring(...))("lamdaUI",LUI)', (ADDON/'LookupEvents.lua').read_text())
lua.execute('S:Tick()')
packet=lua.eval('function()local p=S.packets[1];local t={};for i=1,#p do t[i]=p:byte(i)end;return table.concat(t,",")end')()
print(packet)
`
	cmd := exec.Command(python, "-c", script)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Lua sender: %v\n%s", err, out)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	data := make([]byte, len(parts))
	for i, p := range parts {
		var n int
		if _, err = fmt.Sscan(p, &n); err != nil {
			t.Fatal(err)
		}
		data[i] = byte(n)
	}
	sample := lookup.DecodeSample(data)
	if sample.Error != "" || len(sample.KBB) == 0 {
		t.Fatalf("wire decode %+v", sample)
	}
	a := Assembler{Client: "retail"}
	snap, err := a.Accept(sample.KBB, fixtureTime)
	if err != nil || snap == nil {
		t.Fatalf("snapshot: %v", err)
	}
	s, err := Open(t.TempDir(), "retail")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Merge(*snap, fixtureTime); err != nil {
		t.Fatal(err)
	}
	state := s.State()
	c := state.Characters[snap.GUID]
	var target Record
	for key, r := range c.Records {
		if strings.HasPrefix(key, "c/") {
			target = r
			break
		}
	}
	var card Card
	_ = json.Unmarshal(target.Data, &card)
	card.Title = "Desktop round trip"
	card.Notes = "- bullet\n\"quote\" and \\ slash"
	raw, _ := json.Marshal(card)
	if err = s.Edit(Edit{snap.GUID, []Change{{Key: target.Key, Base: &target.Stamp, Data: raw}}}, fixtureTime); err != nil {
		t.Fatal(err)
	}
	luaData, err := BuildData(s.State())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "KBBData.lua")
	_ = os.WriteFile(path, luaData, 0600)
	script = `import sys,json
sys.path.insert(0,'Tools/ui-tests');sys.path.insert(0,'Tools')
from pathlib import Path
import test_addon;test_addon.ADDON=Path('Lamda/UIAddon')
from test_kbb_sync import synced
from sim_results_runtime import validate_kbb
from pathlib import Path
source=Path(sys.argv[1]).read_bytes();validate_kbb(source,'retail')
lua=synced();lua.execute(source.decode());lua.execute('S.ready=false;S:Initialize();assert(not S.error,S.error);assert(LUI.KBB:Board().cards[1].title=="Desktop round trip")')
print(lua.eval('S:Batches()[1]'))
`
	cmd = exec.Command(python, "-c", script, path)
	cmd.Dir = root
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Lua reload: %v\n%s", err, out)
	}
	var echo Snapshot
	if err = Decode(out, &echo); err != nil {
		t.Fatal(err)
	}
	if err = s.Merge(echo, fixtureTime); err != nil {
		t.Fatal(err)
	}
	if s.State().Characters[snap.GUID].GameSeen[target.Key] != s.State().Characters[snap.GUID].Records[target.Key].Stamp {
		t.Fatal("reload echo not acknowledged")
	}
}
