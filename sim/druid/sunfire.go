package druid

import (
	"time"

	"github.com/wowsims/sod/sim/core"
	"github.com/wowsims/sod/sim/core/proto"
)

const SunfireTicks = int32(4)

func (druid *Druid) registerSunfireSpell() {
	if !druid.HasRune(proto.DruidRune_RuneHandsSunfire) {
		return
	}

	moonfuryMultiplier := druid.MoonfuryDamageMultiplier()
	impMoonfireMultiplier := druid.ImprovedMoonfireDamageMultiplier()

	baseDamageLow := druid.baseRuneAbilityDamage() * 1.3 * moonfuryMultiplier * impMoonfireMultiplier
	baseDamageHigh := druid.baseRuneAbilityDamage() * 1.52 * moonfuryMultiplier * impMoonfireMultiplier
	baseDotDamage := druid.baseRuneAbilityDamage() * 0.65 * moonfuryMultiplier * impMoonfireMultiplier

	druid.registerSunfireHumanoidSpell(baseDamageLow, baseDamageHigh, baseDotDamage)
	druid.registerSunfireCatSpell(baseDamageLow, baseDamageHigh, baseDotDamage)
}

func (druid *Druid) registerSunfireHumanoidSpell(baseDamageLow float64, baseDamageHigh float64, baseDotDamage float64) {
	actionID := core.ActionID{SpellID: int32(proto.DruidRune_RuneHandsSunfire)}
	spellCoeff := .15
	dotCoeff := .13

	druid.SunfireDotMultiplier = 1

	config := druid.getSunfireBaseSpellConfig(
		actionID,
		core.SpellFlagResetAttackSwing,
		func(sim *core.Simulation, _ *core.Spell) float64 {
			return sim.Roll(baseDamageLow, baseDamageHigh)
		},
		func(_ *core.Spell) float64 {
			return baseDotDamage
		},
		func(_ *core.Simulation, _ *core.Spell) {},
	)

	config.ManaCost = core.ManaCostOptions{
		BaseCost: 0.21,
	}
	config.BonusCoefficient = spellCoeff
	config.Dot.BonusCoefficient = dotCoeff

	druid.Sunfire = druid.RegisterSpell(Humanoid|Moonkin, *config)
}

// TODO: Bear form sunfire
// func (druid *Druid) registerSunfireBearSpell(baseDamageLow float64, baseDamageHigh float64, baseDotDamage float64) {}

func (druid *Druid) registerSunfireCatSpell(baseDamageLow float64, baseDamageHigh float64, baseDotDamage float64) {
	actionID := core.ActionID{SpellID: 414689}

	spellAPCoeff := .12
	dotAPCoeff := .104

	config := druid.getSunfireBaseSpellConfig(
		actionID,
		core.SpellFlagNone,
		func(sim *core.Simulation, spell *core.Spell) float64 {
			// Sunfire (Cat) uses a different scaling formula based on the Druid's AP
			return sim.Roll(baseDamageLow, baseDamageHigh) + spellAPCoeff*spell.MeleeAttackPower()
		},
		func(spell *core.Spell) float64 {
			// Sunfire (Cat) uses a different scaling formula based on the Druid's AP
			return baseDotDamage + dotAPCoeff*spell.MeleeAttackPower()
		},
		func(sim *core.Simulation, spell *core.Spell) {
			druid.AddComboPoints(sim, 1, spell.ComboPointMetrics())
		},
	)

	config.EnergyCost = core.EnergyCostOptions{
		Cost: 40,
	}

	druid.SunfireCat = druid.RegisterSpell(Cat, *config)
}

func (druid *Druid) getSunfireBaseSpellConfig(
	actionID core.ActionID,
	additionalFlags core.SpellFlag,
	getBaseDamage func(sim *core.Simulation, spell *core.Spell) float64,
	getBaseDotDamage func(spell *core.Spell) float64,
	// Callback for additional logic after a cast lands like adding a combo point for the Feral spell
	onResultLanded func(sim *core.Simulation, spell *core.Spell),
) *core.SpellConfig {
	return &core.SpellConfig{
		ActionID:    actionID,
		SpellSchool: core.SpellSchoolNature,
		DefenseType: core.DefenseTypeMagic,
		ProcMask:    core.ProcMaskSpellDamage,
		Flags:       core.SpellFlagAPL | SpellFlagOmen | additionalFlags,

		Cast: core.CastConfig{
			DefaultCast: core.Cast{
				GCD: core.GCDDefault,
			},
		},

		Dot: core.DotConfig{
			Aura: core.Aura{
				Label:    "Sunfire",
				ActionID: actionID,
			},
			NumberOfTicks: SunfireTicks,
			TickLength:    time.Second * 3,
			OnSnapshot: func(sim *core.Simulation, target *core.Unit, dot *core.Dot, isRollover bool) {
				baseDamage := getBaseDotDamage(dot.Spell)
				dot.Snapshot(target, baseDamage, isRollover)
				dot.SnapshotAttackerMultiplier *= druid.SunfireDotMultiplier
			},
			OnTick: func(sim *core.Simulation, target *core.Unit, dot *core.Dot) {
				dot.CalcAndDealPeriodicSnapshotDamage(sim, target, dot.OutcomeTick)
			},
		},

		BonusCritRating: druid.ImprovedMoonfireCritBonus() * core.SpellCritRatingPerCritChance,

		CritDamageBonus: druid.vengeance(),

		DamageMultiplier: 1,
		ThreatMultiplier: 1,

		ApplyEffects: func(sim *core.Simulation, target *core.Unit, spell *core.Spell) {
			baseDamage := getBaseDamage(sim, spell)
			result := spell.CalcAndDealDamage(sim, target, baseDamage, spell.OutcomeMagicHitAndCrit)

			if result.Landed() {
				dot := spell.Dot(target)
				dot.NumberOfTicks = SunfireTicks
				dot.RecomputeAuraDuration()
				dot.Apply(sim)
				onResultLanded(sim, spell)
			}
		},

		ExpectedTickDamage: func(sim *core.Simulation, target *core.Unit, spell *core.Spell, _ bool) *core.SpellResult {
			dot := spell.Dot(target)
			return dot.CalcSnapshotDamage(sim, target, dot.Spell.OutcomeExpectedMagicAlwaysHit)
		},
	}
}
