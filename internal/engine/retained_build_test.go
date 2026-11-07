package engine

import (
	"testing"
	"time"
)

func TestRetainedScanRecoversAfterEngineRestartOnlyInLiveParty(t *testing.T) {
	now := time.Now()
	guid := "Player-1-ALICE"
	m := NewModel()
	b := BuildSnapshot{GUID: guid, Source: "inspect", At: now.Add(-time.Hour), SpecID: 73, GameBuild: "12.1.0", SpellRanks: map[int]int{6552: 1}}
	if m.ObserveInspectedBuild(b, now) || m.ObserveRetainedPartyBuild(b, now) {
		t.Fatal("stale scan accepted without live membership")
	}
	m.TrackPartyRoster(map[string]bool{guid: true}, now)
	if !m.ObserveRetainedPartyBuild(b, now) || !m.HasInspectedBuild(guid, now) {
		t.Fatal("held scan not restored")
	}
	if !m.InspectedBuilds[guid].At.Equal(b.At) {
		t.Fatal("inspection timestamp rewritten")
	}
	// A newer scan can first arrive after a long hidden interval. It must
	// replace the previously retained scan without changing its timestamp.
	b.At = now.Add(-10 * time.Minute)
	b.SpellRanks = map[int]int{6552: 2}
	if !m.ObserveRetainedPartyBuild(b, now) {
		t.Fatal("newer retained scan rejected")
	}
	current, ok := m.buildFor(guid, now)
	if !ok || current.SpellRanks[6552] != 2 || !current.At.Equal(b.At) {
		t.Fatal("newer retained scan did not replace previous scan")
	}
	m.TrackPartyRoster(map[string]bool{}, now.Add(time.Second))
	if m.HasInspectedBuild(guid, now.Add(time.Second)) {
		t.Fatal("held build survived party departure")
	}
}

func TestRetainedScanRefreshKeepsClockButChangedBuildDoesNot(t *testing.T) {
	start := time.Unix(1789500000, 0)
	guid := "Player-1-FRIEND"
	m := NewModel()
	m.TrackPartyRoster(map[string]bool{guid: true}, start)
	inspectedKeyMember(m, guid, start, 264, map[int]int{108271: 1})
	b := m.partyBuilds[guid].Build
	b.At = start.Add(10 * time.Minute)
	now := start.Add(20 * time.Minute)
	if !m.ObserveRetainedPartyBuild(b, now) || !m.partyBuilds[guid].Since.Equal(start) {
		t.Fatal("unchanged held scan restarted warm-up")
	}
	b.At = start.Add(15 * time.Minute)
	b.SpellRanks = map[int]int{108271: 1, 381647: 1}
	if !m.ObserveRetainedPartyBuild(b, now) || !m.partyBuilds[guid].Since.Equal(b.At) {
		t.Fatal("changed held scan inherited old warm-up")
	}
	// A newer combat-log spec change invalidates the cached selection even
	// across a zone boundary; it cannot come back after the short cache expires.
	m.Builds[guid] = BuildSnapshot{GUID: guid, Source: "combat_log", At: now, SpecID: 253}
	m.ResetLog(now)
	for _, got := range m.Snapshot(now).Builds {
		if got.Source == "inspect" {
			t.Fatal("conflicting cached build survived", got)
		}
	}
	if m.HasInspectedBuild(guid, now) {
		t.Fatal("conflicting cache supplied ability ownership")
	}
}
