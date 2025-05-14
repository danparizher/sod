package paladin

import (
	"time"

	"github.com/wowsims/sod/sim/core"
	"github.com/wowsims/sod/sim/core/proto"
	"github.com/wowsims/sod/sim/core/stats"
)

func (paladin *Paladin) registerOnHolyPowerSpent(onHolyPowerSpent OnHolyPowerSpent) {
	paladin.onHolyPowerSpent = append(paladin.onHolyPowerSpent, onHolyPowerSpent)
}

func (paladin *Paladin) registerHolyPowerAura() {
	if paladin.holyPowerAura != nil {
		return
	}

	paladin.holyPowerICD = &core.Cooldown{
		Timer:    paladin.NewTimer(),
		Duration: time.Millisecond * 100,
	}
	paladin.holyPowerAura = paladin.GetOrRegisterAura(core.Aura{
		ActionID:  core.ActionID{SpellID: 1226461},
		Label:     "Holy Power",
		MaxStacks: 3,
		Duration:  time.Second * 15,
		OnStacksChange: func(aura *core.Aura, sim *core.Simulation, oldStacks int32, newStacks int32) {
			aura.Unit.PseudoStats.SchoolDamageDealtMultiplier[stats.SchoolIndexHoly] *= ((1.0 + (0.10 * float64(newStacks))) / (1.0 + (0.10 * float64(oldStacks))))
		},
	})

	paladin.RegisterItemSwapCallback([]proto.ItemSlot{proto.ItemSlot_ItemSlotMainHand}, func(sim *core.Simulation, _ proto.ItemSlot, _ bool) {
		paladin.holyPowerAura.Deactivate(sim)
	})
}

var ItemSetInquisitionWarplate = core.NewItemSet(core.ItemSet{
	ID:   1940,
	Name: "Inquisition Warplate",
	Bonuses: map[int32]core.ApplyEffect{
		// While you have a two-handed weapon equipped, Crusader Strike and Exorcism grant you Holy Power, increasing all Holy damage you deal by 10%, stacking up to 3 times.
		2: func(agent core.Agent) {
			paladin := agent.(PaladinAgent).GetPaladin()
			paladin.applyScarletEnclaveRetribution2PBonus()
		},
		// Divine Storm, Holy Shock, and Holy Wrath consume all your Holy Power, dealing 100% increased damage per Holy Power you have accumulated.
		4: func(agent core.Agent) {
			paladin := agent.(PaladinAgent).GetPaladin()
			paladin.applyScarletEnclaveDps4PBonus()
		},
		// Consuming Holy Power increases your Attack Power by 15% per Holy Power consumed for 10 sec.
		6: func(agent core.Agent) {
			paladin := agent.(PaladinAgent).GetPaladin()
			paladin.applyScarletEnclaveRetribution6PBonus()
		},
	},
})

// While you have a two-handed weapon equipped, Crusader Strike and Exorcism grant you Holy Power, increasing all Holy damage you deal by 10%, stacking up to 3 times.
func (paladin *Paladin) applyScarletEnclaveRetribution2PBonus() {
	label := "S03 - Item - Scarlet Enclave - Paladin - Retribution 2P Bonus"
	if paladin.HasAura(label) {
		return
	}

	paladin.registerHolyPowerAura()

	core.MakePermanent(paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: PaladinTSERet2P},
		Label:    label,
		OnSpellHitDealt: func(aura *core.Aura, sim *core.Simulation, spell *core.Spell, result *core.SpellResult) {
			if paladin.MainHand().HandType != proto.HandType_HandTypeTwoHand {
				return
			}

			if !paladin.holyPowerICD.IsReady(sim) {
				return
			}

			if spell.Matches(ClassSpellMask_PaladinExorcism|ClassSpellMask_PaladinCrusaderStrike) && result.Landed() {
				paladin.holyPowerICD.Use(sim)
				paladin.holyPowerAura.Activate(sim)
				paladin.holyPowerAura.AddStack(sim)
			}
		},
	}))
}

// Divine Storm, Holy Shock, and Holy Wrath consume all your Holy Power, dealing 100% increased damage per Holy Power you have accumulated.
func (paladin *Paladin) applyScarletEnclaveDps4PBonus() {
	label := "S03 - Item - Scarlet Enclave - Paladin - Retribution 4P Bonus"
	if paladin.HasAura(label) {
		return
	}

	paladin.registerHolyPowerAura()

	additiveMod := paladin.AddDynamicMod(core.SpellModConfig{
		Kind:      core.SpellMod_DamageDone_Flat,
		ClassMask: ClassSpellMask_PaladinDivineStorm | ClassSpellMask_PaladinHolyShock,
	})

	multiplicativeMod := paladin.AddDynamicMod(core.SpellModConfig{
		Kind:       core.SpellMod_DamageDone_Pct,
		ClassMask:  ClassSpellMask_PaladinHolyWrath,
		FloatValue: 1.0,
	})

	paladin.holyPowerAura.ApplyOnGain(func(aura *core.Aura, sim *core.Simulation) {
		additiveMod.Activate()
		multiplicativeMod.Activate()
	}).ApplyOnExpire(func(aura *core.Aura, sim *core.Simulation) {
		additiveMod.Deactivate()
		multiplicativeMod.Deactivate()
	}).ApplyOnStacksChange(func(aura *core.Aura, sim *core.Simulation, oldStacks, newStacks int32) {
		additiveMod.UpdateIntValue(100 * int64(newStacks))
		multiplicativeMod.UpdateFloatValue(1.0 + (1.0 * float64(newStacks)))
	})

	core.MakePermanent(paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: PaladinTSERet4P},
		Label:    label,
		OnSpellHitDealt: func(aura *core.Aura, sim *core.Simulation, spell *core.Spell, result *core.SpellResult) {
			if !paladin.holyPowerAura.IsActive() {
				return
			}

			if !spell.Matches(ClassSpellMask_PaladinDivineStorm|ClassSpellMask_PaladinHolyShock|ClassSpellMask_PaladinHolyWrath) || !result.Landed() {
				return
			}

			holyPower := paladin.holyPowerAura.GetStacks()
			for _, onHolyPowerSpent := range paladin.onHolyPowerSpent {
				onHolyPowerSpent(sim, holyPower)
			}

			paladin.holyPowerAura.Deactivate(sim)
		},
	}))
}

// Consuming Holy Power increases your Attack Power by 15% per Holy Power consumed for 10 sec.
func (paladin *Paladin) applyScarletEnclaveRetribution6PBonus() {
	label := "S03 - Item - Scarlet Enclave - Paladin - Retribution 6P Bonus"
	if paladin.HasAura(label) {
		return
	}

	apDeps := []*stats.StatDependency{
		paladin.NewDynamicMultiplyStat(stats.AttackPower, 1.0),
		paladin.NewDynamicMultiplyStat(stats.AttackPower, 1.15),
		paladin.NewDynamicMultiplyStat(stats.AttackPower, 1.30),
		paladin.NewDynamicMultiplyStat(stats.AttackPower, 1.45),
	}

	templarAura := paladin.RegisterAura(core.Aura{
		ActionID:  core.ActionID{SpellID: 1226464},
		Label:     "Templar",
		Duration:  time.Second * 10,
		MaxStacks: 3,
		OnStacksChange: func(aura *core.Aura, sim *core.Simulation, oldStacks, newStacks int32) {
			paladin.DisableDynamicStatDep(sim, apDeps[oldStacks])
			paladin.EnableDynamicStatDep(sim, apDeps[newStacks])
		},
	})

	paladin.registerOnHolyPowerSpent(func(sim *core.Simulation, holyPower int32) {
		if holyPower > 0 {
			core.StartDelayedAction(sim, core.DelayedActionOptions{
				DoAt: sim.CurrentTime + core.DurationFromSeconds(sim.Roll(0.05, 0.1)),
				OnAction: func(sim *core.Simulation) {
					templarAura.Activate(sim)
					templarAura.SetStacks(sim, holyPower)
				},
			})
		}
	})

	core.MakePermanent(paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: PaladinTSERet6P},
		Label:    label,
	}))
}

var ItemSetInquisitionShockplate = core.NewItemSet(core.ItemSet{
	ID:   1963,
	Name: "Inquisition Shockplate",
	Bonuses: map[int32]core.ApplyEffect{
		2: func(agent core.Agent) {
			paladin := agent.(PaladinAgent).GetPaladin()
			paladin.applyScarletEnclaveShockadin2PBonus()
		},
		4: func(agent core.Agent) {
			paladin := agent.(PaladinAgent).GetPaladin()
			paladin.applyScarletEnclaveDps4PBonus()
		},
		6: func(agent core.Agent) {
			paladin := agent.(PaladinAgent).GetPaladin()
			paladin.applyScarletEnclaveShockadin6PBonus()
		},
	},
})

// While Shock and Awe is active, Crusader Strike and Exorcism grant you Holy Power, increasing all Holy damage you deal by 10%, stacking up to 3 times.
func (paladin *Paladin) applyScarletEnclaveShockadin2PBonus() {
	if !paladin.hasRune(proto.PaladinRune_RuneCloakShockAndAwe) {
		return
	}

	label := "S03 - Item - Scarlet Enclave - Paladin - Shockadin 2P Bonus"
	if paladin.HasAura(label) {
		return
	}

	paladin.registerHolyPowerAura()

	core.MakePermanent(paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: PaladinTSEShock2P},
		Label:    label,
		OnSpellHitDealt: func(aura *core.Aura, sim *core.Simulation, spell *core.Spell, result *core.SpellResult) {
			if !paladin.holyPowerICD.IsReady(sim) {
				return
			}

			if spell.Matches(ClassSpellMask_PaladinExorcism|ClassSpellMask_PaladinCrusaderStrike) && result.Landed() {
				paladin.holyPowerICD.Use(sim)
				paladin.holyPowerAura.Activate(sim)
				paladin.holyPowerAura.AddStack(sim)
			}
		},
	}))
}

// Consuming Holy Power increases your Spell Power by 15% per Holy Power consumed for 10 sec.
func (paladin *Paladin) applyScarletEnclaveShockadin6PBonus() {
	label := "S03 - Item - Scarlet Enclave - Paladin - Shockadin 6P Bonus"
	if paladin.HasAura(label) {
		return
	}

	sd2hDeps := &[]*stats.StatDependency{
		paladin.NewDynamicMultiplyStat(stats.SpellDamage, 1.0),
		paladin.NewDynamicMultiplyStat(stats.SpellDamage, 1.18),
		paladin.NewDynamicMultiplyStat(stats.SpellDamage, 1.36),
		paladin.NewDynamicMultiplyStat(stats.SpellDamage, 1.54),
	}

	sd1hDeps := &[]*stats.StatDependency{
		paladin.NewDynamicMultiplyStat(stats.SpellDamage, 1.0),
		paladin.NewDynamicMultiplyStat(stats.SpellDamage, 1.08),
		paladin.NewDynamicMultiplyStat(stats.SpellDamage, 1.16),
		paladin.NewDynamicMultiplyStat(stats.SpellDamage, 1.24),
	}

	var currentEnabledDeps *[]*stats.StatDependency
	templarAura := paladin.RegisterAura(core.Aura{
		ActionID:  core.ActionID{SpellID: 1240574},
		Label:     "Templar",
		Duration:  time.Second * 10,
		MaxStacks: 3,
		OnStacksChange: func(aura *core.Aura, sim *core.Simulation, oldStacks, newStacks int32) {
			paladin.DisableDynamicStatDep(sim, (*currentEnabledDeps)[oldStacks])
			paladin.EnableDynamicStatDep(sim, (*currentEnabledDeps)[newStacks])
		},
	})

	paladin.RegisterItemSwapCallback([]proto.ItemSlot{proto.ItemSlot_ItemSlotMainHand}, func(sim *core.Simulation, _ proto.ItemSlot, _ bool) {
		templarAura.Deactivate(sim)

		if paladin.MainHand().HandType == proto.HandType_HandTypeTwoHand {
			currentEnabledDeps = sd2hDeps
		} else {
			currentEnabledDeps = sd1hDeps
		}
	})

	if paladin.MainHand().HandType == proto.HandType_HandTypeTwoHand {
		currentEnabledDeps = sd2hDeps
	} else {
		currentEnabledDeps = sd1hDeps
	}

	paladin.registerOnHolyPowerSpent(func(sim *core.Simulation, holyPower int32) {
		if holyPower > 0 {
			core.StartDelayedAction(sim, core.DelayedActionOptions{
				DoAt: sim.CurrentTime + core.DurationFromSeconds(sim.Roll(0.05, 0.1)),
				OnAction: func(sim *core.Simulation) {
					templarAura.Activate(sim)
					templarAura.SetStacks(sim, holyPower)
				},
			})
		}
	})

	core.MakePermanent(paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: PaladinTSEShock6P},
		Label:    label,
	}))
}

var ItemSetInquisitionBulwark = core.NewItemSet(core.ItemSet{
	ID:   1942,
	Name: "Inquisition Bulwark",
	Bonuses: map[int32]core.ApplyEffect{
		// Shield of Righteousness also increases your Block Value by 30% for 6 sec.
		2: func(agent core.Agent) {
			paladin := agent.(PaladinAgent).GetPaladin()
			paladin.applyScarletEnclaveProtection2PBonus()
		},
		// Shield of Righteousness deals percentage increased damage equal to your Block Chance.
		4: func(agent core.Agent) {
			paladin := agent.(PaladinAgent).GetPaladin()
			paladin.applyScarletEnclaveProtection4PBonus()
		},
		// Your Avenging Wrath no longer triggers Forbearance, lasts 15 sec longer, and increases your Block Value by 30%.
		6: func(agent core.Agent) {
			paladin := agent.(PaladinAgent).GetPaladin()
			paladin.applyScarletEnclaveProtection6PBonus()
		},
	},
})

// Shield of Righteousness also increases your Block Value by 30% for 6 sec.
func (paladin *Paladin) applyScarletEnclaveProtection2PBonus() {
	label := "S03 - Item - Scarlet Enclave - Paladin - Protection 2P Bonus"
	if paladin.HasAura(label) {
		return
	}

	blockValueModifier := paladin.NewDynamicMultiplyStat(stats.BlockValue, 1.3)

	righteousShieldAura := paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: 1226466},
		Label:    "Righteous Shield",
		Duration: time.Second * 6,
		OnGain: func(aura *core.Aura, sim *core.Simulation) {
			paladin.EnableDynamicStatDep(sim, blockValueModifier)
		},
		OnExpire: func(aura *core.Aura, sim *core.Simulation) {
			paladin.DisableDynamicStatDep(sim, blockValueModifier)
		},
	})

	core.MakePermanent(paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: PaladinTSEProt2P},
		Label:    label,
		OnSpellHitDealt: func(aura *core.Aura, sim *core.Simulation, spell *core.Spell, result *core.SpellResult) {
			if !spell.Matches(ClassSpellMask_PaladinShieldOfRighteousness) {
				return
			}

			if !result.Landed() {
				return
			}

			righteousShieldAura.Activate(sim)
		},
	}))
}

// Shield of Righteousness deals percentage increased damage equal to your Block Chance.
func (paladin *Paladin) applyScarletEnclaveProtection4PBonus() {
	label := "S03 - Item - Scarlet Enclave - Paladin - Protection 4P Bonus"
	if paladin.HasAura(label) {
		return
	}

	damageMod := paladin.AddDynamicMod(core.SpellModConfig{
		Kind:       core.SpellMod_DamageDone_Pct,
		ClassMask:  ClassSpellMask_PaladinShieldOfRighteousness,
		FloatValue: 1.0,
	})

	core.MakePermanent(paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: PaladinTSEProt4P},
		Label:    label,
		OnGain: func(aura *core.Aura, sim *core.Simulation) {
			damageMod.Activate()
		},
		OnExpire: func(aura *core.Aura, sim *core.Simulation) {
			damageMod.Deactivate()
		},
		OnApplyEffects: func(aura *core.Aura, sim *core.Simulation, target *core.Unit, spell *core.Spell) {
			if !spell.Matches(ClassSpellMask_PaladinShieldOfRighteousness) {
				return
			}

			damageMod.UpdateFloatValue(1.0 + (paladin.GetStat(stats.Block) / 100))
		},
	}))
}

// Your Avenging Wrath no longer triggers Forbearance, lasts 15 sec longer, and increases your Block Value by 30%.
func (paladin *Paladin) applyScarletEnclaveProtection6PBonus() {
	label := "S03 - Item - Scarlet Enclave - Paladin - Protection 6P Bonus"
	if paladin.HasAura(label) {
		return
	}

	paladin.bypassAvengingWrathForbearance = true

	blockValueModifier := paladin.NewDynamicMultiplyStat(stats.BlockValue, 1.3)

	avengingShield := paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: 1233525},
		Label:    "Avenging Shield",
		Duration: time.Second * 35,
		OnGain: func(aura *core.Aura, sim *core.Simulation) {
			paladin.EnableDynamicStatDep(sim, blockValueModifier)
		},
		OnExpire: func(aura *core.Aura, sim *core.Simulation) {
			paladin.DisableDynamicStatDep(sim, blockValueModifier)
		},
	})

	core.MakePermanent(paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: PaladinTSEProt6P},
		Label:    label,
		OnInit: func(aura *core.Aura, sim *core.Simulation) {
			paladin.avengingWrathAura.Duration += time.Second * 15
			paladin.avengingWrathAura.ApplyOnGain(func(aura *core.Aura, sim *core.Simulation) {
				avengingShield.Activate(sim)
			}).ApplyOnExpire(func(aura *core.Aura, sim *core.Simulation) {
				avengingShield.Deactivate(sim)
			})
		},
	}))
}

var ItemSetInquisitionArmor = core.NewItemSet(core.ItemSet{
	ID:   1941,
	Name: "Inquisition Armor",
	Bonuses: map[int32]core.ApplyEffect{
		// Lay on Hands also grants you 20% spell haste for 1 min.
		2: func(agent core.Agent) {
			paladin := agent.(PaladinAgent).GetPaladin()
			paladin.applyScarletEnclaveHoly2PBonus()
		},
		// Casting Holy Light, Flash of Light, or Divine Light on your Beacon of Light target causes you to gain 100% of the spell's base mana cost.
		4: func(agent core.Agent) {
		},
		// An additional 25% of your healing is transferred to your Beacon of Light target.
		6: func(agent core.Agent) {
		},
	},
})

// Lay on Hands also grants you 20% spell haste for 1 min.
func (paladin *Paladin) applyScarletEnclaveHoly2PBonus() {
	label := "S03 - Item - Scarlet Enclave - Paladin - Holy 2P Bonus"
	if paladin.HasAura(label) {
		return
	}

	emergencyAura := paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: 1226451},
		Label:    "Emergency",
		Duration: time.Second * 60,

		OnGain: func(aura *core.Aura, sim *core.Simulation) {
			paladin.MultiplyCastSpeed(1.20)
		},
		OnExpire: func(aura *core.Aura, sim *core.Simulation) {
			paladin.MultiplyCastSpeed(1 / 1.20)
		},
	})

	core.MakePermanent(paladin.RegisterAura(core.Aura{
		ActionID: core.ActionID{SpellID: PaladinTSEHoly2P},
		Label:    label,
		OnCastComplete: func(aura *core.Aura, sim *core.Simulation, spell *core.Spell) {
			if !spell.Matches(ClassSpellMask_PaladinLayOnHands) {
				return
			}

			emergencyAura.Activate(sim)
		},
	}))
}
