from pathlib import Path
R=Path(__file__).resolve().parents[1]

def text(n): return (R/n).read_text(errors='ignore')
def ck(name, cond):
    if not cond: raise AssertionError(name)
    print('PASS', name)

tal=text('afk_research_talents.go'); guide=text('afk_starbase_hub.go'); cloud=text('afk_cloud_sync.go')
ck('fourteen scarce milestones', tal.count('{"') >= 14 and 'REACH STELLAR PRESTIGE 3' in tal)
ck('no old dense research thresholds', '[]int{5, 12, 20, 30, 40, 50, 60, 70, 80, 90, 100}' not in tal)
ck('milestones span systems', all(x in tal for x in ['REACH 20 TOTAL RESEARCH LEVELS','RECRUIT 3 OPERATORS','LEVEL ANY OPERATOR TO 7','SCOUT TIER 3 + 20 EXPEDITIONS','BUILD DRONE BAY TIER 2','BUILD ORBITAL EXTRACTOR TIER 2','REACH STELLAR PRESTIGE 3']))
ck('legacy unspent point pool can shrink', 'oversized pool of' in tal and 'earned < gameMeta.AFKTalentPointsSpent' in tal)
ck('owned skills remain grandfathered', 'gameMeta.AFKTalentPointsSpent' in tal and 'gameMeta.AFKTalentsUnlocked' in tal)
ck('cloud earned points no longer monotonic', 'afkCloudKeepHigherInt(state, "afk_talent_points_earned"' not in cloud and 'afkRefreshTalentPointAwards()' in cloud)
ck('pilot UI explains source', 'EARN 1 POINT PER MAJOR MILESTONE' in tal and 'NEXT SKILL POINT //' in tal and 'MILESTONES %d / %d' in tal)
ck('guide explains scarce milestone points', 'PILOT SKILL POINTS // MILESTONES ONLY' in guide and 'SKILL POINTS ARE RARE' in guide)
ck('tree canvas leaves room for point explanation', 'p.Top + sy(94, hgt)' in tal)
print('\nPASS v443 Skill Point milestone regression')
