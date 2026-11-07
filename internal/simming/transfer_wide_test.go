package simming

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/lookup"
)

// The real Lua sender must produce a decodable request when bags contain two
// identical copies. The profile retains multiplicity; metadata is shared.
func TestLuaDuplicateUpgradeSnapshotRoundTrip(t *testing.T) {
	path := os.Getenv("LAMDA_SIM_DUPLICATE_SNAPSHOT")
	if path == "" {
		t.Skip("set LAMDA_SIM_DUPLICATE_SNAPSHOT to the Lua regression fixture")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := DecodeSnapshot(data, "retail")
	if err != nil || snapshot == nil {
		t.Fatal("real Lua export rejected", err)
	}
	if len(snapshot.Upgrades) != 4 || strings.Count(snapshot.Profile, "# head=,id=201,bonus_id=1/2") != 2 || snapshot.GameRequest.Task != "analysis" {
		t.Fatal("duplicate item identity or request lost")
	}
	a := Assembler{Client: "retail"}
	var got *Snapshot
	for _, packet := range wideChunkPackets(t, snapshot) {
		got, err = a.Accept(lookup.DecodeSample(packet).Sim, time.Unix(snapshot.CapturedAt, 0))
		if err != nil {
			t.Fatal(err)
		}
	}
	if got == nil || got.ID != snapshot.ID {
		t.Fatal("request did not survive real optical packet path")
	}
}

func wideChunkPackets(t *testing.T, s *Snapshot) [][]byte {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	count := (len(data) + WideChunkSize - 1) / WideChunkSize
	chunks := make([][]byte, 0, count)
	for i := 0; i < count; i++ {
		body := fmt.Sprintf("2|%s|%s|%d|%d|%d|%d|%d\n", s.ID, s.Client, s.CapturedAt, len(data), i+1, count, crc32.ChecksumIEEE(data)) + string(data[i*WideChunkSize:min((i+1)*WideChunkSize, len(data))])
		if len(body) > lookup.SimBridgeBytes-10 {
			t.Fatal("wide header exceeds optical packet")
		}
		packet := make([]byte, lookup.SimBridgeBytes)
		copy(packet, "LUS2")
		binary.BigEndian.PutUint16(packet[4:6], uint16(len(body)))
		binary.BigEndian.PutUint32(packet[6:10], crc32.ChecksumIEEE([]byte(body)))
		copy(packet[10:], body)
		chunks = append(chunks, packet)
	}
	return chunks
}

func TestWideTransferReorderingLossDuplicatesAndLegacy(t *testing.T) {
	now := time.Unix(1800000000, 0)
	for _, client := range []string{"retail", "forever"} {
		t.Run(client, func(t *testing.T) {
			s := fixture(now)
			s.Client = client
			if client == "forever" {
				s.Interface = 16001
			}
			s.Profile += strings.Repeat("# UTF-8: é猫\n", 6500)
			packets := wideChunkPackets(t, s)
			if len(packets) < 10 || len(packets) >= len(chunkPackets(t, s)) {
				t.Fatal("wide format does not increase throughput")
			}
			a := Assembler{Client: client}
			for i := len(packets) - 1; i > 0; i-- {
				body := lookup.DecodeSample(packets[i]).Sim
				for repeat := 0; repeat < 2; repeat++ {
					got, err := a.Accept(body, now)
					if err != nil || got != nil {
						t.Fatalf("incomplete/duplicate: %v %v", got, err)
					}
				}
			}
			first := lookup.DecodeSample(packets[0])
			if first.Error != "" || first.Event.ID != "" || first.Frames != nil || first.Kick != nil {
				t.Fatal("wide sim leaked into tracking")
			}
			got, err := a.Accept(first.Sim, now)
			if err != nil || got == nil || got.Profile != s.Profile {
				t.Fatalf("wide reassembly: %v %v", got, err)
			}
			if got, err = a.Accept(first.Sim, now); err != nil || got != nil {
				t.Fatal("completed wide replay accepted")
			}
			legacy := Assembler{Client: client}
			for _, packet := range chunkPackets(t, s) {
				got, err = legacy.Accept(lookup.DecodeSample(packet).Sim, now)
				if err != nil {
					t.Fatal(err)
				}
			}
			if got == nil || got.Profile != s.Profile {
				t.Fatal("legacy support regressed")
			}
		})
	}
}

func TestWideTransferLimitsIsolationAndVersionConflict(t *testing.T) {
	now := time.Unix(1800000000, 0)
	s := fixture(now)
	s.ID = strings.Repeat("I", 96)
	raw, _ := json.Marshal(s)
	s.Profile += strings.Repeat("x", MaxSnapshot-len(raw))
	raw, _ = json.Marshal(s)
	if len(raw) != MaxSnapshot {
		t.Fatal("boundary fixture invalid")
	}
	packets := wideChunkPackets(t, s)
	a := Assembler{Client: "retail"}
	var got *Snapshot
	var err error
	for _, p := range packets {
		got, err = a.Accept(lookup.DecodeSample(p).Sim, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	if got == nil {
		t.Fatal("maximum allowed snapshot rejected")
	}
	s.Profile += "x"
	tooBig := lookup.DecodeSample(wideChunkPackets(t, s)[0]).Sim
	if _, err = (&Assembler{Client: "retail"}).Accept(tooBig, now); err == nil {
		t.Fatal("oversized v2 accepted")
	}
	first := lookup.DecodeSample(packets[0]).Sim
	for _, tc := range []struct {
		client string
		at     time.Time
	}{{"forever", now}, {"retail", now.Add(TransferTTL + time.Second)}, {"retail", now.Add(-31 * time.Second)}} {
		if _, err = (&Assembler{Client: tc.client}).Accept(first, tc.at); err == nil {
			t.Fatal("foreign or expired/future v2 accepted")
		}
	}
	// A request may retransmit within one protocol, but switching its version
	// while it is incomplete must never combine the two byte layouts.
	mixed := fixture(now)
	mixed.Profile += strings.Repeat("# more data\n", 1000)
	legacyFirst := lookup.DecodeSample(chunkPackets(t, mixed)[0]).Sim
	wideFirst := lookup.DecodeSample(wideChunkPackets(t, mixed)[0]).Sim
	for _, order := range [][2][]byte{{legacyFirst, wideFirst}, {wideFirst, legacyFirst}} {
		a = Assembler{Client: "retail"}
		if _, err = a.Accept(order[0], now); err != nil {
			t.Fatal(err)
		}
		if _, err = a.Accept(order[1], now); err == nil || !strings.Contains(err.Error(), "metadata") {
			t.Fatalf("mixed version accepted: %v", err)
		}
	}
	a = Assembler{Client: "retail"}
	if _, err = a.Accept(first, now); err != nil {
		t.Fatal(err)
	}
	bad := append([]byte(nil), first...)
	bad[len(bad)-1] ^= 1
	if _, err = a.Accept(bad, now); err == nil {
		t.Fatal("conflicting v2 chunk accepted")
	}
	for _, body := range [][]byte{bytes.Replace(first, []byte("2|"), []byte("3|"), 1), bytes.Replace(first, []byte("|1|"), []byte("|0|"), 1)} {
		if _, err = (&Assembler{Client: "retail"}).Accept(body, now); err == nil {
			t.Fatal("invalid version/index accepted")
		}
	}
}
