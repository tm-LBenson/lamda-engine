package augmentation

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestReviewedRecoveryAddsCompletedKeyWithoutReplacingLiveIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "original.txt")
	damage := actor("SPELL_DAMAGE", aug, "Creature-1", 1)
	damage[3] = "0x511"
	damage[7] = "0xa48"
	for i := 0; i < 19; i++ {
		damage = append(damage, "0")
	}
	damage = append(damage, "50", "50", "-1", "12", "0", "0", "0", "nil", "nil", "nil", "ST")
	lines := []string{record(0, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5"), record(1, "CHALLENGE_MODE_START", "Test dungeon", "123", "1", "10"), record(2, damage...), record(3, "COMBAT_LOG_VERSION", "22", "ADVANCED_LOG_ENABLED", "1", "BUILD_VERSION", "12.1.5"), record(4, damage...), record(10, "CHALLENGE_MODE_END", "123", "1", "10000")}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s := ready()
	s.own = "Player-Other-Character"
	start(s, 20)
	oldActive := s.active["boss"]
	m, err := s.recoverSummary(context.Background(), path, "key", aug, nil)
	if err != nil || m == nil || m.RawTotal != 100 || m.Seconds != 9 || !m.Valid {
		t.Fatal(m, err)
	}
	if s.own != "Player-Other-Character" || s.active["boss"] != oldActive || len(s.damageMeters) != 1 {
		t.Fatal("live collector changed")
	}
	if _, err = s.recoverSummary(context.Background(), path, "key", aug, m); err != nil || len(s.damageMeters) != 1 {
		t.Fatal("duplicate recovery", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.recoverSummary(ctx, path, "key", aug, nil); err == nil {
		t.Fatal("cancelled replay ran")
	}
}

func TestRecoverOriginalLogAudit(t *testing.T) {
	path := os.Getenv("LAMDA_RECOVER_LOG")
	if path == "" {
		t.Skip("local replay not requested")
	}
	owner := os.Getenv("LAMDA_RECOVER_OWNER")
	s := ready()
	started := time.Now()
	m, err := s.recoverSummary(context.Background(), path, "key", owner, nil)
	if err != nil || m == nil || !m.Valid {
		f, _ := os.Open(path)
		defer f.Close()
		scan := bufio.NewScanner(f)
		scan.Buffer(make([]byte, 64<<10), 2<<20)
		r := New("")
		r.SetEnabled(true)
		for scan.Scan() {
			old := r.active["key"]
			last := r.last
			r.Ingest(scan.Text(), time.Now())
			if old != nil && r.active["key"] == nil {
				t.Log("KEY CLOSED/LOST", old.name, "last", last, "status", r.status, "line", scan.Text()[:min(180, len(scan.Text()))])
			}
		}
		for _, v := range r.damageMeters {
			if v.Kind == "key" {
				t.Log("KEY", v.Name, v.ID, v.Owner, v.Valid)
			}
		}
		t.Fatal(m, err)
	}
	t.Log(m.ID, m.Name, m.RawTotal, m.AdjustedTotal, m.Rows, "elapsed", time.Since(started))
	c := readSummaryCounts(context.Background(), path, *m)
	if !c.Known {
		t.Fatal("unknown utility coverage")
	}
	if m.Utility == nil || !m.Utility.Known || !reflect.DeepEqual(m.Utility.Dispels, c.Dispels) || !reflect.DeepEqual(m.Utility.Deaths, c.Deaths) {
		t.Fatalf("stream utility differs from independent original-log scan: got %+v want %+v", m.Utility, c)
	}
	want := makeChatSummary(*m, c)
	started = time.Now()
	got, err := s.SummaryForPlayerWhenReady(context.Background(), "key", 0, owner)
	if err != nil || got.Text != want.Text || !got.MetricsKnown {
		t.Fatal("fast completed report mismatch", got, err)
	}
	t.Log("completed report prepared in", time.Since(started))
	t.Log(got.Text)
}
