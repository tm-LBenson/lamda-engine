package augmentation

import (
	"fmt"
	"strings"
	"time"
)

type mechanicAward struct {
	label, unit, joke string
	keys              []string
	banter, damage    bool
	minimum           int64
	extended          bool
	minimumVersion    int
}

// Spell effects are scoped to an encounter. Reviewed DBM definitions provide
// mechanic meaning; original logs establish the actual effect IDs. A targeted
// aura alone never proves a successful bomb drop or a correct raid marker.
var encounterAwards = map[string][]mechanicAward{
	"3470": {
		{label: "Fire Brigade", unit: "Hungering Pyre soaks", joke: "Fire drill attendance: excellent.", keys: []string{"soak/Hungering Pyre"}},
		{label: "Exorcist", unit: "Essence Rend dispels", joke: "Evicted another unwanted spirit.", keys: []string{"dispel/1287434"}},
		{label: "Cultist Cosplayer", unit: "Latent Cultist damage", joke: "The floor accepted your application.", keys: []string{"avoid/1288554"}, banter: true, damage: true},
	},
	"3445": {
		{label: "Hazmat Crew", unit: "Blighted Blood dispels", joke: "Somebody brought gloves.", keys: []string{"dispel/1284471"}},
		{label: "Toxic Taste Tester", unit: "Miasma/Droplet soaks", joke: "A questionable tasting menu.", keys: []string{"soak/Unstable Miasma", "soak/Toxic Droplets"}},
		{label: "Blood Pool Enjoyer", unit: "Blood Venom damage", joke: "The spa was not complimentary.", keys: []string{"avoid/1284210"}, banter: true, damage: true},
	},
	"3455": {
		{label: "Catalyst Crew", unit: "Malignant Catalyst soaks", joke: "Actually showed up for the experiment.", keys: []string{"soak/Malignant Catalyst"}},
		{label: "Antidote Dealer", unit: "Infection/Congealing Bolt dispels", joke: "Keeping the plague off the guest list.", keys: []string{"dispel/1295173", "dispel/1294994", "dispel/1295224", "dispel/1305833"}},
		{label: "Fume Enthusiast", unit: "Virulent Fumes damage", joke: "Deep breaths were a bold choice.", keys: []string{"avoid/1291461"}, banter: true, damage: true},
	},
	"3497": {
		{label: "Shock Absorber", unit: "Mighty Thud soaks", joke: "Volunteered as landing gear.", keys: []string{"soak/Mighty Thud"}},
		{label: "Cold Caller", unit: "Icebound Flames interrupts", joke: "Iku's sales pitch was cancelled.", keys: []string{"interrupt/1286922"}},
		{label: "Icebreaker", unit: "Icebound Flames dispels", joke: "Awkward introduction successfully removed.", keys: []string{"dispel/1286922"}},
		{label: "Spicy Footwork", unit: "Spreading Flames damage", joke: "The dance floor was literally on fire.", keys: []string{"avoid/1297650"}, banter: true, damage: true},
		{label: "Surprise Guest", unit: "Explosive Surprise blasts caught", joke: "Really committed to attending that party.", keys: []string{"hit/1296245"}, banter: true},
	},
	"3420": {
		{label: "Bomb Courier", unit: "Venomous Surge bomb drops", joke: "Shipping department MVP.", keys: []string{"bomb/drop"}},
		{label: "Mutilate Mutual", unit: "Mutilate soaks", joke: "Group damage insurance paid up.", keys: []string{"soak/Mutilate"}},
		{label: "Double Dipper", unit: "Mutilate soaks with an existing Gash", joke: "The second helping was not free.", keys: []string{"fail/double_mutilate"}, banter: true},
		{label: "Puddle Patron", unit: "Caustic Claws damage", joke: "Reserved a table in the splash zone.", keys: []string{"avoid/1305998"}, banter: true, damage: true},
		{label: "Spin Class", unit: "Tempest catches", joke: "Tornado membership renewed.", keys: []string{"aura/1287083"}, banter: true},
	},
	"3421": {
		{label: "All You Can Eat", unit: "Ravenous Feast soaks", joke: "Took the buffet invitation personally.", keys: []string{"soak/Ravenous Feast"}},
		{label: "Stonewall", unit: "Stone Breaker tank soaks", joke: "The wall filed a complaint.", keys: []string{"soak/Stone Breaker"}},
		{label: "Gore Tourist", unit: "Congealed Gore damage", joke: "Visited every puddle on the tour.", keys: []string{"avoid/1292552"}, banter: true, damage: true},
		{label: "Slick Operator", unit: "Noxious Slick damage", joke: "Traction remains a work in progress.", keys: []string{"avoid/1309471"}, banter: true, damage: true},
	},
	"3429": {
		{label: "Guillotine Insurance", unit: "Guillotine soaks", joke: "Headcount maintained.", keys: []string{"soak/Guillotine", "soak/Grim Guillotine"}},
		{label: "Venom Removal Service", unit: "Venomfang dispels", joke: "Poison control finally answered.", keys: []string{"dispel/1306906"}},
		{label: "Axe Collector", unit: "Axegrinder damage", joke: "Caught those with the wrong inventory slot.", keys: []string{"avoid/1285017"}, banter: true, damage: true},
		{label: "Groundskeeper", unit: "Noxious/Defiled Ground damage", joke: "Personally inspected the entire lawn.", keys: []string{"avoid/1283290", "avoid/1298591"}, banter: true, damage: true},
	},
	"3492": {
		{label: "Fang Club", unit: "Serpent's Bite/Spectral Coils soaks", joke: "Membership comes with puncture marks.", keys: []string{"soak/Serpent's Bite", "soak/Spectral Coils"}},
		{label: "Snakebite Specialist", unit: "Acidic Burst dispels", joke: "Kept the venom on a short leash.", keys: []string{"dispel/1301800"}},
		{label: "Fossil Collector", unit: "Calcified Corpse failures", joke: "Added yourself to the museum.", keys: []string{"aura/1306119"}, banter: true},
		{label: "Stone Cold", unit: "Petrifying Sting stuns", joke: "A very convincing statue impression.", keys: []string{"aura/1303414"}, banter: true},
	},
	"3513": {
		{label: "Fear Bouncer", unit: "Overwhelming Fear dispels", joke: "Kith'ix's horror show lost its audience.", keys: []string{"dispel/1304046"}},
	},
}

func (d *damageSession) addMechanic(key, id string, n int64) {
	if n <= 0 {
		return
	}
	if d.mechanicTotals == nil {
		d.mechanicTotals = map[string]map[string]int64{}
	}
	if d.mechanicTotals[key] == nil {
		d.mechanicTotals[key] = map[string]int64{}
	}
	d.mechanicTotals[key][id] += n
}

func wantedMechanic(boss, key string) bool {
	for _, a := range encounterAwards[boss] {
		for _, k := range a.keys {
			if k == key {
				return true
			}
		}
	}
	return false
}

type bombDrop struct {
	at        time.Time
	detonated bool
}

func (s *Store) trackDetailedMechanics(seg *segment, f []string) {
	scope := mechanicScope(seg)
	if scope == "" || len(f) < 9 {
		return
	}
	d := seg.damage
	if d.bombCarriers == nil {
		d.bombCarriers = map[string]time.Time{}
		d.bombDrops = map[string]bombDrop{}
		d.dropBlocked = map[string]time.Time{}
		d.mutilated = map[string]time.Time{}
	}
	if f[0] == "UNIT_DIED" && player(f[5]) {
		d.dropBlocked[f[5]] = s.last
	}
	if len(f) < 12 {
		s.finishBombDrops(seg)
		return
	}
	if f[0] == "SPELL_DISPEL" && len(f) > 12 && friendlyGroup(f[3]) && player(f[5]) && friendlyGroup(f[7]) {
		if seg.id == "3420" && f[12] == "1305963" {
			d.dropBlocked[f[5]] = s.last
		}
		key := "dispel/" + f[12]
		if wantedMechanic(scope, key) {
			d.addMechanic(key, f[1], 1)
		}
	}
	if f[0] == "SPELL_INTERRUPT" && len(f) > 12 && friendlyGroup(f[3]) && !friendlyGroup(f[7]) {
		key := "interrupt/" + f[12]
		if wantedMechanic(scope, key) {
			d.addMechanic(key, f[1], 1)
		}
	}
	if player(f[5]) && friendlyGroup(f[7]) {
		switch f[0] {
		case "SPELL_AURA_APPLIED":
			key := "aura/" + f[9]
			if wantedMechanic(scope, key) || seg.id == "3421" && f[9] == "1310096" {
				d.players[f[5]] = true
				s.party[f[5]] = f[6]
				// A refresh or periodic tick is not a second mechanic failure.
				if d.lastSoak[key+"/"+f[5]].IsZero() {
					d.addMechanic(key, f[5], 1)
					d.lastSoak[key+"/"+f[5]] = s.last
				}
			}
			if seg.id == "3420" && f[9] == "1305963" {
				if old, ok := d.bombCarriers[f[5]]; !ok || s.last.Sub(old) > 12*time.Second {
					d.bombCarriers[f[5]] = s.last
				}
				d.players[f[5]] = true
				s.party[f[5]] = f[6]
			}
			if seg.id == "3420" && f[9] == "1277051" {
				if d.mutilated[f[5]].IsZero() {
					d.mutilated[f[5]] = s.last
				}
			}
		case "SPELL_AURA_REMOVED":
			delete(d.lastSoak, "aura/"+f[9]+"/"+f[5])
			if seg.id == "3420" && f[9] == "1277051" {
				delete(d.mutilated, f[5])
			}
			if seg.id == "3420" && f[9] == "1305963" {
				start, ok := d.bombCarriers[f[5]]
				delete(d.bombCarriers, f[5])
				age := s.last.Sub(start)
				if ok && age >= 9*time.Second && age <= 12*time.Second {
					d.bombDrops[f[5]] = bombDrop{s.last, !d.bombDetonation.IsZero() && mechanicTimeGap(s.last.Sub(d.bombDetonation)) <= 100*time.Millisecond}
				}
			}
		}
		if f[0] == "SPELL_DAMAGE" && !friendlyGroup(f[3]) && wantedMechanic(scope, "hit/"+f[9]) {
			if base, support, ok := damagePayload(f); ok && !support {
				if _, reason := recordedDamageAmount(f, base); reason == "" {
					d.addMechanic("hit/"+f[9], f[5], 1)
				}
			}
		}
		if seg.id == "3420" && f[9] == "1306120" && f[0] == "SPELL_DAMAGE" && !friendlyGroup(f[3]) {
			if _, reason := recordedDamageAmount(f, 12); reason == "" {
				d.bombDetonation = s.last
				for id, drop := range d.bombDrops {
					if mechanicTimeGap(s.last.Sub(drop.at)) <= 100*time.Millisecond {
						drop.detonated = true
						d.bombDrops[id] = drop
					}
				}
			}
		}
	}
	s.finishBombDrops(seg)
}

func mechanicTimeGap(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func (s *Store) finishBombDrops(seg *segment) {
	d := seg.damage
	for id, drop := range d.bombDrops {
		if s.last.Sub(drop.at) < 250*time.Millisecond {
			continue
		}
		dead := d.dropBlocked[id]
		if drop.detonated && (dead.IsZero() || mechanicTimeGap(dead.Sub(drop.at)) > 250*time.Millisecond) {
			d.addMechanic("bomb/drop", id, 1)
		}
		delete(d.bombDrops, id)
	}
}

func detailedBossAwards(m DamageMeter) (good, banter []string, soaks, avoidable, dispels, interrupts bool) {
	c := m.Utility
	if c == nil || !c.Known || c.AwardVersion < awardStatsVersion {
		return
	}
	goodLimit, badLimit := bossAwardLimit, bossAwardLimit
	if hasDungeonAwards(m) {
		goodLimit -= 3
		badLimit -= 2
	}
	for _, a := range encounterAwards[awardScope(m)] {
		required := a.minimumVersion
		if a.extended && required < 1 {
			required = 1
		}
		if c.EncounterAwardVersion < required {
			continue
		}
		values := map[string]int64{}
		unknown := false
		for _, key := range a.keys {
			unknown = unknown || c.MechanicUnknown[key]
			for id, n := range c.MechanicTotals[key] {
				values[id] += n
			}
		}
		if unknown {
			continue
		}
		name, n := awardNames(m.Rows, func(r DamageRow) int64 { return values[r.GUID] })
		if n <= 0 || n < a.minimum {
			continue
		}
		amount := fmt.Sprint(n)
		if a.damage {
			amount = compact(n)
		}
		unit := a.unit
		if n == 1 {
			unit = strings.NewReplacer(" dispels", " dispel", " soaks", " soak", " stuns", " stun", " catches", " catch", " drops", " drop", " failures", " failure", " interrupts", " interrupt", " blasts", " blast", "assignments", "assignment", "applications", "application", "pickups", "pickup", "trips", "trip", "bounces", "bounce", "breaks", "break", "puzzles", "puzzle", "kills", "kill").Replace(unit)
		}
		line := fmt.Sprintf("%s: %s - %s %s. %s", a.label, name, amount, unit, a.joke)
		if a.banter {
			if len(banter) < badLimit {
				banter = append(banter, line)
				for _, key := range a.keys {
					avoidable = avoidable || strings.HasPrefix(key, "avoid/")
				}
			}
		} else if len(good) < goodLimit {
			good = append(good, line)
			for _, key := range a.keys {
				soaks = soaks || strings.HasPrefix(key, "soak/")
				dispels = dispels || strings.HasPrefix(key, "dispel/")
				interrupts = interrupts || strings.HasPrefix(key, "interrupt/")
			}
		}
	}
	return
}
