package priest

import (
	"fmt"
	"time"

	"github.com/wowsims/sod/sim/core"
	"github.com/wowsims/sod/sim/core/proto"
)

const MindFlayRanks = 6
const MindFlayTicks = 3

var MindFlaySpellId = [MindFlayRanks + 1]int32{0, 15407, 17311, 17312, 17313, 17314, 18807}
var MindFlayTickSpellId = [MindFlayRanks + 1]int32{0, 16568, 7378, 17316, 17317, 17318, 18808}
var MindFlayBaseDamage = [MindFlayRanks + 1]float64{0, 75, 126, 186, 261, 330, 426}
var MindFlayManaCost = [MindFlayRanks + 1]float64{0, 45, 70, 100, 135, 165, 205}
var MindFlayLevel = [MindFlayRanks + 1]int{0, 20, 28, 36, 44, 52, 60}

func (priest *Priest) registerMindFlay() {
	if !priest.Talents.MindFlay {
		return
	}

	priest.MindFlay = make([][]*core.Spell, MindFlayRanks+1)

	for rank := 1; rank < MindFlayRanks; rank++ {
		priest.MindFlay[rank] = make([]*core.Spell, MindFlayTicks+1)

		var tick int32
		for tick = 0; tick < MindFlayTicks; tick++ {
			config := priest.newMindFlaySpellConfig(rank, tick)

			if config.RequiredLevel <= int(priest.Level) {
				priest.MindFlay[rank][tick] = priest.RegisterSpell(config)
			}
		}
	}
}

func (priest *Priest) newMindFlaySpellConfig(rank int, tickIdx int32) core.SpellConfig {
	ticks := tickIdx
	flags := core.SpellFlagChanneled
	if tickIdx == 0 {
		ticks = 3
		flags |= core.SpellFlagAPL
	}

	spellId := MindFlaySpellId[rank]
	baseDamage := MindFlayBaseDamage[rank] / float64(ticks)
	manaCost := MindFlayManaCost[rank]
	level := MindFlayLevel[rank]

	spellCoeff := 0.15 // classic penalty for mf having a slow effect

	tickLength := time.Second

	hasDespairRune := priest.HasRune(proto.PriestRune_RuneBracersDespair)

	return core.SpellConfig{
		ActionID:    core.ActionID{SpellID: spellId}.WithTag(tickIdx),
		SpellSchool: core.SpellSchoolShadow,
		DefenseType: core.DefenseTypeMagic,
		ProcMask:    core.ProcMaskSpellDamage,
		Flags:       flags | core.SpellFlagBinary,

		RequiredLevel: level,
		Rank:          rank,

		ManaCost: core.ManaCostOptions{
			FlatCost: manaCost,
		},

		Cast: core.CastConfig{
			DefaultCast: core.Cast{
				GCD: core.GCDDefault,
			},
		},

		BonusCritRating: priest.forceOfWillCritRating(),
		BonusHitRating:  priest.shadowHitModifier(),

		CritDamageBonus: core.TernaryFloat64(hasDespairRune, 1, 0),

		DamageMultiplier: 1,
		ThreatMultiplier: priest.shadowThreatModifier(),

		Dot: core.DotConfig{
			Aura: core.Aura{
				Label: fmt.Sprintf("MindFlay-%d-%d", rank, tickIdx),
			},
			NumberOfTicks:       ticks,
			TickLength:          tickLength,
			AffectedByCastSpeed: false,
			BonusCoefficient:    spellCoeff,
			OnSnapshot: func(sim *core.Simulation, target *core.Unit, dot *core.Dot, isRollover bool) {
				dot.Spell.DamageMultiplier *= priest.MindFlayModifier
				dot.Snapshot(target, baseDamage, isRollover)
				dot.Spell.DamageMultiplier /= priest.MindFlayModifier
			},
			OnTick: func(sim *core.Simulation, target *core.Unit, dot *core.Dot) {
				if hasDespairRune {
					dot.CalcAndDealPeriodicSnapshotDamage(sim, target, dot.OutcomeTickSnapshotCritCounted)
				} else {
					dot.CalcAndDealPeriodicSnapshotDamage(sim, target, dot.OutcomeTickCounted)
				}
			},
		},

		ApplyEffects: func(sim *core.Simulation, target *core.Unit, spell *core.Spell) {
			result := spell.CalcOutcome(sim, target, spell.OutcomeMagicHit)
			if result.Landed() {
				spell.SpellMetrics[target.UnitIndex].Hits--
				priest.AddShadowWeavingStack(sim, target)
				spell.Dot(target).Apply(sim)
			}
			spell.DealOutcome(sim, result)
		},

		ExpectedTickDamage: func(sim *core.Simulation, target *core.Unit, spell *core.Spell, _ bool) *core.SpellResult {
			baseDamage := baseDamage / MindFlayTicks
			spell.DamageMultiplier *= priest.MindFlayModifier
			result := spell.CalcPeriodicDamage(sim, target, baseDamage, spell.OutcomeExpectedMagicAlwaysHit)
			spell.DamageMultiplier /= priest.MindFlayModifier
			return result
		},
	}
}
