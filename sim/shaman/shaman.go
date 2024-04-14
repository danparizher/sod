package shaman

import (
	"time"

	"github.com/wowsims/sod/sim/core"
	"github.com/wowsims/sod/sim/core/proto"
	"github.com/wowsims/sod/sim/core/stats"
)

var TalentTreeSizes = [3]int{15, 16, 15}

const (
	SpellFlagElectric  = core.SpellFlagAgentReserved1
	SpellFlagTotem     = core.SpellFlagAgentReserved2
	SpellFlagFocusable = core.SpellFlagAgentReserved3
)

func NewShaman(character *core.Character, talents string, selfBuffs SelfBuffs) *Shaman {
	shaman := &Shaman{
		Character: *character,
		Talents:   &proto.ShamanTalents{},
		SelfBuffs: selfBuffs,
	}
	shaman.waterShieldManaMetrics = shaman.NewManaMetrics(core.ActionID{SpellID: int32(proto.ShamanRune_RuneHandsWaterShield)})

	core.FillTalentsProto(shaman.Talents.ProtoReflect(), talents, TalentTreeSizes)
	shaman.EnableManaBar()

	// Add Shaman stat dependencies
	shaman.AddStatDependency(stats.Strength, stats.AttackPower, 2)
	shaman.AddStatDependency(stats.Agility, stats.MeleeCrit, core.CritPerAgiAtLevel[character.Class][int(shaman.Level)]*core.CritRatingPerCritChance)
	shaman.AddStatDependency(stats.Intellect, stats.SpellCrit, core.CritPerIntAtLevel[character.Class][int(shaman.Level)]*core.SpellCritRatingPerCritChance)
	shaman.AddStatDependency(stats.BonusArmor, stats.Armor, 1)

	if selfBuffs.Shield == proto.ShamanShield_WaterShield {
		shaman.AddStat(stats.MP5, shaman.MaxMana()*.01)
	}

	shaman.ApplyRockbiterImbue(shaman.getImbueProcMask(character, proto.WeaponImbue_RockbiterWeapon))
	shaman.ApplyFlametongueImbue(shaman.getImbueProcMask(character, proto.WeaponImbue_FlametongueWeapon))
	shaman.ApplyFrostbrandImbue(shaman.getImbueProcMask(character, proto.WeaponImbue_FrostbrandWeapon))
	shaman.ApplyWindfuryImbue(shaman.getImbueProcMask(character, proto.WeaponImbue_WindfuryWeapon))

	return shaman
}

func (shaman *Shaman) getImbueProcMask(_ *core.Character, imbue proto.WeaponImbue) core.ProcMask {
	var mask core.ProcMask
	if shaman.HasMHWeapon() && shaman.Consumes.MainHandImbue == imbue {
		mask |= core.ProcMaskMeleeMH
	}
	if shaman.HasOHWeapon() && shaman.Consumes.OffHandImbue == imbue {
		mask |= core.ProcMaskMeleeOH
	}
	return mask
}

// Which buffs this shaman is using.
type SelfBuffs struct {
	Shield proto.ShamanShield
}

// Indexes into NextTotemDrops for self buffs
const (
	AirTotem int = iota
	EarthTotem
	FireTotem
	WaterTotem
)

const (
	SpellCode_ShamanNone int32 = iota
	SpellCode_ShamanLightningBolt
	SpellCode_ShamanChainLightning
	SpellCode_ShamanLavaBurst

	SpellCode_ShamanEarthShock
	SpellCode_ShamanFlameShock
	SpellCode_ShamanFrostShock

	SpellCode_ShamanMoltenBlast
	SpellCode_ShamanFireNova

	SpellCode_ShamanHealingWave
	SpellCode_ShamanLesserHealingWave
	SpellCode_ShamanChainHeal

	SpellCode_SearingTotem
	SpellCode_MagmaTotem
	SpellCode_FireNovaTotem
)

// Shaman represents a shaman character.
type Shaman struct {
	core.Character

	Talents   *proto.ShamanTalents
	SelfBuffs SelfBuffs

	Totems *proto.ShamanTotems

	// The expiration time of each totem (earth, air, fire, water).
	TotemExpirations [4]time.Duration

	LightningBolt         []*core.Spell
	LightningBoltOverload []*core.Spell

	ChainLightning         []*core.Spell
	ChainLightningOverload []*core.Spell

	Stormstrike *core.Spell

	LightningShield     []*core.Spell
	LightningShieldAura *core.Aura

	EarthShock     []*core.Spell
	FlameShock     []*core.Spell
	FlameShockDots []*core.Spell
	FrostShock     []*core.Spell

	// Totems
	ActiveTotems [4]*core.Spell

	StrengthOfEarthTotem []*core.Spell
	StoneskinTotem       []*core.Spell
	TremorTotem          *core.Spell

	SearingTotem  []*core.Spell
	MagmaTotem    []*core.Spell
	FireNovaTotem []*core.Spell

	HealingStreamTotem []*core.Spell
	ManaSpringTotem    []*core.Spell

	WindfuryTotem   []*core.Spell
	GraceOfAirTotem []*core.Spell

	// Healing Spells
	tidalWaveProc *core.Aura

	HealingWave         []*core.Spell
	HealingWaveOverload []*core.Spell

	LesserHealingWave []*core.Spell

	ChainHeal         []*core.Spell
	ChainHealOverload []*core.Spell

	waterShieldManaMetrics *core.ResourceMetrics

	// Runes
	EarthShield       *core.Spell
	FireNova          *core.Spell
	LavaBurst         *core.Spell
	LavaBurstOverload *core.Spell
	LavaLash          *core.Spell
	MoltenBlast       *core.Spell

	MaelstromWeaponAura *core.Aura
	PowerSurgeAura      *core.Aura

	// Used by Ancestral Guidance rune
	lastFlameShockTarget *core.Unit

	AncestralAwakening     *core.Spell
	ancestralHealingAmount float64
}

// Implemented by each Shaman spec.
type ShamanAgent interface {
	core.Agent

	// The Shaman controlled by this Agent.
	GetShaman() *Shaman
}

func (shaman *Shaman) GetCharacter() *core.Character {
	return &shaman.Character
}

func (shaman *Shaman) AddRaidBuffs(_ *proto.RaidBuffs) {
	// Buffs are handled explicitly through APLs now
}

func (shaman *Shaman) Initialize() {
	character := shaman.GetCharacter()

	// Core abilities
	shaman.registerChainLightningSpell()
	shaman.registerLightningBoltSpell()
	shaman.registerLightningShieldSpell()
	shaman.registerShocks()
	shaman.registerStormstrikeSpell()

	// Imbues
	// In the Initialize due to frost brand adding the aura to the enemy
	shaman.RegisterRockbiterImbue(shaman.getImbueProcMask(character, proto.WeaponImbue_RockbiterWeapon))
	shaman.RegisterFlametongueImbue(shaman.getImbueProcMask(character, proto.WeaponImbue_FlametongueWeapon))
	shaman.RegisterWindfuryImbue(shaman.getImbueProcMask(character, proto.WeaponImbue_WindfuryWeapon))
	shaman.RegisterFrostbrandImbue(shaman.getImbueProcMask(character, proto.WeaponImbue_FrostbrandWeapon))

	if shaman.ItemSwap.IsEnabled() {
		mh := shaman.ItemSwap.GetItem(proto.ItemSlot_ItemSlotMainHand)
		shaman.ApplyRockbiterImbueToItem(mh)
		oh := shaman.ItemSwap.GetItem(proto.ItemSlot_ItemSlotOffHand)
		shaman.ApplyRockbiterImbueToItem(oh)
	}

	// Totems
	shaman.registerStrengthOfEarthTotemSpell()
	shaman.registerStoneskinTotemSpell()
	shaman.registerTremorTotemSpell()
	shaman.registerSearingTotemSpell()
	shaman.registerMagmaTotemSpell()
	shaman.registerFireNovaTotemSpell()
	shaman.registerHealingStreamTotemSpell()
	shaman.registerManaSpringTotemSpell()
	shaman.registerWindfuryTotemSpell()
	shaman.registerGraceOfAirTotemSpell()

	// // This registration must come after all the totems are registered
	// shaman.registerCallOfTheElements()

	shaman.RegisterHealingSpells()
}

func (shaman *Shaman) RegisterHealingSpells() {
	shaman.registerLesserHealingWaveSpell()
	shaman.registerHealingWaveSpell()
	shaman.registerChainHealSpell()
}

func (shaman *Shaman) HasRune(rune proto.ShamanRune) bool {
	return shaman.HasRuneById(int32(rune))
}

func (shaman *Shaman) baseRuneAbilityDamage() float64 {
	return 7.583798 + 0.471881*float64(shaman.Level) + 0.036599*float64(shaman.Level*shaman.Level)
}

func (shaman *Shaman) Reset(_ *core.Simulation) {
	for i := range shaman.TotemExpirations {
		shaman.TotemExpirations[i] = 0
	}
}
