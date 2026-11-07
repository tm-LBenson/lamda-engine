package healsim

import (
	"fmt"
	"github.com/tm-LBenson/lamda-engine/internal/wowspec"
)

func ValidateSpellbook(spec int, stats Stats) error {
	if spec == 0 || spec == 1468 {
		return validatePreservation(stats)
	}
	useful := false
	for _, sp := range spellsFor(spec) {
		if sp.passive {
			continue
		}
		known, ok := stats.KnownSpells[sp.id]
		if !ok {
			return fmt.Errorf("update your character outside combat to capture the complete healer spellbook")
		}
		if known && (sp.direct > 0 || sp.hot > 0 || sp.damage > 0) {
			useful = true
		}
	}
	if !useful {
		return fmt.Errorf("none of this build's active healing spells are modeled yet")
	}
	return nil
}

func Supported(id int) bool  { s, ok := wowspec.Lookup(id); return ok && s.Healer }
func SpecName(id int) string { s, _ := wowspec.Lookup(id); return s.Label() }
func resourceName(id int) string {
	if id == 65 {
		return "Holy Power"
	}
	if id == 1468 {
		return "Essence"
	}
	return ""
}

// Original transcription of the pinned 12.1.0.69933 DBC numeric facts.
// Coefficients include the listed spec aura and spell-specific modifiers.
// Optional spells must also be present in the captured spellbook. Passive rows
// identify secondary healing without granting the player an extra cast.
var coreKits = map[int][]spell{
	105: { // Restoration aura 137012: +8%; Wild Growth periodic +44%.
		{id: 8936, name: "Regrowth", direct: 5.36 * 1.08, hot: .224968 * 1.08, period: 2, duration: 6, cast: 1.5, cost: .0252, targets: 1, charges: 1},
		{id: 774, name: "Rejuvenation", hot: .924 * 1.08, period: 3, duration: 12, cost: .0189, targets: 1, charges: 1, initial: true},
		{id: 33763, name: "Lifebloom", hot: .2684 * 1.08, bloom: 2.18868 * 1.08, period: 1, duration: 15, cost: .0128, targets: 1, charges: 1},
		{id: 18562, name: "Swiftmend", direct: 10.37 * 1.08, cd: 15, cost: .014, targets: 1, charges: 1},
		{id: 48438, name: "Wild Growth", hot: .481 * 1.08 * 1.44, period: 1, duration: 7, cast: 1.5, cd: 10, cost: .0437, targets: 5, charges: 1},
		{id: 145205, name: "Efflorescence", hot: .5856 * 1.08, period: 2, duration: 30, cost: .034, targets: 3, charges: 1, ground: true},
		{id: 740, name: "Tranquility", hot: 10.1992 * 1.08, period: 1, duration: 6, cast: 6, cost: .0368, cd: 180, targets: 40, charges: 1, channel: true, ground: true, initial: true},
	},
	256: { // Discipline aura 137032 +10% healing; Flash Heal +120%, shield coefficient +149%.
		{id: 2061, name: "Flash Heal", direct: 6.82 * 1.1 * 2.2, cast: 1.5, cost: .036, targets: 1, charges: 1},
		{id: 17, name: "Power Word: Shield", shield: 4.638 * 2.49, duration: 15, cd: 7.5, cost: .026, targets: 1, charges: 1},
		{id: 194509, name: "Power Word: Radiance", direct: 5.75 * 1.1, cast: 2, cd: 18, cost: .054, targets: 5, charges: 1},
		{id: 585, name: "Smite", damage: .987 * 1.25 * 1.4, cast: 1.5, cost: .004, targets: 1, charges: 1},
		{id: 47540, name: "Penance (damage)", damage: .932 * 1.25, period: 1, duration: 2, cast: 2, cd: 9, cost: .0128, targets: 1, charges: 1, channel: true, initial: true, hastedCD: true},
		{id: 81751, name: "Atonement", passive: true, charges: 1},
	},
	257: { // Holy aura 137031 +49%; Flash Heal additionally +48%.
		{id: 2061, name: "Flash Heal", direct: 6.82 * 1.49 * 1.48, cast: 1.5, cost: .0261, targets: 1, charges: 1},
		{id: 596, name: "Prayer of Healing", direct: 5.78742 * 1.49, cast: 2.5, cost: .04347, targets: 5, charges: 1},
		{id: 2050, name: "Holy Word: Serenity", direct: 21.4493 * 1.49, cd: 60, cost: .02375, targets: 1, charges: 1},
		{id: 34861, name: "Holy Word: Sanctify", direct: 6.01321 * 1.49, cd: 60, cost: .03325, targets: 5, charges: 1},
		{id: 33076, name: "Prayer of Mending", direct: .732 * 1.49, cd: 12, cost: .0175, targets: 1, charges: 1},
		{id: 64843, name: "Divine Hymn", hot: 5.6088 * 1.49, period: 1.25, duration: 5, cast: 5, cd: 180, cost: .044, targets: 40, charges: 1, channel: true, ground: true, initial: true},
		{id: 77489, name: "Echo of Light", passive: true, charges: 1},
	},
	264: { // Restoration aura 137039 +15%; Chain Heal +240%.
		{id: 77472, name: "Healing Wave", direct: 15.01 * 1.15, cast: 2, cost: .0238, targets: 1, charges: 1},
		{id: 8004, name: "Healing Surge", direct: 11 * 1.15, cast: 1.5, cost: .044, targets: 1, charges: 1},
		{id: 61295, name: "Riptide", direct: 6 * 1.15, hot: .797 * 1.15, period: 3, duration: 18, cd: 6, cost: .0152, targets: 1, charges: 1},
		{id: 1064, name: "Chain Heal", direct: 2.31 * 1.15 * 3.4, chain: .7, cast: 2, cost: .0428, targets: 4, charges: 1},
		{id: 73920, name: "Healing Rain", hot: 1.08 * 1.15, period: 2, duration: 18, cast: 2, cd: 12, cost: .0432, targets: 6, charges: 1, ground: true},
		{id: 5394, name: "Healing Stream Totem", hot: .6768 * 1.15, period: 2, duration: 15, cd: 30, cost: .018, targets: 1, charges: 1, ground: true},
		{id: 108280, name: "Healing Tide Totem", hot: 5.145 * 1.15, period: 2, duration: 10, cd: 180, cost: .0112, targets: 40, charges: 1, ground: true},
	},
	270: { // Mistweaver aura 137024 -5%; direct Vivify +121%.
		{id: 116670, name: "Vivify", direct: 2.58 * .95 * 2.21, cast: 1.5, cost: .03, targets: 1, charges: 1},
		{id: 115151, name: "Renewing Mist", hot: .19665 * .95, period: 2, duration: 20, cd: 9, cost: .018, targets: 1, charges: 2},
		{id: 124682, name: "Enveloping Mist", hot: 1.248 * .95, period: 1, duration: 6, cast: 2, cost: .036, targets: 1, charges: 1},
		{id: 115310, name: "Revival", direct: 59.8828 * .95, cd: 180, cost: .04374, targets: 40, charges: 1},
		{id: 191894, name: "Gust of Mists", passive: true, charges: 1},
		{id: 425804, name: "Invigorating Mists", direct: 3.2318 * .95, passive: true, charges: 1},
	},
	65: { // Holy aura 137029 +19%; Flash of Light +50%; Word of Glory +472%.
		{id: 19750, name: "Flash of Light", direct: 3.156 * 1.19 * 1.5, cast: 1.5, cost: .006, targets: 1, charges: 1},
		{id: 82326, name: "Holy Light", direct: 32.5 * 1.19, cast: 2, cost: .07, targets: 1, charges: 1},
		{id: 20473, name: "Holy Shock", direct: 11.17 * 1.19, cd: 6, cost: .02, targets: 1, charges: 1, hastedCD: true},
		{id: 85673, name: "Word of Glory", direct: 4.158 * 1.19 * 5.72, cost: .006, targets: 1, charges: 1},
		{id: 85222, name: "Light of Dawn", direct: 5.06736 * 1.19, cost: .006, targets: 5, charges: 1},
		{id: 31884, name: "Avenging Wrath", duration: 20, cd: 120, targets: 1, charges: 1},
		{id: 53563, name: "Beacon of Light", hot: .33 * 1.19, period: 3, duration: 600, cost: .005, targets: 1, charges: 1},
		{id: 53652, name: "Beacon transfer", passive: true, charges: 1},
	},
}

func spellsFor(spec int) []spell {
	if spec == 0 || spec == 1468 {
		return spells
	}
	return coreKits[spec]
}
func modeledFor(spec int) []int {
	out := []int{}
	for _, sp := range spellsFor(spec) {
		if sp.id > 0 {
			out = append(out, sp.id)
		}
	}
	return out
}
func coreWarnings(spec int) []string {
	common := []string{
		"Experimental " + SpecName(spec) + " core-spell model. Hero/apex talents, most optional talents, set bonuses, gear procs, external buffs and movement are not modeled. Do not rank builds, gear or healer specializations with these results.",
		"Captured spellbook gates active abilities; the model never substitutes another healer's spells. Missing or replaced core spells reduce coverage. Stats stay fixed; resources start full (Holy Power starts at zero), cooldowns ready. Casts and ticks resolve at 50 ms. Captured max mana is treated as base mana.",
		"One healer with all allies in range. Synthetic raid damage is an assigned workload. No co-healer healing, self-healing, defensives, resurrection, target movement or enemy mitigation. Raid and dungeon spell policies are simplified, guide-informed priorities, not complete rotations.",
		"Absorbs count only when consumed and are reported separately from effective healing. Low incoming damage caps useful HPS. Sampling uncertainty does not measure model accuracy.",
	}
	extra := map[int]string{
		105: "Druid: Regrowth, Rejuvenation, one Lifebloom with bloom/late refresh, Swiftmend consuming an eligible HoT, Wild Growth, Efflorescence and channeled Tranquility. Harmony includes only the first HoT bonus; additional diminishing stacks are unmodeled. Wild Growth uses average tick strength; its decay, Swiftmend's HoT-size bonus, Lifebloom stacks, Flourish, guardians, Convoke and Tree of Life are omitted.",
		256: "Discipline: shields, Flash Heal/Radiance Atonement application, Smite and channeled damage Penance. Atonement uses one damage crit roll without double-dipping healing crit/versatility, +40% outside raids, and an assumed square-root reduction above five targets. Enemy damage is unmitigated. DoTs, Mind Blast, pets, Evangelism, Rapture, Voidweaver/Oracle mechanics and damage buffs are omitted.",
		257: "Holy Priest: Flash Heal/Prayer reduce Holy Word cooldowns, Prayer of Mending triggers on surviving damage and jumps, Echo of Light rolls remaining healing forward, and Hymn channels with its healing-taken buff. Extra charges and talent cooldown modifiers, Benediction, Apotheosis, Halo and hero trees are omitted.",
		264: "Shaman: Riptide, Wave/Surge, diminishing Chain Heal jumps, dynamic Deep Healing and smart ground/totem pulses. Tidal Waves, Earth Shield, Resurgence, Spirit Link, Ascendance and hero totems/ancestors are omitted. Rain assumes all allies remain inside it.",
		270: "Mistweaver: caster core only: Renewing Mist, Enveloping Mist, Vivify/Invigorating Mists, Gust of Mists and Revival. Renewing Mist does not jump; Enveloping Mist's group direct-heal effect is omitted pending trigger calibration. Melee healing, tea, celestials, Sheilun's Gift, hero and apex mechanics are not implemented. This is not a complete fistweaving model.",
		65:  "Paladin: Holy Power generation/spending, Shock, Flash/Holy Light, Word of Glory, Dawn, Wings and one Beacon with 10% effective-heal transfer. All targets are assumed within full mastery range. Infusion, Eternal Flame, Virtue/Faith, Toll, Judgment healing, hero and apex effects are omitted. Replaced Word of Glory is never silently treated as Eternal Flame.",
	}
	return append(common, extra[spec])
}
