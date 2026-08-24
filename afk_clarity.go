//go:build windows

package main

import (
	"fmt"
	"strings"
)

const afkStarbaseMaxLevel = 7

// Starbase Level is a presentation layer over the authored progression gates.
// It gives players one obvious overall stage without replacing the individual
// facility tiers/research/ascension systems that already own balance.
func afkStarbaseLevel() int {
	level := 1
	if afkCoreTier() >= 2 {
		level = 2
	}
	if afkCoreTier() >= 3 && afkSection5IsComplete() {
		level = 3
	}
	if afkCoreTier() >= 4 && gameMeta.AFKSection6Complete {
		level = 4
	}
	if afkCoreTier() >= 5 && gameMeta.AFKSection8Complete {
		level = 5
	}
	if gameMeta.AFKSection9Complete && afkDroneBayTier() >= 3 {
		level = 6
	}
	if gameMeta.AFKSection10Complete {
		level = 7
	}
	return level
}

func afkStarbaseLevelName(level int) string {
	names := [...]string{"", "CALIBRATION", "EXPLORATION", "RESEARCH", "OPERATIONS", "INDUSTRIAL", "ORBITAL", "STELLAR"}
	if level < 1 {
		level = 1
	}
	if level >= len(names) {
		level = len(names) - 1
	}
	return names[level]
}

func afkStarbaseLevelNextUnlock(level int) string {
	switch level {
	case 1:
		return "SCOUT PAD + SPACE EXPEDITIONS"
	case 2:
		return "TECH LAB + RESEARCH"
	case 3:
		return "OPERATIONS + SPECIALISTS"
	case 4:
		return "FABRICATOR + DRONE NETWORK"
	case 5:
		return "ORBITAL EXTRACTOR"
	case 6:
		return "STELLAR ENDGAME SYSTEMS"
	default:
		return "ALL MAJOR STARBASE SYSTEMS ONLINE"
	}
}

func afkStarbaseNextObjectiveText() string {
	title, _, _, cur, target, complete := afkObjective()
	if complete {
		return "NETWORK COMPLETE // ALL MAJOR SYSTEMS ONLINE"
	}
	if target > 1 {
		return fmt.Sprintf("NEXT // %s // %s / %s", title, formatAFKNumber(cur), formatAFKNumber(target))
	}
	return "NEXT // " + title
}

func afkScoutDevelopmentSummary() string {
	parts := make([]string, 0, 5)
	if afkScoutTier() < 2 {
		parts = append(parts, fmt.Sprintf("SCOUT TIER %d/2", afkScoutTier()))
	}
	if gameMeta.AFKExpeditionsCompleted < 6 {
		parts = append(parts, fmt.Sprintf("EXPEDITIONS %d/6", gameMeta.AFKExpeditionsCompleted))
	}
	if afkScoutSpeedLevel() < 3 {
		parts = append(parts, fmt.Sprintf("SPEED LV.%d/3", afkScoutSpeedLevel()))
	}
	if afkScoutCapacityLevel() < 3 {
		parts = append(parts, fmt.Sprintf("SCOUT SYSTEMS LV.%d/3", afkScoutCapacityLevel()))
	}
	if afkScoutCacheLevel() < 3 {
		parts = append(parts, fmt.Sprintf("CACHE CHANCE LV.%d/3", afkScoutCacheLevel()))
	}
	if len(parts) == 0 {
		return "REQUIREMENT // SCOUT DEVELOPMENT COMPLETE"
	}
	return "REQUIREMENT // " + strings.Join(parts, " // ")
}

func afkFacilityRequirement(i int) string {
	if afkWorldBuilt(i) {
		switch i {
		case afkWorldCore:
			if afkCoreTier() >= afkCoreMaxTier {
				return "REQUIREMENT // MAXIMUM CORE TIER REACHED"
			}
			if afkCoreTierAdvanceAllowed() {
				return "REQUIREMENT // ENOUGH STARBITS"
			}
			switch afkCoreTier() {
			case 2:
				return afkScoutDevelopmentSummary()
			case 3:
				return "REQUIREMENT // COMPLETE TECH LAB DEVELOPMENT"
			case 4:
				return "REQUIREMENT // COMPLETE MODULE DEVELOPMENT"
			}
		case afkWorldScout:
			c, runs := afkNextScoutTierCost()
			if c <= 0 {
				return "REQUIREMENT // MAXIMUM SCOUT TIER REACHED"
			}
			return fmt.Sprintf("REQUIREMENT // %d NAV DATA + %d COMPLETED EXPEDITIONS", c, runs)
		case afkWorldDrone:
			if afkDroneBayTier() >= afkDroneBayMaxTier {
				return "REQUIREMENT // MAXIMUM DRONE BAY TIER REACHED"
			}
			if afkDroneBayCanAscend() {
				return "REQUIREMENT // ASCENSION GOALS COMPLETE + STARBITS"
			}
			return "REQUIREMENT // COMPLETE THE TIER GOALS SHOWN IN DRONE BAY"
		case afkWorldOrbital:
			if afkOrbitalTier() >= afkOrbitalMaxTier {
				return "REQUIREMENT // MAXIMUM ORBITAL TIER REACHED"
			}
			if afkOrbitalCanAscend() {
				return "REQUIREMENT // ASCENSION GOALS COMPLETE + STARBITS"
			}
			return "REQUIREMENT // COMPLETE THE TIER GOALS SHOWN IN ORBITAL"
		case afkWorldTech:
			return "PROGRESSION // BUY RESEARCH LEVELS IN TECH LAB"
		case afkWorldOps:
			return "PROGRESSION // RECRUIT + LEVEL OPERATORS"
		case afkWorldModules:
			return "PROGRESSION // FIND COMPONENTS + CRAFT MODULE SETS"
		}
	}

	switch i {
	case afkWorldCore:
		if !afkSection1IsComplete() {
			return "LOCKED // COLLECT 100 STARBITS FROM THE SINGULARITY"
		}
		return fmt.Sprintf("READY TO BUILD // COST %s STARBITS", formatAFKNumber(afkCursorCoreBuildCost))
	case afkWorldScout:
		if afkCoreTier() < 2 {
			return "LOCKED // REACH STARBASE LEVEL 2 // CURSOR CORE TIER 2"
		}
		return fmt.Sprintf("READY TO BUILD // COST %s STARBITS", formatAFKNumber(afkScoutShipUnlockCost))
	case afkWorldTech:
		if !afkSection5IsComplete() {
			return "LOCKED // " + afkScoutDevelopmentSummary()
		}
		if afkCoreTier() < 3 {
			return "LOCKED // REACH CURSOR CORE TIER 3"
		}
		return fmt.Sprintf("READY TO BUILD // COST %s STARBITS", formatAFKNumber(afkTechLabUnlockCost))
	case afkWorldOps:
		if afkCoreTier() < 4 {
			return "LOCKED // REACH CURSOR CORE TIER 4"
		}
		if !gameMeta.AFKSection6Complete {
			return "LOCKED // COMPLETE TECH LAB DEVELOPMENT"
		}
		return "AVAILABLE // OPEN OPERATIONS"
	case afkWorldModules:
		if afkCoreTier() < 4 {
			return "LOCKED // REACH CURSOR CORE TIER 4"
		}
		if !gameMeta.AFKSection7Complete {
			return "LOCKED // COMPLETE OPERATOR DEVELOPMENT"
		}
		return "AVAILABLE // OPEN FABRICATOR"
	case afkWorldDrone:
		if afkCoreTier() < 5 {
			return "LOCKED // REACH CURSOR CORE TIER 5"
		}
		if !gameMeta.AFKSection8Complete {
			return "LOCKED // COMPLETE MODULE DEVELOPMENT"
		}
		return fmt.Sprintf("READY TO BUILD // COST %s STARBITS", formatAFKNumber(afkDroneBayBuildCost))
	case afkWorldOrbital:
		if afkDroneBayTier() < 3 {
			return "LOCKED // UPGRADE DRONE BAY TO TIER 3"
		}
		if !gameMeta.AFKSection9Complete {
			return "LOCKED // COMPLETE DRONE NETWORK DEVELOPMENT"
		}
		return fmt.Sprintf("READY TO BUILD // COST %s STARBITS", formatAFKNumber(afkOrbitalBuildCost))
	}
	return "LOCKED // REQUIREMENT NOT MET"
}

func afkFacilityContextAction(i int) (label string, cost int64, enabled bool) {
	switch i {
	case afkWorldCore:
		if !afkCursorCoreBuilt() {
			return "BUILD CURSOR CORE", afkCursorCoreBuildCost, afkSection1IsComplete() && gameMeta.AFKStarbits >= afkCursorCoreBuildCost
		}
		c := afkNextCoreTierCost()
		if c <= 0 {
			return "CURSOR CORE // MAX TIER", 0, false
		}
		if !afkCoreTierAdvanceAllowed() {
			switch afkCoreTier() {
			case 2:
				return "SCOUT DEVELOPMENT REQUIRED", c, false
			case 3:
				return "TECH LAB DEVELOPMENT REQUIRED", c, false
			case 4:
				return "MODULE DEVELOPMENT REQUIRED", c, false
			}
		}
		if gameMeta.AFKStarbits < c {
			return fmt.Sprintf("NEED %s STARBITS", formatAFKNumber(c-gameMeta.AFKStarbits)), c, false
		}
		return fmt.Sprintf("UPGRADE CURSOR CORE // TIER %d", afkCoreTier()+1), c, true
	case afkWorldScout:
		if !afkScoutUnlocked() {
			return "BUILD SCOUT PAD", afkScoutShipUnlockCost, afkCoreTier() >= 2 && gameMeta.AFKStarbits >= afkScoutShipUnlockCost
		}
		c, _ := afkNextScoutTierCost()
		if c <= 0 {
			return "SCOUT PAD // MAX TIER", 0, false
		}
		return fmt.Sprintf("UPGRADE SCOUT PAD // TIER %d", afkScoutTier()+1), 0, afkCanUpgradeScoutTier()
	case afkWorldTech:
		if !afkTechLabUnlocked() {
			return "BUILD TECH LAB", afkTechLabUnlockCost, afkTechLabAvailable() && gameMeta.AFKStarbits >= afkTechLabUnlockCost
		}
		return "OPEN TECH LAB // RESEARCH UPGRADES", 0, true
	case afkWorldOps:
		if !afkOperatorsAvailable() {
			return "OPERATIONS // REQUIREMENTS NOT MET", 0, false
		}
		return "OPEN OPERATIONS // MANAGE OPERATORS", 0, true
	case afkWorldModules:
		if !afkModulesAvailable() {
			return "FABRICATOR // REQUIREMENTS NOT MET", 0, false
		}
		return "OPEN FABRICATOR // CRAFT MODULES", 0, true
	case afkWorldDrone:
		if !afkDroneBayBuilt() {
			return "BUILD DRONE BAY // TIER 1", afkDroneBayBuildCost, afkDronesAvailable() && gameMeta.AFKStarbits >= afkDroneBayBuildCost
		}
		c := afkDroneBayAscendCost()
		if c <= 0 {
			return "DRONE BAY // MAX TIER", 0, false
		}
		return fmt.Sprintf("ASCEND DRONE BAY // TIER %d", afkDroneBayTier()+1), c, afkDroneBayCanAscend() && gameMeta.AFKStarbits >= c
	case afkWorldOrbital:
		if !afkOrbitalBuilt() {
			return "BUILD ORBITAL EXTRACTOR // TIER 1", afkOrbitalBuildCost, afkOrbitalAvailable() && gameMeta.AFKStarbits >= afkOrbitalBuildCost
		}
		c := afkOrbitalAscendCost()
		if c <= 0 {
			return "ORBITAL EXTRACTOR // MAX TIER", 0, false
		}
		return fmt.Sprintf("ASCEND ORBITAL EXTRACTOR // TIER %d", afkOrbitalTier()+1), c, afkOrbitalCanAscend() && gameMeta.AFKStarbits >= c
	}
	return "SELECT A FACILITY", 0, false
}
