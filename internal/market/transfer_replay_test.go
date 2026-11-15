package market

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func marketChunks(id string, data []byte) [][]byte {
	count := (len(data) + 5599) / 5600
	out := make([][]byte, count)
	for i := range out {
		out[i] = append([]byte(fmt.Sprintf("1|%s|%d|%d|%d\n", id, i+1, count, len(data))), data[i*5600:min(len(data), (i+1)*5600)]...)
	}
	return out
}

func TestCompletedMarketPassTailsDoNotFillPendingQueue(t *testing.T) {
	a := NewAssembler()
	for index := 0; index < 30; index++ {
		id := fmt.Sprintf("page-%d", index)
		data, _ := json.Marshal(map[string]string{"id": id, "text": strings.Repeat("x", 9000)})
		chunks := marketChunks(id, data)
		emitted := 0
		for pass := 0; pass < 3; pass++ {
			for _, chunk := range chunks {
				got, err := a.Accept(chunk, fixtureNow.Add(time.Duration(index)*time.Second))
				if err != nil {
					t.Fatal(index, pass, err)
				}
				if got != nil {
					emitted++
					if !bytes.Equal(got, data) {
						t.Fatal("changed data")
					}
				}
			}
			// Screen polling can observe the final painted packet after completion.
			for repeat := 0; repeat < 5; repeat++ {
				if got, err := a.Accept(chunks[len(chunks)-1], fixtureNow); err != nil || got != nil {
					t.Fatal("repeated tail", err)
				}
			}
		}
		if emitted != 1 || len(a.pending) != 0 {
			t.Fatalf("page %d emitted %d with %d orphan assemblies", index, emitted, len(a.pending))
		}
	}
}

func TestCompletedMarketChunkConflictsAndCacheBounds(t *testing.T) {
	a := NewAssembler()
	for i := 0; i < 300; i++ {
		id := fmt.Sprintf("event-%d", i)
		data, _ := json.Marshal(map[string]string{"id": id})
		if _, err := a.Accept(marketChunks(id, data)[0], fixtureNow.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.complete) != 256 || len(a.pending) != 0 {
		t.Fatal("completion cache not bounded", len(a.complete), len(a.pending))
	}
	id := "event-299"
	data, _ := json.Marshal(map[string]string{"id": id})
	bad := marketChunks(id, data)[0]
	bad[len(bad)-1] = '!'
	if _, err := a.Accept(bad, fixtureNow.Add(300*time.Second)); err == nil {
		t.Fatal("conflicting replay accepted")
	}
	good := marketChunks(id, data)[0]
	if got, err := a.Accept(good, fixtureNow.Add(301*time.Second)); err != nil || got != nil {
		t.Fatal("valid replay damaged by conflict", err)
	}
	if got, err := a.Accept(good, fixtureNow.Add(20*time.Minute)); err != nil || got == nil {
		t.Fatal("expired cache blocked full replay", err)
	}
}

func TestIncompleteOldMarketEventsDoNotStarveNewPages(t *testing.T) {
	a := NewAssembler()
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("obscured-%d", i)
		data, _ := json.Marshal(map[string]string{"id": id, "text": strings.Repeat("x", 9000)})
		if _, err := a.Accept(marketChunks(id, data)[0], fixtureNow.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	data := []byte(`{"id":"new-page"}`)
	got, err := a.Accept(marketChunks("new-page", data)[0], fixtureNow.Add(10*time.Second))
	if err != nil || !bytes.Equal(got, data) || len(a.pending) != 7 {
		t.Fatal("lost chunks blocked new page", err, len(a.pending))
	}
}

func TestFailedMarketIngestionRemainsRetryable(t *testing.T) {
	s := fixture(t)
	e := event("retry-ingest", "vendor")
	body, _ := json.Marshal(e)
	chunk := marketChunks(e.ID, body)[0]
	if _, err := s.db.Exec(`CREATE TEMP TRIGGER reject_event BEFORE INSERT ON events BEGIN SELECT RAISE(FAIL,'temporary write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.Accept(chunk, fixtureNow); err == nil {
		t.Fatal("expected ingestion error")
	}
	if len(s.transfer.complete) != 0 {
		t.Fatal("failed event was marked consumed")
	}
	if _, err := s.db.Exec(`DROP TRIGGER reject_event`); err != nil {
		t.Fatal(err)
	}
	if err := s.Accept(chunk, fixtureNow); err != nil {
		t.Fatal("retry failed", err)
	}
	var count int
	s.db.QueryRow(`SELECT count(*) FROM events`).Scan(&count)
	if count != 1 || s.Capture().Error != "" {
		t.Fatal("retry did not persist exactly once", count, s.Capture())
	}
}
