//go:build windows

package main

import "time"

var afkLastDroneMiningSFX time.Time

func playAFKStarbitCollectSound()   { playSFX("afk_starbit_collect") }
func playAFKConstructionSound()     { playSFX("afk_construction") }
func playAFKTierUpSound()           { playSFX("afk_tier_up") }
func playAFKExpeditionLaunchSound() { playSFX("afk_expedition_launch") }
func playAFKResearchSound()         { playSFX("afk_research_complete") }
func playAFKOperatorSound()         { playSFX("afk_operator_recruited") }
func playAFKModuleCraftedSound()    { playSFX("afk_module_crafted") }
func playAFKDroneDeploySound()      { playSFX("afk_drone_deploy") }
func playAFKOverdriveSound()        { playSFX("afk_overdrive") }
func playAFKOrbitalChargeSound()    { playSFX("afk_orbital_charge") }
func playAFKOrbitalFireSound()      { playSFX("afk_orbital_fire") }
func playAFKPrestigeSound()         { playSFX("afk_prestige") }
func playAFKUpgradeSound()          { playSFX("afk_upgrade") }
func playAFKCollectItemSound()      { playSFX("afk_collect_item") }

func afkMaybePlayDroneMiningSound(now time.Time) {
	if overlayMode != OverlayAFKSingularity || !afkDroneBayBuilt() || gameMeta.AFKDronesDeployed <= 0 {
		return
	}
	gap := 1450*time.Millisecond - time.Duration((gameMeta.AFKDronesDeployed%4)*120)*time.Millisecond
	if gap < 900*time.Millisecond {
		gap = 900 * time.Millisecond
	}
	if afkLastDroneMiningSFX.IsZero() || now.Sub(afkLastDroneMiningSFX) >= gap {
		afkLastDroneMiningSFX = now
		playSFX("afk_drone_mining")
	}
}
