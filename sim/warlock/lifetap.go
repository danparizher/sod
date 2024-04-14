package warlock

import (
	"github.com/wowsims/sod/sim/core"
)

func (warlock *Warlock) getLifeTapBaseConfig(rank int) core.SpellConfig {
	spellId := [7]int32{0, 1454, 1455, 1456, 11687, 11688, 11689}[rank]
	baseDamage := [7]float64{0, 30, 75, 140, 220, 310, 424}[rank]
	level := [7]int{0, 6, 16, 26, 36, 46, 56}[rank]

	actionID := core.ActionID{SpellID: spellId}
	var manaPetMetrics *core.ResourceMetrics
	if warlock.Pet != nil {
		manaPetMetrics = warlock.Pet.NewManaMetrics(actionID)
	}
	manaMetrics := warlock.NewManaMetrics(actionID)
	impLifetap := 1.0 + 0.1*float64(warlock.Talents.ImprovedLifeTap)
	spellCoef := 0.68

	return core.SpellConfig{
		ActionID:      actionID,
		SpellSchool:   core.SpellSchoolShadow,
		ProcMask:      core.ProcMaskSpellDamage,
		Flags:         core.SpellFlagAPL | core.SpellFlagResetAttackSwing,
		RequiredLevel: level,
		Rank:          rank,

		Cast: core.CastConfig{
			DefaultCast: core.Cast{
				GCD: core.GCDDefault,
			},
		},

		ThreatMultiplier: 1,

		ApplyEffects: func(sim *core.Simulation, target *core.Unit, spell *core.Spell) {
			restore := (baseDamage + spellCoef*spell.BonusDamage()) * impLifetap
			// TODO: Deal damage to warlock for tank sims

			if warlock.MetamorphosisAura != nil && warlock.MetamorphosisAura.IsActive() {
				restore *= 2
			}
			warlock.AddMana(sim, restore, manaMetrics)
			if warlock.Pet != nil && warlock.Pet.IsActive() {
				warlock.Pet.AddMana(sim, restore, manaPetMetrics)
			}
		},
	}
}

func (warlock *Warlock) registerLifeTapSpell() {
	maxRank := 6

	for i := 1; i <= maxRank; i++ {
		config := warlock.getLifeTapBaseConfig(i)

		if config.RequiredLevel <= int(warlock.Level) {
			warlock.LifeTap = warlock.GetOrRegisterSpell(config)
		}
	}
}
