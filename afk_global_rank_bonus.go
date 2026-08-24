//go:build windows

package main

import (
	"fmt"
	"strings"
)

// Global-rank Starbit bonus: +10% for every authenticated Global EXP rank step.
// Bronze I starts at x1.00 and Master III reaches x2.70. Rank is deliberately
// the strongest broad permanent Starbit multiplier so global progression matters. Local EXP is never
// used here; if Global EXP has not synced yet the safe fallback is x1.00.
func afkGlobalRankName() string {
	if globalAccountEXPLoaded {
		return strings.ToUpper(strings.TrimSpace(rankForEXP(globalAccountEXP)))
	}
	if r, ok := localPlayerServerRank(); ok {
		return strings.ToUpper(strings.TrimSpace(r))
	}
	return "UNRANKED"
}

func afkGlobalRankMultiplierMilli() int64 {
	rank := afkGlobalRankName()
	if rank == "" || rank == "UNRANKED" {
		return 1000
	}
	for i, r := range expRanks {
		if strings.EqualFold(strings.TrimSpace(r.Name), rank) {
			return 1000 + int64(i)*100
		}
	}
	return 1000
}

func afkGlobalRankMultiplierText() string {
	return fmt.Sprintf("x%.2f", float64(afkGlobalRankMultiplierMilli())/1000.0)
}

func afkGlobalRankBonusPercent() int {
	return int((afkGlobalRankMultiplierMilli() - 1000) / 10)
}
