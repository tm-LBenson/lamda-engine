package augmentation

import (
	"strconv"
	"strings"
	"time"
)

const encounterAwardVersion = 2

// Each row describes an explicit player assignment/use aura, not a damage
// tick, cast warning, global raid hit, or assumption about world coordinates.
var encounterObservations = map[string]map[string]string{
	"3470": {"1287434": "target/rend", "1306666": "target/pyre", "1294933": "target/flame", "1300524": "use/coil", "1300521": "use/coil"},
	"3445": {"1284471": "target/blood", "1288260": "target/miasma"},
	"3455": {"1281913": "target/froth", "1295173": "target/infection", "1294994": "target/infection", "1295224": "target/infection"},
	"3497": {"1297625": "target/surprise", "1296092": "target/thud", "1295886": "target/volley", "1295935": "target/volley", "1299854": "use/bounce", "1299855": "use/bounce"},
	"3420": {"1305963": "target/surge", "1285425": "target/crosswinds", "1285453": "target/crosswinds", "1297111": "target/crosswinds", "1297096": "target/crosswinds", "1305621": "target/fury"},
	"3421": {"1290814": "target/ichor", "1303230": "target/torrent"},
	"3429": {"1282419": "use/venom_orb", "1310498": "use/mutagen_orb", "1283485": "target/guillotine", "1299266": "target/guillotine", "1285911": "target/fixation", "1297445": "target/dreadmarch", "1286901": "target/gloombomb"},
	"3492": {"1295360": "use/egg", "1307612": "use/egg", "1300312": "use/doomscale_egg", "1293046": "target/bite", "1305163": "target/sting", "1301118": "target/fangs"},
}

type venomAwardState struct {
	stacks int64
	known  bool
	nineAt time.Time
}
type encounterAwardState struct {
	observed             map[string]time.Time
	firstSeen, lastSeen  map[string]time.Time
	dead, died           map[string]bool
	venom                map[string]*venomAwardState
	zeroPending, feasted map[string]time.Time
	helix, crates        map[string]int64
	helixAt              map[string]time.Time
	venomSeen, finalized bool
}

func (d *damageSession) encounter() *encounterAwardState {
	if d.encounterState == nil {
		d.encounterState = &encounterAwardState{
			observed: map[string]time.Time{}, firstSeen: map[string]time.Time{}, lastSeen: map[string]time.Time{}, dead: map[string]bool{}, died: map[string]bool{},
			venom: map[string]*venomAwardState{}, zeroPending: map[string]time.Time{}, feasted: map[string]time.Time{}, helix: map[string]int64{}, crates: map[string]int64{}, helixAt: map[string]time.Time{},
		}
	}
	return d.encounterState
}

func auraStacks(f []string) (int64, bool) {
	if len(f) <= 13 {
		return 0, false
	}
	n, err := strconv.ParseInt(f[13], 10, 64)
	return n, err == nil && n >= 0 && n <= 100
}

func (s *Store) trackEncounterAwards(seg *segment, f []string) {
	scope := mechanicScope(seg)
	if scope == "" || len(f) < 9 || encounterObservations[scope] == nil {
		return
	}
	d := seg.damage
	st := d.encounter()
	for _, p := range []struct{ id, name, flags string }{{f[1], f[2], f[3]}, {f[5], f[6], f[7]}} {
		if player(p.id) && friendlyGroup(p.flags) {
			if st.firstSeen[p.id].IsZero() {
				st.firstSeen[p.id] = s.last
			}
			st.lastSeen[p.id] = s.last
			d.players[p.id] = true
			s.party[p.id] = p.name
		}
	}
	if !player(f[5]) || !friendlyGroup(f[7]) {
		return
	}
	id := f[5]
	if f[0] == "UNIT_DIED" {
		st.dead[id] = true
		st.died[id] = true
		delete(st.zeroPending, id)
		if v := st.venom[id]; v != nil {
			v.nineAt = time.Time{}
			v.known = false
		}
		return
	}
	if f[0] == "SPELL_RESURRECT" {
		delete(st.dead, id)
		return
	}
	if len(f) < 13 {
		return
	}
	spell, event := f[9], f[0]
	if event == "SPELL_AURA_APPLIED" {
		if key := encounterObservations[scope][spell]; key != "" {
			stamp := key + "/" + id
			last := st.observed[stamp]
			if last.IsZero() || s.last.Sub(last) >= time.Second {
				d.addMechanic(key, id, 1)
				st.observed[stamp] = s.last
				if strings.HasPrefix(key, "target/") {
					d.addMechanic("target/all", id, 1)
				}
			}
		}
	}
	if seg.id == "3445" && spell == "1284590" {
		switch event {
		case "SPELL_AURA_APPLIED", "SPELL_AURA_APPLIED_DOSE":
			n, ok := auraStacks(f)
			if event == "SPELL_AURA_APPLIED" && len(f) == 13 {
				n, ok = 1, true
			}
			if ok {
				st.helix[id] = n
				st.helixAt[id] = s.last
			} else {
				delete(st.helix, id)
			}
		case "SPELL_AURA_REMOVED":
			if st.helix[id] == 4 && s.last.Sub(st.helixAt[id]) <= time.Second && !st.dead[id] {
				d.addMechanic("puzzle/four", id, 1)
			}
			delete(st.helix, id)
		}
	}
	// Mythic Splinters hits the entire raid. Its recipient cannot identify the
	// box opener, unlike the personal Normal/Heroic/LFR application.
	if seg.id == "3497" && spell == "1308853" && (seg.difficulty == "14" || seg.difficulty == "15" || seg.difficulty == "17") {
		switch event {
		case "SPELL_AURA_APPLIED", "SPELL_AURA_APPLIED_DOSE", "SPELL_AURA_REMOVED_DOSE":
			n, ok := auraStacks(f)
			if event == "SPELL_AURA_APPLIED" && len(f) == 13 {
				n, ok = 1, true
			}
			if ok {
				if event != "SPELL_AURA_REMOVED_DOSE" && n > st.crates[id] {
					d.addMechanic("use/crate", id, n-st.crates[id])
				}
				st.crates[id] = n
			}
		case "SPELL_AURA_REMOVED":
			delete(st.crates, id)
		}
	}
	if seg.id != "3421" {
		return
	}
	if spell == "1310096" && event == "SPELL_AURA_APPLIED" {
		st.feasted[id] = s.last
		if at := st.zeroPending[id]; !at.IsZero() && mechanicTimeGap(s.last.Sub(at)) <= 250*time.Millisecond && st.venom[id].known && st.venom[id].stacks == 1 {
			st.venom[id].stacks = 0
			st.venom[id].known = true
			delete(st.zeroPending, id)
		}
	}
	if spell != "1290336" {
		return
	}
	st.venomSeen = true
	v := st.venom[id]
	if v == nil {
		v = &venomAwardState{}
		st.venom[id] = v
	}
	switch event {
	case "SPELL_AURA_APPLIED", "SPELL_AURA_APPLIED_DOSE", "SPELL_AURA_REMOVED_DOSE":
		n, ok := auraStacks(f)
		if event == "SPELL_AURA_APPLIED" && len(f) == 13 {
			n, ok = 1, true
		}
		v.known = ok && n <= 10
		if v.known {
			v.stacks = n
			if n == 9 && v.nineAt.IsZero() {
				v.nineAt = s.last
			}
		}
		delete(st.zeroPending, id)
	case "SPELL_AURA_REMOVED":
		// The kill/wipe/death cleanup also removes this aura. Zero requires
		// the matching Feast, in either log order; cleanup is never a cleanse.
		if at := st.feasted[id]; !at.IsZero() && mechanicTimeGap(s.last.Sub(at)) <= 250*time.Millisecond && v.known && v.stacks == 1 {
			v.stacks = 0
			v.known = true
		} else {
			st.zeroPending[id] = s.last
		}
	}
}

func (s *Store) finishEncounterAwards(seg *segment, at time.Time, success bool) {
	st := seg.damage.encounterState
	if seg.kind != "boss" || st == nil || st.finalized {
		return
	}
	st.finalized = true
	if !success {
		return
	}
	for id, first := range st.firstSeen {
		// A kill-only positive result needs evidence that this player was in
		// the encounter throughout. Late arrivals and stale rows do not qualify.
		if st.dead[id] || first.Sub(seg.start) > 30*time.Second || at.Sub(st.lastSeen[id]) > 10*time.Second {
			continue
		}
		if !st.died[id] {
			if n := seg.damage.mechanicTotals["target/all"][id]; n >= 3 {
				seg.damage.addMechanic("finish/assignment_survivor", id, n)
			}
			if seg.id == "3420" {
				if n := seg.damage.mechanicTotals["target/crosswinds"][id]; n >= 2 {
					seg.damage.addMechanic("finish/crosswinds_survived", id, n)
				}
				if n := seg.damage.mechanicTotals["target/surge"][id]; n >= 2 && seg.damage.mechanicTotals["bomb/drop"][id] == n {
					seg.damage.addMechanic("finish/bombs_delivered", id, n)
				}
			}
		}
		if seg.id != "3421" || !st.venomSeen {
			continue
		}
		v := st.venom[id]
		// Preserve the pre-cleanup value only at the actual kill boundary.
		// An unexplained earlier removal leaves the current stack count unknown.
		if pending := st.zeroPending[id]; v != nil && !pending.IsZero() && at.Sub(pending) > 250*time.Millisecond {
			v.known = false
		}
		if v != nil && v.known && v.stacks < 10 && !v.nineAt.IsZero() && at.Sub(v.nineAt) >= time.Second {
			seg.damage.addMechanic("finish/venom_nine_survived", id, 1)
		}
		if v != nil && v.known && v.stacks == 9 {
			seg.damage.addMechanic("finish/venom_nine", id, 1)
		}
		if !st.died[id] && (v == nil || v.known && v.stacks == 0) {
			seg.damage.addMechanic("finish/venom_zero", id, 1)
		}
	}
}
