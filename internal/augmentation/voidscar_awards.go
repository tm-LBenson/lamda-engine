package augmentation

import (
	"strings"
	"time"
)

const voidscarKeyScope = "key:2923"

var dungeonAwardScopes = map[string]string{"2923": voidscarKeyScope, "1877": templeKeyScope}

func hasDungeonAwards(m DamageMeter) bool {
	return m.Kind == "key" && dungeonAwardScopes[encounterID(m)] != ""
}

func mechanicScope(seg *segment) string {
	if seg.kind == "boss" {
		return seg.id
	}
	if seg.kind == "key" {
		return dungeonAwardScopes[seg.id]
	}
	return ""
}
func awardScope(m DamageMeter) string {
	if m.Kind == "key" {
		if scope := dungeonAwardScopes[encounterID(m)]; scope != "" {
			return scope
		}
	}
	return encounterID(m)
}
func voidscarBoss(id string) bool { return id == "3285" || id == "3286" || id == "3287" }

// Definitions reviewed against DBM-Party-Midnight (20261026), the dungeon
// journal, and the 2026-10-29 original key. An unavoidable raid pulse, a tank
// buster, or a selected player's own mandatory line hit is not avoidable damage.
var voidscarBossAwards = map[string][]mechanicAward{
	"3285": {
		{label: "Orbital Footwork", unit: "deathless Taz'Rah kills without tracked Fissure/orb damage", joke: "Your arena ticket came with floor awareness.", keys: []string{"finish/clean_taz"}},
		{label: "Dashing Through Danger", unit: "mechanic assignments survived on a deathless kill", joke: "Still taking calls. Still standing.", keys: []string{"finish/assignment_survivor"}},
		{label: "Rent in the Rift", unit: "Void Fissure damage", joke: "That puddle is charging by the second.", keys: []string{"avoid/1296967"}, damage: true, banter: true},
		{label: "Orb Collector", unit: "Dark Bloom orb damage", joke: "These collectibles do not go in your bags.", keys: []string{"avoid/1300262"}, damage: true, banter: true},
		{label: "Lane Invader", unit: "confirmed extra Nether Dash damage", joke: "Your own lane was apparently not enough.", keys: []string{"avoid/extra_dash"}, damage: true, banter: true},
		{label: "Reserved Lane", unit: "explicit Nether Dash selections", joke: "The shades keep reserving your lane.", keys: []string{"target/dash"}, minimum: 3, banter: true},
		{label: "Shard Subscription", unit: "Ethereal Shards applications", joke: "The free trial was apparently non-refundable.", keys: []string{"aura/1225011"}, minimum: 3, banter: true},
	},
	"3286": {
		{label: "Creeper Keeper", unit: "damage to Toxic Creepers", joke: "Actually changed targets. Revolutionary.", keys: []string{"work/239070"}, damage: true},
		{label: "Antivenom Clerk", unit: "Mind-Numbing Poison dispels", joke: "Haste restored. Paperwork pending.", keys: []string{"dispel/1263971"}},
		{label: "Clawback Department", unit: "Hulking Claw dispels", joke: "The tank would like their health bar back.", keys: []string{"dispel/1222642"}},
		{label: "Clean Boots", unit: "deathless Atroxus kills without tracked pool or breath damage", joke: "Left the poison where you found it.", keys: []string{"finish/clean_atrox"}},
		{label: "Pool Membership", unit: "Poison Pool damage", joke: "Premium access to the wrong swimming club.", keys: []string{"avoid/1222484"}, damage: true, banter: true},
		{label: "Blank Stare", unit: "Mind-Numbing Poison applications", joke: "Your haste went out for a walk without you.", keys: []string{"aura/1263971"}, banter: true},
		{label: "Breath Mint Needed", unit: "Noxious Breath damage", joke: "Requested a closer look at the dragon's dental plan.", keys: []string{"avoid/1222724"}, damage: true, banter: true},
		{label: "Creeper's Chew Toy", unit: "Sickening Bite applications", joke: "The kiting lesson included a practical demonstration.", keys: []string{"aura/1282892"}, banter: true},
		{label: "Breath of Bad Luck", unit: "explicit Noxious Breath selections", joke: "Atroxus keeps trying to have a conversation.", keys: []string{"target/breath"}, minimum: 3, banter: true},
	},
	"3287": {
		{label: "Clean Orbit", unit: "deathless Charonus kills without tracked singularity damage or Atomized", joke: "Kept your sightseeing outside the event horizon.", keys: []string{"finish/clean_char"}},
		{label: "Orb Wrangler", unit: "Gravitic Orbs destroyed by Unstable Singularity", joke: "Returned every parcel to the void.", keys: []string{"orb/clear"}},
		{label: "Singularity Vacation", unit: "Atomized applications", joke: "Fifteen seconds of all-inclusive nothing.", keys: []string{"aura/1310026"}, banter: true},
		{label: "Event Horizon Resident", unit: "singularity ground-effect damage", joke: "Gravity was a very persuasive landlord.", keys: []string{"avoid/1248130"}, damage: true, banter: true},
		{label: "Mass Appeal", unit: "Condensed Mass stack gains", joke: "An impressive amount of emotional baggage.", keys: []string{"exposure/mass"}, minimum: 10, banter: true},
		{label: "Orbs Know My Address", unit: "distinct Gravitic Orb fixates", joke: "Your forwarding address has been leaked.", keys: []string{"target/orb"}, minimum: 3, banter: true},
		{label: "Wrong Side of the Wave", unit: "Dark Waves damage on a known non-tank", joke: "Front-row tickets were a questionable upgrade.", keys: []string{"avoid/dark_waves"}, damage: true, banter: true},
		{label: "Cascade Collector", unit: "Void Cascade projectile damage", joke: "The souvenir stream was not meant to be caught.", keys: []string{"avoid/1222755"}, damage: true, banter: true},
		{label: "Cascade Courier", unit: "Void Cascade assignments", joke: "Charonus keeps addressing the stream to you.", keys: []string{"target/cascade"}, minimum: 3, banter: true},
		{label: "Singularity Scheduler", unit: "Unstable Singularity marker assignments", joke: "Charonus keeps inviting you to make space.", keys: []string{"target/singularity"}, minimum: 3, banter: true},
	},
}

var voidscarTrashAwards = []mechanicAward{
	{label: "Volume Control", unit: "Shadowbolt Volley interrupts", joke: "Muted the arena's least popular group chat.", keys: []string{"interrupt/1299938"}},
	{label: "Morale Officer", unit: "Demoralizing Shout interrupts", joke: "Negativity has been successfully moderated.", keys: []string{"interrupt/1298899"}},
	{label: "Sandman Silencer", unit: "Violent Sand interrupts", joke: "No sand in this sandwich.", keys: []string{"interrupt/1249621"}},
	{label: "Indoor Voices", unit: "Mad Shriek interrupts", joke: "The neighbours appreciate your service.", keys: []string{"interrupt/1233398"}},
	{label: "No Refunds", unit: "Mending Void interrupts", joke: "Their health bar was not getting a refund.", keys: []string{"interrupt/1310324"}},
	{label: "Armor Warranty", unit: "Melt Armor dispels", joke: "Saved the tank's remaining trousers.", keys: []string{"dispel/1250043"}},
	{label: "Poison Control", unit: "Corrosive Essence dispels", joke: "Another call handled by the helpline.", keys: []string{"dispel/1289258", "dispel/1289265"}},
	{label: "Anger Management", unit: "Bolster/Bloodsurge removals", joke: "Asked the local wildlife to use its inside voice.", keys: []string{"purge/1310319", "purge/1254826"}},
	{label: "Totem Deletion Service", unit: "damage to Magma Totems", joke: "A small target with a large customer-service problem.", keys: []string{"work/248666"}, damage: true},
	{label: "Disruption Dispatched", unit: "damage to Orbs of Disruption", joke: "Priority parcel removed from circulation.", keys: []string{"work/272930"}, damage: true},
	{label: "Landing Crew", unit: "Sky Strike soaks", joke: "Volunteered as the arena's landing gear.", keys: []string{"soak/Sky Strike"}},
	{label: "Null and Void Subscription", unit: "Null Eruption assignments", joke: "The renewal notice was also a puddle.", keys: []string{"target/null"}, minimum: 3, banter: true},
	{label: "Leap Year", unit: "Savage Leap bleed applications", joke: "Somehow every year is your year.", keys: []string{"aura/1267894"}, minimum: 3, banter: true},
	{label: "Spotlight Addict", unit: "Void Beam assignments", joke: "The spotlight has terrible bedside manners.", keys: []string{"target/beam"}, minimum: 3, banter: true},
}

func init() {
	for _, id := range []string{"3285", "3286", "3287", voidscarKeyScope} {
		bossMechanics[id] = bossMechanicRules{good: names(), bad: names()}
		encounterObservations[id] = map[string]string{}
	}
	bossMechanics["3285"].bad["1296967"] = true
	bossMechanics["3285"].bad["1300262"] = true
	bossMechanics["3286"].bad["1222484"] = true
	bossMechanics["3286"].bad["1222724"] = true
	bossMechanics["3287"].bad["1248130"] = true
	bossMechanics["3287"].bad["1222755"] = true
	encounterObservations["3287"]["1282770"] = "target/singularity"
	encounterObservations["3287"]["1227247"] = "target/cascade"
	for _, id := range []string{"3285", "3286", "3287"} {
		defs := voidscarBossAwards[id]
		for i := range defs {
			defs[i].minimumVersion = 2
		}
		encounterAwards[id] = defs
		for spell := range bossMechanics[id].bad {
			bossMechanics[voidscarKeyScope].bad[spell] = true
		}
		for spell, key := range encounterObservations[id] {
			encounterObservations[voidscarKeyScope][spell] = key
		}
	}
	for i := range voidscarTrashAwards {
		voidscarTrashAwards[i].minimumVersion = 2
	}
	// Interleave the bosses and trash so an end-of-key recap does not spend
	// every mechanic slot on the first boss. Keep room for core meter awards.
	groups := [][]mechanicAward{encounterAwards["3285"], encounterAwards["3286"], encounterAwards["3287"], voidscarTrashAwards}
	limit := 0
	for _, defs := range groups {
		limit = max(limit, len(defs))
	}
	for i := 0; i < limit; i++ {
		for _, defs := range groups {
			if i < len(defs) && defs[i].label != "Dashing Through Danger" {
				encounterAwards[voidscarKeyScope] = append(encounterAwards[voidscarKeyScope], defs[i])
			}
		}
	}
	bossMechanics[voidscarKeyScope].good["Sky Strike"] = true
	encounterObservations[voidscarKeyScope]["1299913"] = "target/null"
	encounterObservations[voidscarKeyScope]["1300138"] = "target/beam"
}

type voidscarAwardState struct {
	orbOwners  map[string]string
	mass       map[string]int64
	dashCasts  map[string]dashSelection
	dashHits   map[string][]dashHit
	dashSeen   map[string]bool
	orbCleared map[string]bool
}
type dashSelection struct {
	target string
	at     time.Time
}
type dashHit struct {
	target string
	at     time.Time
	amount int64
}

func (d *damageSession) voidscar() *voidscarAwardState {
	if d.voidscarState == nil {
		d.voidscarState = &voidscarAwardState{orbOwners: map[string]string{}, mass: map[string]int64{}, dashCasts: map[string]dashSelection{}, dashHits: map[string][]dashHit{}, dashSeen: map[string]bool{}, orbCleared: map[string]bool{}}
	}
	return d.voidscarState
}
func npcID(guid string) string {
	p := strings.Split(guid, "-")
	if len(p) > 5 && (p[0] == "Creature" || p[0] == "Vehicle") {
		return p[5]
	}
	return ""
}
func (d *damageSession) unknownMechanic(key string) {
	if d.unknownMechanics == nil {
		d.unknownMechanics = map[string]bool{}
	}
	d.unknownMechanics[key] = true
}

func (s *Store) trackVoidscarAwards(seg *segment, f []string) {
	scope := mechanicScope(seg)
	if !voidscarBoss(scope) && scope != voidscarKeyScope || len(f) < 12 {
		return
	}
	d := seg.damage
	// The original log uses a null caster and a party-owned orb destination.
	// This exact instakill effect proves a singularity collapse; ordinary
	// death/despawn/aura removal does not. Attribute it through the orb GUID.
	if (scope == "3287" || scope == voidscarKeyScope) && f[0] == "SPELL_INSTAKILL" && f[9] == "1263984" && npcID(f[5]) == "255001" {
		st := d.voidscar()
		owner := st.orbOwners[f[5]]
		if owner != "" && !st.orbCleared[f[5]] {
			d.addMechanic("orb/clear", owner, 1)
			st.orbCleared[f[5]] = true
		}
		return
	}
	if (f[0] == "SPELL_DISPEL" || f[0] == "SPELL_STOLEN") && len(f) > 12 && friendlyGroup(f[3]) && !friendlyGroup(f[7]) && npcID(f[5]) != "" {
		key := "purge/" + f[12]
		if wantedMechanic(scope, key) {
			d.addMechanic(key, f[1], 1)
		}
	}
	if friendlyGroup(f[3]) && !friendlyGroup(f[7]) {
		key := "work/" + npcID(f[5])
		if wantedMechanic(scope, key) {
			if base, support, ok := damagePayload(f); ok && !support {
				if n, reason := recordedDamageAmount(f, base); reason == "" {
					d.addMechanic(key, f[1], n)
				} else if reason != "not_damage" {
					d.unknownMechanic(key)
				}
			}
		}
	}
	ownedOrb := (scope == "3287" || scope == voidscarKeyScope) && f[9] == "1263983" && npcID(f[1]) == "255001"
	if !player(f[5]) || !friendlyGroup(f[7]) || friendlyGroup(f[3]) && !ownedOrb {
		return
	}
	if f[0] == "SPELL_CAST_SUCCESS" {
		key := ""
		if f[9] == "1222100" && (scope == "3285" || scope == voidscarKeyScope) {
			key = "target/dash"
			st := d.voidscar()
			selection := dashSelection{f[5], s.last}
			st.dashCasts[f[1]] = selection
			for _, hit := range st.dashHits[f[1]] {
				if mechanicTimeGap(s.last.Sub(hit.at)) <= time.Second {
					d.confirmDashHit(hit, selection)
				} else {
					d.unknownMechanic("avoid/extra_dash")
					d.mechanicsInvalid = true
				}
			}
			delete(st.dashHits, f[1])
		}
		if f[9] == "1222721" && (scope == "3286" || scope == voidscarKeyScope) {
			key = "target/breath"
		}
		if key != "" && npcID(f[1]) != "" {
			st := d.encounter()
			stamp := key + "/" + f[1] + "/" + f[5]
			last := st.observed[stamp]
			if last.IsZero() || s.last.Sub(last) >= time.Second {
				d.addMechanic(key, f[5], 1)
				d.addMechanic("target/all", f[5], 1)
				st.observed[stamp] = s.last
			}
		}
	}
	if f[0] == "SPELL_DAMAGE" && f[9] == "1222103" && (scope == "3285" || scope == voidscarKeyScope) {
		if n, reason := recordedDamageAmount(f, 12); reason == "" {
			st := d.voidscar()
			stamp := f[1] + "/" + f[5] + "/" + s.last.Format(time.RFC3339Nano)
			if !st.dashSeen[stamp] {
				st.dashSeen[stamp] = true
				hit := dashHit{f[5], s.last, n}
				selection := st.dashCasts[f[1]]
				if !selection.at.IsZero() && mechanicTimeGap(s.last.Sub(selection.at)) <= time.Second {
					d.confirmDashHit(hit, selection)
				} else {
					st.dashHits[f[1]] = append(st.dashHits[f[1]], hit)
				}
			}
		} else if reason != "not_damage" {
			d.unknownMechanic("avoid/extra_dash")
			d.mechanicsInvalid = true
		}
	}
	if scope != "3287" && scope != voidscarKeyScope {
		return
	}
	if f[9] == "1311923" && s.specs[f[5]] > 0 && !tankSpec(s.specs[f[5]]) {
		if base, support, ok := damagePayload(f); ok && !support {
			if n, reason := recordedDamageAmount(f, base); reason == "" {
				d.addMechanic("avoid/dark_waves", f[5], n)
				d.avoidable[f[5]] += n
				d.avoidableHits[f[5]]++
			} else if reason != "not_damage" {
				d.unknownMechanic("avoid/dark_waves")
			}
		}
	}
	if f[9] != "1263983" || len(f) < 13 {
		return
	}
	st := d.voidscar()
	old := st.mass[f[5]]
	switch f[0] {
	case "SPELL_AURA_APPLIED", "SPELL_AURA_APPLIED_DOSE", "SPELL_AURA_REMOVED_DOSE":
		n, ok := auraStacks(f)
		if f[0] == "SPELL_AURA_APPLIED" && len(f) == 13 {
			n, ok = 1, true
		}
		if !ok {
			d.unknownMechanic("exposure/mass")
			return
		}
		if f[0] != "SPELL_AURA_REMOVED_DOSE" && n > old {
			d.addMechanic("exposure/mass", f[5], n-old)
		}
		st.mass[f[5]] = n
		if npcID(f[1]) == "255001" && st.orbOwners[f[1]] == "" {
			st.orbOwners[f[1]] = f[5]
			d.addMechanic("target/orb", f[5], 1)
		}
	case "SPELL_AURA_REMOVED":
		delete(st.mass, f[5])
	}
}

func (d *damageSession) confirmDashHit(hit dashHit, selection dashSelection) {
	if hit.target == selection.target {
		return
	}
	d.addMechanic("avoid/extra_dash", hit.target, hit.amount)
	d.avoidable[hit.target] += hit.amount
	d.avoidableHits[hit.target]++
}

func (s *Store) finishVoidscarAwards(seg *segment, at time.Time, success bool) {
	if st := seg.damage.voidscarState; st != nil {
		for _, hits := range st.dashHits {
			if len(hits) > 0 {
				seg.damage.unknownMechanic("avoid/extra_dash")
				seg.damage.mechanicsInvalid = true
			}
		}
	}
	if seg.kind != "boss" || !voidscarBoss(seg.id) || !success || at.Sub(seg.start) < 20*time.Second {
		return
	}
	d := seg.damage
	st := d.encounterState
	if st == nil || d.mechanicsInvalid {
		return
	}
	key := map[string]string{"3285": "finish/clean_taz", "3286": "finish/clean_atrox", "3287": "finish/clean_char"}[seg.id]
	for id, first := range st.firstSeen {
		if st.dead[id] || st.died[id] || first.Sub(seg.start) > 30*time.Second || at.Sub(st.lastSeen[id]) > 10*time.Second || d.avoidableHits[id] > 0 {
			continue
		}
		if seg.id == "3287" && d.mechanicTotals["aura/1310026"][id] > 0 {
			continue
		}
		if seg.id == "3286" && d.mechanicTotals["aura/1263971"][id] > 0 {
			continue
		}
		if len(d.unknownMechanics) > 0 {
			continue
		}
		d.addMechanic(key, id, 1)
		if run := s.active["key"]; run != nil && run.id == "2923" && run.start.Before(seg.start) {
			run.damage.addMechanic(key, id, 1)
		}
	}
}
