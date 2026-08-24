//go:build windows

package main

// v129 Endurance meteor shield collision hardening.

import (
	"encoding/json"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	HALFTONE          = 4
	CLEARTYPE_QUALITY = 5

	CS_HREDRAW = 0x0002
	CS_VREDRAW = 0x0001
	IDC_ARROW  = 32512

	WS_CAPTION      = 0x00C00000
	WS_SYSMENU      = 0x00080000
	WS_MINIMIZEBOX  = 0x00020000
	WS_VISIBLE      = 0x10000000
	WS_DISABLED     = 0x08000000
	WS_CHILD        = 0x40000000
	WS_CLIPCHILDREN = 0x02000000
	WS_CLIPSIBLINGS = 0x04000000
	WINDOW_STYLE    = WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX | WS_VISIBLE | WS_CLIPCHILDREN
	CW_USEDEFAULT   = 0x80000000
	SW_HIDE         = 0
	SW_SHOW         = 5

	WM_DESTROY          = 0x0002
	WM_CLOSE            = 0x0010
	WM_PAINT            = 0x000F
	WM_MOUSEMOVE        = 0x0200
	WM_MOUSEWHEEL       = 0x020A
	WM_LBUTTONDOWN      = 0x0201
	WM_RBUTTONDOWN      = 0x0204
	WM_RBUTTONUP        = 0x0205
	WM_LBUTTONUP        = 0x0202
	WM_KEYDOWN          = 0x0100
	WM_CHAR             = 0x0102
	WM_TIMER            = 0x0113
	WM_ERASEBKGND       = 0x0014
	WM_SETCURSOR        = 0x0020
	WM_NCHITTEST        = 0x0084
	WM_QUIT             = 0x0012
	WM_APP              = 0x8000
	WM_SURVIVAL_RESPAWN = WM_APP + 17
	WM_MAIN_THREAD_TASK = WM_APP + 18
	PM_REMOVE           = 0x0001

	MB_YESNO       = 0x00000004
	MB_ICONWARNING = 0x00000030
	IDYES          = 6

	SRCCOPY = 0x00CC0020

	VK_ESCAPE = 0x1B
	VK_TAB    = 0x09
	VK_RETURN = 0x0D
	VK_SPACE  = 0x20
	VK_SHIFT  = 0x10
	VK_F2     = 0x71
	VK_LEFT   = 0x25
	VK_UP     = 0x26
	VK_RIGHT  = 0x27
	VK_DOWN   = 0x28

	PS_SOLID     = 0
	PS_GEOMETRIC = 0x00010000
	BS_SOLID     = 0
	TRANSPARENT  = 1

	TIMER_GAME         = 1
	TIMER_FAIL_RESET   = 2
	TIMER_RESULT_RESET = 3
	TIMER_LEVELUP      = 4
	TIMER_INTRO        = 5
	TIMER_PARTICLES    = 6
	TIMER_UI           = 7
	TIMER_LIVE         = 8
	TIMER_LIVE_SYSTEMS = 9
	TIMER_FAIL_ANIM    = 10
	TIMER_STARBASE     = 11

	DIB_RGB_COLORS = 0
	BI_RGB         = 0
	AC_SRC_OVER    = 0x00
	AC_SRC_ALPHA   = 0x01
)

type BLENDFUNCTION struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type SIZE struct{ Cx, Cy int32 }
type FPoint struct{ X, Y float64 }

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type D2D1PointF struct{ X, Y float32 }
type D2D1RectF struct{ Left, Top, Right, Bottom float32 }
type D2D1Ellipse struct {
	Point   D2D1PointF
	RadiusX float32
	RadiusY float32
}
type D2D1ColorF struct{ R, G, B, A float32 }
type D2D1PixelFormat struct {
	Format    uint32
	AlphaMode uint32
}
type D2D1RenderTargetProperties struct {
	Type        uint32
	PixelFormat D2D1PixelFormat
	DpiX        float32
	DpiY        float32
	Usage       uint32
	MinLevel    uint32
}
type D2D1SizeU struct{ Width, Height uint32 }
type D2D1HwndRenderTargetProperties struct {
	Hwnd           uintptr
	PixelSize      D2D1SizeU
	PresentOptions uint32
}
type D2D1BitmapProperties struct {
	PixelFormat D2D1PixelFormat
	DpiX        float32
	DpiY        float32
}
type D2D1StrokeStyleProperties struct {
	StartCap   uint32
	EndCap     uint32
	DashCap    uint32
	LineJoin   uint32
	MiterLimit float32
	DashStyle  uint32
	DashOffset float32
}
type D2D1Matrix3x2F struct {
	M11, M12, M21, M22, Dx, Dy float32
}
type EnduranceD2DGeometry struct {
	Geometry uintptr
	MinX     float64
	MaxX     float64
}

type CachedBGRASprite struct {
	DC     uintptr
	Bitmap uintptr
	Old    uintptr
	W      int32
	H      int32
	Ready  bool
}

type GDIPStartupInput struct {
	GdiplusVersion           uint32
	DebugEventCallback       uintptr
	SuppressBackgroundThread int32
	SuppressExternalCodecs   int32
}
type GDIPPointF struct{ X, Y float32 }

type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}
type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}
type MSG struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}
type LOGBRUSH struct {
	LbStyle uint32
	LbColor uint32
	LbHatch uintptr
}
type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}
type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]uint32
}

type DiffSettings struct {
	name     string
	controls int
	width    float64
	wiggle   float64
	color    uintptr
}

type GameState int

const (
	StateWaiting GameState = iota
	StatePlaying
	StateFailed
	StateResult
	StateIntro
)

type EnduranceBlock struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
	Speed  float64
	Orange bool
}

type EnduranceAlienMinion struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
	Speed  float64
}

const (
	alienBossIdle = iota
	alienBossWarning
	alienBossEntering
	alienBossAim1
	alienBossLaser1Extend
	alienBossLaser1Hold
	alienBossLaser1Retract
	alienBossReposition
	alienBossAim2
	alienBossLaser2Extend
	alienBossLaser2Hold
	alienBossLaser2Retract
	alienBossReposition2
	alienBossAim3
	alienBossLaser3Extend
	alienBossLaser3Hold
	alienBossLaser3Retract
	alienBossReposition3
	alienBossAim4
	alienBossLaser4Extend
	alienBossLaser4Hold
	alienBossLaser4Retract
	alienBossExiting
	alienBossDone
)

const (
	endurancePowerupDistance = iota
	endurancePowerupShield
	endurancePowerupSlow
)

type EndurancePowerup struct {
	Point FPoint // static world coordinate; camera translation is applied while drawing
	Kind  int
}

// TargetExplosion is a lightweight renderer-only burst used when an Endurance
// target is destroyed. It has no collision and deliberately expires quickly.
type TargetExplosion struct {
	Point   FPoint
	Started time.Time
	Seed    float64
}

type Target struct {
	Point     FPoint
	Index     int
	Clicked   bool
	MoveRange int
	MinIndex  int
	MaxIndex  int
	Phase     float64
	Speed     float64
}

type ResultData struct {
	Time        float64
	TrackingAcc float64
	TargetAcc   float64
	CombinedAcc float64
	TargetsHit  int
	TargetCount int
	RoundPoints int
	TotalScore  int
	Streak      int
	Combo       float64
	Rating      string
	Rank        string
	EXPEarned   int
	TotalEXP    int
	Course      string
	Difficulty  string
	Distance    float64
	CoinsEarned int
}

type LeaderboardEntry struct {
	Name       string  `json:"name"`
	Score      int     `json:"score"`
	Streak     int     `json:"streak"`
	Accuracy   float64 `json:"accuracy"`
	Difficulty string  `json:"difficulty"`
	Rank       string  `json:"rank"`
	Date       string  `json:"date"`
	Time       string  `json:"time"`
	Distance   float64 `json:"distance,omitempty"`
	TargetsHit int     `json:"targets_hit,omitempty"`
}

type GlobalLeaderboardEntry struct {
	UserID       string
	Position     int
	Name         string
	NameColour   int
	Score        int
	Streak       int
	Accuracy     float64
	Difficulty   string
	Rank         string
	Distance     float64
	TargetsHit   int
	EasyClears   int
	NormalClears int
	HardClears   int
	InsaneClears int
	TotalClears  int
	AchievedAt   string
}

type RemoteProfileData struct {
	UserID                  string           `json:"user_id"`
	DisplayName             string           `json:"display_name"`
	AvatarURL               string           `json:"avatar_url"`
	CreatedAt               string           `json:"created_at"`
	EXP                     int              `json:"exp"`
	EXPRank                 string           `json:"exp_rank"`
	EasyClears              int              `json:"easy_clears"`
	NormalClears            int              `json:"normal_clears"`
	HardClears              int              `json:"hard_clears"`
	InsaneClears            int              `json:"insane_clears"`
	TotalClears             int              `json:"total_clears"`
	UnlockedShips           []int            `json:"unlocked_ships"`
	SelectedShip            int              `json:"selected_ship"`
	UnlockedTitles          []string         `json:"unlocked_titles"`
	SelectedTitle           string           `json:"selected_title"`
	UnlockedNameColours     []int            `json:"unlocked_name_colours"`
	UnlockedProfileFrames   []int            `json:"unlocked_profile_frames"`
	SelectedNameColour      int              `json:"selected_name_colour"`
	SelectedProfileFrame    int              `json:"selected_profile_frame"`
	SelectedProfileFont     int              `json:"selected_profile_font"`
	SelectedProfileNameFont int              `json:"selected_profile_name_font"`
	ProfilePrimaryColour    int              `json:"selected_profile_primary_colour"`
	ProfileSecondaryColour  int              `json:"selected_profile_secondary_colour"`
	ProfileNameShadow       bool             `json:"profile_name_shadow"`
	ProfileShadowColour     int              `json:"profile_shadow_colour"`
	ProfileGradientVertical bool             `json:"profile_gradient_vertical"`
	ProfileAnimation        int              `json:"selected_profile_animation"`
	CompetitiveBadge        string           `json:"competitive_badge"`
	SeasonBest              string           `json:"season_best"`
	AchievementShowcase     []string         `json:"achievement_showcase"`
	BestSurvivalWave        int              `json:"best_survival_wave"`
	BestSurvivalKills       int              `json:"best_survival_kills"`
	Positions               map[string]int   `json:"positions"`
	Scores                  []map[string]any `json:"scores"`
}

type LiveAnnouncement struct {
	ID        int64  `json:"id"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type PlayerProgress struct {
	EasyCompleted   int `json:"easy_completed"`
	NormalCompleted int `json:"normal_completed"`
	HardCompleted   int `json:"hard_completed"`
	InsaneCompleted int `json:"insane_completed"`
	EXP             int `json:"exp"`
}

// GameMeta stores commercial-polish systems independently of the original
// progression file so existing v36 player progression remains compatible.
type GameMeta struct {
	FirstLaunchDone bool `json:"first_launch_done"`

	TotalRuns             int     `json:"total_runs"`
	TotalClears           int     `json:"total_clears"`
	TotalFailures         int     `json:"total_failures"`
	TargetsHit            int     `json:"targets_hit"`
	BestAccuracy          float64 `json:"best_accuracy"`
	BestStreakEver        int     `json:"best_streak_ever"`
	BestEnduranceDistance float64 `json:"best_endurance_distance"`
	PlaySeconds           int64   `json:"play_seconds"`
	Sessions              int     `json:"sessions"`

	Achievements          []string `json:"achievements"`
	AchievementEXPGranted []string `json:"achievement_exp_granted"`

	DailyDate           string `json:"daily_date"`
	DailyClears         int    `json:"daily_clears"`
	DailyHardClears     int    `json:"daily_hard_clears"`
	DailyHighAcc        int    `json:"daily_high_acc"`
	DailyClearsRewarded bool   `json:"daily_clears_rewarded"`
	DailyHardRewarded   bool   `json:"daily_hard_rewarded"`
	DailyAccRewarded    bool   `json:"daily_acc_rewarded"`
	DailyRewarded       bool   `json:"daily_rewarded"`

	WeeklyKey        string `json:"weekly_key"`
	WeeklyClears     int    `json:"weekly_clears"`
	WeeklyInsane     int    `json:"weekly_insane"`
	WeeklyBestStreak int    `json:"weekly_best_streak"`
	WeeklyRewarded   bool   `json:"weekly_rewarded"`

	ParticleQuality             int      `json:"particle_quality"` // 0 off, 1 low, 2 high
	FPSMode                     int      `json:"fps_mode"`         // 0 60, 1 120, 2 unlimited
	ReducedMotion               bool     `json:"reduced_motion"`
	MusicVolume                 int      `json:"music_volume"`
	EffectsVolume               int      `json:"effects_volume"`
	FontOverride                int      `json:"font_override"`         // 0-7 selectable UI typeface
	HUDCornerStyle              int      `json:"hud_corner_style"`      // 0 sharp,1 compact,2 sci-fi,3 industrial
	HUDBackgroundTheme          int      `json:"hud_background_theme"`  // 0 default,1 dark glass,2 military,3 terminal,4 minimal,5 industrial
	EXPBarAnimation             int      `json:"exp_bar_animation"`     // 0 static,1 pulse,2 flowing highlight
	BossHPBarTheme              int      `json:"boss_hp_bar_theme"`     // 0 boss-specific,1 red plasma,2 segmented arcade,3 minimalist
	ButtonHoverEffect           int      `json:"button_hover_effect"`   // 0 brighten,1 outline,2 glow,3 pulse
	AnnouncementTheme           int      `json:"announcement_theme"`    // 0 sci-fi,1 warning,2 hologram,3 minimal,4 industrial,5 neon
	ScreenShakeStrength         int      `json:"screen_shake_strength"` // 0 normal,1 off,2 low,3 high
	FailureSound                int      `json:"failure_sound"`         // 0 Default, 1 Fortnite, 2 Roblox, 3 Minecraft, 4 Among Us
	CrosshairStyle              int      `json:"crosshair_style"`       // 0 plus,1 dot,2 cross,3 circle-dot,4 sniper
	CrosshairSize               int      `json:"crosshair_size"`        // 0 small,1 medium,2 large
	CrosshairColor              int      `json:"crosshair_color"`       // black,white,cyan,green,red,yellow,purple
	MovingBackground            bool     `json:"moving_background"`
	ShowShipHitbox              bool     `json:"show_ship_hitbox"`
	ShareAnonymousAnalytics     bool     `json:"share_anonymous_analytics"`
	FirstPlayedDate             string   `json:"first_played_date"`
	LastPlayedDate              string   `json:"last_played_date"`
	ActivePlayDates             []string `json:"active_play_dates"`
	CompetitiveBadge            string   `json:"competitive_badge"`
	SeasonBest                  string   `json:"season_best"`
	MovingBackgroundV79Migrated bool     `json:"moving_background_v104_migrated"`

	// v59 fair-rank migration: preserve historic clears/stats, reset legacy EXP once.
	RankResetV59Applied             bool `json:"rank_reset_v59_applied"`
	AchievementEXPTripleV105Applied bool `json:"achievement_exp_triple_v105_applied"`

	// Endurance space economy / garage.
	SpaceCoins                           int    `json:"space_coins"`
	PrecisionCompetitionActiveDifficulty string `json:"precision_competition_active_difficulty,omitempty"`

	// Cursor Control AFK / Singularity progression. Every progression value is
	// explicit and table-driven so later sections can extend the system without
	// changing the meaning of earlier saves.
	AFKStarbits                   int64 `json:"afk_starbits"`
	AFKOperatorWaitNoticeSeenUnix int64 `json:"afk_operator_wait_notice_seen_unix"`
	AFKSection1Complete           bool  `json:"afk_section1_complete"`
	AFKCursorCoreBuilt            bool  `json:"afk_cursor_core_built"`
	AFKCursorCoreTier             int   `json:"afk_cursor_core_tier"`
	AFKAutoCursors                int   `json:"afk_auto_cursors"`
	AFKCursorProcessingLevel      int   `json:"afk_cursor_processing_level"`
	AFKStarbitRemainderMilli      int64 `json:"afk_starbit_remainder_milli"`
	AFKStarbitsMigrated           bool  `json:"afk_starbits_migrated"`
	// Legacy pre-Starbits AFK fields are read once for save compatibility, then cleared.
	AFKLegacyCursorEnergy        int64  `json:"afk_cursor_energy,omitempty"`
	AFKLegacyEnergyRemainder     int64  `json:"afk_energy_remainder_milli,omitempty"`
	AFKLegacyLifetimeEnergy      int64  `json:"afk_lifetime_cursor_energy,omitempty"`
	AFKLegacyOfflineEnergy       int64  `json:"afk_offline_pending_energy,omitempty"`
	AFKLegacyExpeditionEnergy    int64  `json:"afk_expedition_pending_energy,omitempty"`
	AFKSection2Complete          bool   `json:"afk_section2_complete"`
	AFKSection3Complete          bool   `json:"afk_section3_complete"`
	AFKSection4Complete          bool   `json:"afk_section4_complete"`
	AFKLastSavedUnix             int64  `json:"afk_last_saved_unix"`
	AFKMaxObservedUnix           int64  `json:"afk_max_observed_unix"`
	AFKOfflinePendingAwaySeconds int64  `json:"afk_offline_pending_away_seconds"`
	AFKOfflinePendingPaidSeconds int64  `json:"afk_offline_pending_paid_seconds"`
	AFKOfflinePendingStarbits    int64  `json:"afk_offline_pending_starbits"`
	AFKOfflinePendingSpaceCoins  int    `json:"afk_offline_pending_space_coins"`
	AFKClaimableSpaceCoins       int    `json:"afk_claimable_space_coins"`
	AFKSpaceCoinRemainderUnits   int64  `json:"afk_space_coin_remainder_units"`
	AFKLifetimeAFKSpaceCoins     int    `json:"afk_lifetime_space_coins"`
	AFKOfflineClaims             int    `json:"afk_offline_claims"`
	AFKClockRollbackCount        int    `json:"afk_clock_rollback_count"`
	AFKSection5Complete          bool   `json:"afk_section5_complete"`
	AFKScoutShipUnlocked         bool   `json:"afk_scout_ship_unlocked"`
	AFKScoutShipTier             int    `json:"afk_scout_ship_tier"`
	AFKExpeditionSpeedLevel      int    `json:"afk_expedition_speed_level"`
	AFKCapacityLevel             int    `json:"afk_capacity_level"`
	AFKCacheChanceLevel          int    `json:"afk_cache_chance_level"`
	AFKNavigationData            int64  `json:"afk_navigation_data"`
	AFKExpeditionsCompleted      int    `json:"afk_expeditions_completed"`
	AFKExpeditionSerial          int    `json:"afk_expedition_serial"`
	AFKExpeditionDestination     int    `json:"afk_expedition_destination"`
	AFKExpeditionShipID          int    `json:"afk_expedition_ship_id"`
	AFKExpeditionFireColorID     int    `json:"afk_expedition_fire_color_id"`
	AFKExpeditionFireSizeID      int    `json:"afk_expedition_fire_size_id"`
	AFKExpeditionStartedUnix     int64  `json:"afk_expedition_started_unix"`
	AFKExpeditionEndsUnix        int64  `json:"afk_expedition_ends_unix"`
	AFKExpeditionPendingStarbits int64  `json:"afk_expedition_pending_starbits"`
	AFKExpeditionPendingData     int64  `json:"afk_expedition_pending_data"`
	AFKExpeditionPendingCache    bool   `json:"afk_expedition_pending_cache"`
	AFKSpaceCacheRarity          int    `json:"afk_space_cache_rarity"`
	AFKSpaceCachesClaimed        int    `json:"afk_space_caches_claimed"`
	AFKNextActiveCacheUnix       int64  `json:"afk_next_active_cache_unix"`
	AFKActiveCacheReward         int64  `json:"afk_active_cache_reward"`
	AFKActiveCachesClaimed       int    `json:"afk_active_caches_claimed"`
	AFKOverdriveUntilUnix        int64  `json:"afk_overdrive_until_unix"`
	AFKOverdriveReadyUnix        int64  `json:"afk_overdrive_ready_unix"`
	AFKSection6Complete          bool   `json:"afk_section6_complete"`
	AFKTechLabUnlocked           bool   `json:"afk_tech_lab_unlocked"`
	AFKResearchLevels            []int  `json:"afk_research_levels"`
	AFKResearchPurchases         int    `json:"afk_research_purchases"`
	AFKTalentPointsEarned        int    `json:"afk_talent_points_earned"`
	AFKTalentPointsSpent         int    `json:"afk_talent_points_spent"`
	AFKTalentsUnlocked           []int  `json:"afk_talents_unlocked"`
	AFKSection7Complete          bool   `json:"afk_section7_complete"`
	AFKOperatorsRecruited        []bool `json:"afk_operators_recruited"`
	// AFKOperatorXPSeconds is retained as the persisted Service XP pool for save/cloud
	// compatibility. v442 no longer interprets this value as elapsed time.
	AFKOperatorXPSeconds        []int64  `json:"afk_operator_xp_seconds"`
	AFKOperatorLevels           []int    `json:"afk_operator_levels"`
	AFKOperatorWorkStartedUnix  []int64  `json:"afk_operator_work_started_unix"`
	AFKOperatorWorkEndsUnix     []int64  `json:"afk_operator_work_ends_unix"`
	AFKOperatorCooldownEndsUnix []int64  `json:"afk_operator_cooldown_ends_unix"`
	AFKOperatorsRecruitedCount  int      `json:"afk_operators_recruited_count"`
	AFKSection8Complete         bool     `json:"afk_section8_complete"`
	AFKCraftComponents          []int    `json:"afk_craft_components"`
	AFKEquipmentCrafted         []bool   `json:"afk_equipment_crafted"`
	AFKEquipmentSetsComplete    int      `json:"afk_equipment_sets_complete"`
	AFKCraftComponentsFound     int      `json:"afk_craft_components_found"`
	AFKPendingComponent         int      `json:"afk_pending_component"`
	AFKPendingComponentCount    int      `json:"afk_pending_component_count"`
	AFKEquipmentAllSetsReward   bool     `json:"afk_equipment_all_sets_reward"`
	AFKSection9Complete         bool     `json:"afk_section9_complete"`
	AFKDroneBayBuilt            bool     `json:"afk_drone_bay_built"`
	AFKDroneBayTier             int      `json:"afk_drone_bay_tier"`
	AFKDronesDeployed           int      `json:"afk_drones_deployed"`
	AFKDroneUpgradeLevels       []int    `json:"afk_drone_upgrade_levels"`
	AFKDroneTargetsHit          int64    `json:"afk_drone_targets_hit"`
	AFKSection10Complete        bool     `json:"afk_section10_complete"`
	AFKOrbitalExtractorBuilt    bool     `json:"afk_orbital_extractor_built"`
	AFKOrbitalExtractorTier     int      `json:"afk_orbital_extractor_tier"`
	AFKOrbitalUpgradeLevels     []int    `json:"afk_orbital_upgrade_levels"`
	AFKOrbitalLastFireUnix      int64    `json:"afk_orbital_last_fire_unix"`
	AFKOrbitalBurstsFired       int64    `json:"afk_orbital_bursts_fired"`
	AFKSection11Complete        bool     `json:"afk_section11_complete"`
	AFKPrestigeRank             int      `json:"afk_prestige_rank"`
	AFKPrestigePurchases        int      `json:"afk_prestige_purchases"`
	AFKSection12Complete        bool     `json:"afk_section12_complete"`
	AFKStationHPBonus           int      `json:"afk_station_hp_bonus"`
	UnlockedShips               []int    `json:"unlocked_ships"`
	SelectedShip                int      `json:"selected_ship"`
	UnlockedFireColors          []int    `json:"unlocked_fire_colors"`
	SelectedFireColor           int      `json:"selected_fire_color"`
	UnlockedFireSizes           []int    `json:"unlocked_fire_sizes"`
	SelectedFireSize            int      `json:"selected_fire_size"`
	UnlockedTitles              []string `json:"unlocked_titles"`
	SelectedTitle               string   `json:"selected_title"`
	UnlockedNameColours         []int    `json:"unlocked_name_colours"`
	SelectedNameColour          int      `json:"selected_name_colour"`
	UnlockedProfileFrames       []int    `json:"unlocked_profile_frames"`
	SelectedProfileFrame        int      `json:"selected_profile_frame"`
	SelectedProfileFont         int      `json:"selected_profile_font"`
	SelectedProfileNameFont     int      `json:"selected_profile_name_font"`
	ProfilePrimaryColour        int      `json:"selected_profile_primary_colour"`
	ProfileSecondaryColour      int      `json:"selected_profile_secondary_colour"`
	ProfileNameShadow           bool     `json:"profile_name_shadow"`
	ProfileShadowColour         int      `json:"profile_shadow_colour"`
	ProfileGradientVertical     bool     `json:"profile_gradient_vertical"`
	ProfileAnimation            int      `json:"selected_profile_animation"`
	AchievementShowcase         []string `json:"achievement_showcase"`

	// Research-informed reward protection. These counters only advance when the
	// cache actually rolls the spaceship category, so normal EXP/coin rewards do
	// not silently burn pity progress.
	ShipDropsSinceRedPlus int `json:"ship_drops_since_red_plus"`
	ShipDropsSinceGold    int `json:"ship_drops_since_gold"`

	// Persistent Survival profile statistics.
	BestSurvivalWave  int `json:"best_survival_wave"`
	BestSurvivalKills int `json:"best_survival_kills"`
	SentinelDefeats   int `json:"sentinel_defeats"`
	SerpentDefeats    int `json:"serpent_defeats"`
	ArrayDefeats      int `json:"array_defeats"`
	BestSurvivalCombo int `json:"best_survival_combo"`

	// Endurance achievement/career telemetry (v207).
	EnduranceRuns           int `json:"endurance_runs"`
	EnduranceBestTargets    int `json:"endurance_best_targets"`
	EnduranceWarpsCompleted int `json:"endurance_warps_completed"`
	AlienBossesSurvived     int `json:"alien_bosses_survived"`
	EndurancePowerups       int `json:"endurance_powerups_collected"`
	EnduranceShieldUses     int `json:"endurance_shield_uses"`
	EnduranceTimeUses       int `json:"endurance_time_uses"`
	SpaceCachesOpened       int `json:"space_caches_opened"`

	// Survival progression. Highest unlocked checkpoint persists locally and syncs to cloud.
	SurvivalCheckpoint int    `json:"survival_checkpoint"`
	SurvivalGuideMask  uint16 `json:"survival_guide_mask"`
}

type CosmeticEntitlements struct {
	UnlockedFireColors []int `json:"unlocked_fire_colors"`
	SelectedFireColor  int   `json:"selected_fire_color"`
	UnlockedFireSizes  []int `json:"unlocked_fire_sizes"`
	SelectedFireSize   int   `json:"selected_fire_size"`
}

func localCosmeticEntitlementsPath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if cfg, err := os.UserConfigDir(); err == nil {
			base = cfg
		}
	}
	if base == "" {
		base = "."
	}
	dir := filepath.Join(base, "KongGame", "CursorControlTrainer")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "garage_entitlements.json")
}

func saveCosmeticEntitlements() {
	e := CosmeticEntitlements{
		UnlockedFireColors: append([]int(nil), gameMeta.UnlockedFireColors...),
		SelectedFireColor:  gameMeta.SelectedFireColor,
		UnlockedFireSizes:  append([]int(nil), gameMeta.UnlockedFireSizes...),
		SelectedFireSize:   gameMeta.SelectedFireSize,
	}
	if data, err := json.MarshalIndent(e, "", "  "); err == nil {
		_ = atomicWriteFile(localCosmeticEntitlementsPath(), data, 0644)
	}
}

func loadCosmeticEntitlements() {
	var e CosmeticEntitlements
	if err := readJSONWithRecovery(localCosmeticEntitlementsPath(), &e); err != nil {
		return
	}
	gameMeta.UnlockedFireColors = mergeCosmeticIDCollections(gameMeta.UnlockedFireColors, e.UnlockedFireColors, len(fireColorDefs)-1)
	gameMeta.UnlockedFireSizes = mergeCosmeticIDCollections(gameMeta.UnlockedFireSizes, e.UnlockedFireSizes, len(fireSizeDefs)-1)
	if e.SelectedFireColor >= 0 && e.SelectedFireColor < len(fireColorDefs) && (e.SelectedFireColor == 0 || fireColorUnlocked(e.SelectedFireColor)) {
		gameMeta.SelectedFireColor = e.SelectedFireColor
	}
	if e.SelectedFireSize >= 0 && e.SelectedFireSize < len(fireSizeDefs) && (e.SelectedFireSize == 0 || fireSizeUnlocked(e.SelectedFireSize)) {
		gameMeta.SelectedFireSize = e.SelectedFireSize
	}
}

type AchievementDef struct {
	ID          string
	Title       string
	Description string
	Tier        int // 0 Easy, 1 Medium, 2 Hard
}

type AuthSession struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

type OverlayMode int

type HUDLayoutRect struct {
	Left   int32 `json:"left"`
	Top    int32 `json:"top"`
	Right  int32 `json:"right"`
	Bottom int32 `json:"bottom"`
}

type HUDLayoutConfig struct {
	ModeSwitch HUDLayoutRect `json:"mode_switch"`
	SpaceCache HUDLayoutRect `json:"space_cache"`
	Garage     HUDLayoutRect `json:"garage"`
	Profile    HUDLayoutRect `json:"profile"`
	Local      HUDLayoutRect `json:"local"`
	Global     HUDLayoutRect `json:"global"`
	Discord    HUDLayoutRect `json:"discord"`
	Bug        HUDLayoutRect `json:"bug"`
	SupportDev HUDLayoutRect `json:"support_dev"`
}

const (
	OverlayNone      OverlayMode = iota
	OverlayNameEntry             // legacy migration support
	OverlayLeaderboard
	OverlayGlobalLeaderboard
	OverlayMainMenu
	OverlayProfile
	OverlaySettings
	OverlayTutorial
	OverlayAchievements
	OverlayReleaseNotes
	OverlayRemoteProfile
	OverlayProfileSkins
	OverlayDifficultyLocked
	OverlaySurvivalSectionLocked
	OverlaySurvivalMonsterGuide
	OverlayGarage
	OverlaySpaceCache
	OverlayAFKSingularity
	OverlayDeveloperConsole
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	winmm    = syscall.NewLazyDLL("winmm.dll")
	msimg32  = syscall.NewLazyDLL("msimg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	gdiplus  = syscall.NewLazyDLL("gdiplus.dll")
	d2d1     = syscall.NewLazyDLL("d2d1.dll")

	registerClassExW   = user32.NewProc("RegisterClassExW")
	createWindowExW    = user32.NewProc("CreateWindowExW")
	defWindowProcW     = user32.NewProc("DefWindowProcW")
	showWindow         = user32.NewProc("ShowWindow")
	updateWindow       = user32.NewProc("UpdateWindow")
	getMessageW        = user32.NewProc("GetMessageW")
	peekMessageW       = user32.NewProc("PeekMessageW")
	translateMessage   = user32.NewProc("TranslateMessage")
	dispatchMessageW   = user32.NewProc("DispatchMessageW")
	postQuitMessage    = user32.NewProc("PostQuitMessage")
	postMessageW       = user32.NewProc("PostMessageW")
	loadCursorW        = user32.NewProc("LoadCursorW")
	loadIconW          = user32.NewProc("LoadIconW")
	setCursor          = user32.NewProc("SetCursor")
	setCursorPos       = user32.NewProc("SetCursorPos")
	clientToScreen     = user32.NewProc("ClientToScreen")
	getCursorPos       = user32.NewProc("GetCursorPos")
	screenToClient     = user32.NewProc("ScreenToClient")
	beginPaint         = user32.NewProc("BeginPaint")
	endPaint           = user32.NewProc("EndPaint")
	getClientRect      = user32.NewProc("GetClientRect")
	getWindowRect      = user32.NewProc("GetWindowRect")
	destroyWindow      = user32.NewProc("DestroyWindow")
	invalidateRect     = user32.NewProc("InvalidateRect")
	setTimer           = user32.NewProc("SetTimer")
	killTimer          = user32.NewProc("KillTimer")
	fillRect           = user32.NewProc("FillRect")
	messageBoxW        = user32.NewProc("MessageBoxW")
	setCapture         = user32.NewProc("SetCapture")
	releaseCapture     = user32.NewProc("ReleaseCapture")
	getKeyState        = user32.NewProc("GetKeyState")
	setProcessDPIAware = user32.NewProc("SetProcessDPIAware")
	getDpiForWindow    = user32.NewProc("GetDpiForWindow")
	getGuiResources    = user32.NewProc("GetGuiResources")
	adjustWindowRectEx = user32.NewProc("AdjustWindowRectEx")
	getSystemMetrics   = user32.NewProc("GetSystemMetrics")

	createPen              = gdi32.NewProc("CreatePen")
	extCreatePen           = gdi32.NewProc("ExtCreatePen")
	createSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	selectObject           = gdi32.NewProc("SelectObject")
	deleteObject           = gdi32.NewProc("DeleteObject")
	moveToEx               = gdi32.NewProc("MoveToEx")
	lineTo                 = gdi32.NewProc("LineTo")
	polygon                = gdi32.NewProc("Polygon")
	ellipse                = gdi32.NewProc("Ellipse")
	setBkMode              = gdi32.NewProc("SetBkMode")
	setTextColor           = gdi32.NewProc("SetTextColor")
	textOutW               = gdi32.NewProc("TextOutW")
	getTextExtentPoint32W  = gdi32.NewProc("GetTextExtentPoint32W")
	createFontW            = gdi32.NewProc("CreateFontW")
	createCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	createCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	createDIBSection       = gdi32.NewProc("CreateDIBSection")
	deleteDC               = gdi32.NewProc("DeleteDC")
	bitBlt                 = gdi32.NewProc("BitBlt")
	stretchDIBits          = gdi32.NewProc("StretchDIBits")
	setStretchBltMode      = gdi32.NewProc("SetStretchBltMode")
	setBrushOrgEx          = gdi32.NewProc("SetBrushOrgEx")
	saveDC                 = gdi32.NewProc("SaveDC")
	restoreDC              = gdi32.NewProc("RestoreDC")
	intersectClipRect      = gdi32.NewProc("IntersectClipRect")

	getModuleHandleW  = kernel32.NewProc("GetModuleHandleW")
	getCurrentProcess = kernel32.NewProc("GetCurrentProcess")
	mciSendStringW    = winmm.NewProc("mciSendStringW")
	timeBeginPeriod   = winmm.NewProc("timeBeginPeriod")
	timeEndPeriod     = winmm.NewProc("timeEndPeriod")
	alphaBlend        = msimg32.NewProc("AlphaBlend")
	transparentBlt    = msimg32.NewProc("TransparentBlt")
	shellExecuteW     = shell32.NewProc("ShellExecuteW")

	gdiplusStartup         = gdiplus.NewProc("GdiplusStartup")
	gdiplusShutdown        = gdiplus.NewProc("GdiplusShutdown")
	gdipCreateFromHDC      = gdiplus.NewProc("GdipCreateFromHDC")
	gdipDeleteGraphics     = gdiplus.NewProc("GdipDeleteGraphics")
	gdipSetSmoothingMode   = gdiplus.NewProc("GdipSetSmoothingMode")
	gdipSetPixelOffsetMode = gdiplus.NewProc("GdipSetPixelOffsetMode")
	gdipCreatePen1         = gdiplus.NewProc("GdipCreatePen1")
	gdipDeletePen          = gdiplus.NewProc("GdipDeletePen")
	gdipSetPenLineJoin     = gdiplus.NewProc("GdipSetPenLineJoin")
	gdipSetPenStartCap     = gdiplus.NewProc("GdipSetPenStartCap")
	gdipSetPenEndCap       = gdiplus.NewProc("GdipSetPenEndCap")
	gdipDrawLines          = gdiplus.NewProc("GdipDrawLines")
	gdipCreateSolidFill    = gdiplus.NewProc("GdipCreateSolidFill")
	gdipDeleteBrush        = gdiplus.NewProc("GdipDeleteBrush")
	gdipFillEllipseI       = gdiplus.NewProc("GdipFillEllipseI")
	gdipDrawEllipseI       = gdiplus.NewProc("GdipDrawEllipseI")
	gdipDrawLineI          = gdiplus.NewProc("GdipDrawLineI")

	d2d1CreateFactory = d2d1.NewProc("D2D1CreateFactory")
)

var (
	mainHwnd            uintptr
	introSplashHwnd     uintptr
	introSplashCallback uintptr
	callbackPtr         uintptr
	arrowCursor         uintptr
	failedFont          uintptr
	failedReasonFont    uintptr
	hudTitleFont        uintptr
	hudStatFont         uintptr
	hudSmallFont        uintptr
	hudTinyFont         uintptr
	profileNameFont     uintptr
	failedNormalBGRA    []byte
	failedEnduranceBGRA []byte
	introLogoFont       uintptr
	introTextFont       uintptr
	gdipToken           uintptr

	path      []FPoint
	targets   []Target
	state     = StateWaiting
	startTime time.Time
	lastTime  = 10.0

	score      int // total score within CURRENT streak
	streak     int
	bestScore  int
	bestStreak int

	difficulty = 1
	status     = "Click START to begin"

	lastMouse     FPoint
	hasLastMouse  bool
	progressIndex int

	previousEnd    FPoint
	hasPreviousEnd bool
	startSide      = 1

	cursorPos     FPoint
	cursorInArena bool

	// Accuracy / score analytics.
	trackAccuracySum     float64
	trackAccuracySamples int
	targetPrecisionSum   float64
	targetPrecisionHits  int

	// Result screen / feedback.
	lastResult   ResultData
	lastHitAt    time.Time
	lastHitPoint FPoint
	hitFXEnabled = true

	// Training options.
	menuOpen         bool
	modeSelectorOpen bool
	overlayMode      OverlayMode
	pathMode         int // 0 random, 1 smooth, 2 precision, 3 flick, 4 tracking, 5 chaos
	currentCourse    int
	adaptiveMode     bool
	adaptiveTier     = 1
	adaptiveMeter    int
	movingMode       int // 0 auto, 1 on, 2 off
	gameMode         int // 0 Standard/Precision, 1 Endurance, 2 Survival

	// Developer HUD layout editor (F2, owner-only).
	hudLayoutEditorActive bool
	hudLayoutSelected     = -1 // 0 Mode, 1 Cache, 2 Garage, 3 Profile, 4 Local, 5 Global, 6 Discord, 7 Bug
	hudLayoutDragging     bool
	hudLayoutDragOffsetX  int32
	hudLayoutDragOffsetY  int32
	hudLayoutConfig       HUDLayoutConfig
	hudLayoutLoaded       bool
	hudLayoutConfigFile   string
	// Settings controls.
	settingsVolumeDrag          int // 0 none, 1 music, 2 effects
	settingsScroll              int32
	settingsScrollbarDragging   bool
	settingsScrollbarDragOffset int32

	// Endurance survival state. Horizontal chunks are generated ahead of time.
	enduranceDistance              float64
	enduranceDistanceBonus         float64
	enduranceBonusScore            int
	enduranceTargetsHit            int
	endurancePowerups              []EndurancePowerup
	enduranceNextPowerupAt         float64
	enduranceShieldUntil           time.Time
	enduranceSlowUntil             time.Time
	enduranceStoredShields         int
	enduranceStoredTime            int
	enduranceNextTargetAt          float64
	enduranceLastTick              time.Time
	enduranceWorldStep             int
	enduranceLastX                 float64
	enduranceTargetSerial          int
	enduranceChunkPoints           int
	enduranceChunkDY               float64
	enduranceChunksBuilt           int
	enduranceGeneratedDistance     float64
	enduranceCameraX               float64
	enduranceBlocks                []EnduranceBlock
	enduranceBlockSpawnTime        time.Time
	enduranceAlienMinions          []EnduranceAlienMinion
	enduranceAlienMinionSpawnTime  time.Time
	enduranceAlienPhaseTriggered   bool
	enduranceAlienBossState        int
	enduranceAlienBossStateStarted time.Time
	enduranceAlienBossX            float64
	enduranceAlienBossY            float64
	enduranceAlienBossTargetY      float64
	enduranceAlienBossMoveTargetY  float64
	enduranceAlienBossLockedX      float64
	enduranceAlienBossLockedY      float64
	enduranceAlienBossEntryStartX  float64
	enduranceAlienBossEntryStartY  float64
	enduranceAlienBossEntryTargetX float64
	enduranceAlienBossEntryTargetY float64
	enduranceNextWarpAt            float64
	enduranceWarpCueActive         bool
	enduranceWarpActive            bool
	enduranceWarpCueStarted        time.Time
	enduranceWarpStartDistance     float64
	enduranceWarpCheckpoint        float64
	enduranceWarpRecoveryUntil     float64
	enduranceWarpTargetsSpawned    bool
	enduranceTargetExplosions      []TargetExplosion
	enduranceWarpCheckpoints       []float64
	enduranceAmbientClock          float64
	enduranceAmbientLastFrame      time.Time
	enduranceParticleClock         float64
	enduranceParticleLastFrame     time.Time
	enduranceWarpAmbientReturnAt   time.Time
	enduranceWarpAmbientReturnFrom float64

	// Space Cache / garage UI state.
	spaceCacheOpenStarted  time.Time
	spaceCacheOpened       bool
	spaceCacheRewardText   string
	spaceCacheRewardCoins  int // >0 renders the Space Coin icon beside the reward amount
	spaceCacheRewardShip   int // >0 renders the unlocked/duplicate ship artwork in the reward popup
	spaceCacheWarningUntil time.Time
	spaceCacheWarningText  string
	freeCacheClaimInFlight bool
	freeCacheAvailable     bool
	freeCacheNextClaimAt   time.Time
	freeCacheStatusKnown   bool
	garageTab              int // 0 ships, 1 thruster fire, 2 fire size
	garageNoticeText       string
	garageNoticeUntil      time.Time

	// Global live announcement rail + server-clock free cache state.
	liveMu                          sync.Mutex
	liveAnnouncementQueue           []string
	liveAnnouncementText            string
	liveAnnouncementStarted         time.Time
	livePreviousAnnouncementText    string
	livePreviousAnnouncementStarted time.Time
	liveNextAmbientAt               time.Time
	liveLastFeedID                  int64
	liveFeedBaselineReady           bool
	liveLeaderboardBaseline         bool
	liveLastLeaderboardPoll         time.Time
	liveLastFeedPoll                time.Time
	liveServerOffset                time.Duration

	// Local/global leaderboard UI.
	leaderboard                []LeaderboardEntry
	leaderboardFile            string
	nameInput                  string // legacy migration support
	localLeaderboardFilter     int    // 0 overall,1 easy,2 normal,3 hard,4 insane,5 endurance
	globalLeaderboardFilter    int
	globalLeaderboardOverall   []GlobalLeaderboardEntry
	globalLeaderboardEasy      []GlobalLeaderboardEntry
	globalLeaderboardNormal    []GlobalLeaderboardEntry
	globalLeaderboardHard      []GlobalLeaderboardEntry
	globalLeaderboardInsane    []GlobalLeaderboardEntry
	globalLeaderboardEndurance []GlobalLeaderboardEntry
	globalLeaderboardStatus    string
	globalLeaderboardLoading   bool
	selectedGlobalPlayer       int
	selectedGlobalOverride     GlobalLeaderboardEntry
	selectedGlobalOverrideOn   bool
	lockedDifficultyPopup      int
	globalMu                   sync.Mutex
	globalSubmitMu             sync.Mutex
	pendingEnduranceSubmitFile string
	syncedEndurancePBFile      string
	globalEnduranceSyncedPB    GlobalLeaderboardEntry
	globalEnduranceSyncedValid bool
	globalMyOverall            GlobalLeaderboardEntry
	globalMyOverallValid       bool

	// Online UI scaffold. Authentication/backend hookup is completed after
	// the Supabase Edge Functions are deployed.
	discordConnected            bool
	discordDisplayName          string
	discordAuthStatus           string
	discordUserID               string
	discordAvatarURL            string
	discordAvatarBGRA           []byte
	discordAvatarW              int32
	discordAvatarH              int32
	discordCreatedAt            string
	remoteProfileMu             sync.Mutex
	remoteProfile               RemoteProfileData
	remoteProfileLoaded         bool
	remoteProfileLoading        bool
	remoteProfileStatus         string
	remoteAvatarBGRA            []byte
	remoteAvatarW               int32
	remoteAvatarH               int32
	developerConsoleInput       string
	developerConsoleStatus      string
	developerGodMode            bool
	developerConfirmAction      string
	developerConfirmUntil       time.Time
	developerPauseStarted       time.Time
	developerBoundaryGraceUntil time.Time
	developerSurvivalGraceUntil time.Time
	authSession                 AuthSession
	authSessionFile             string
	authMu                      sync.Mutex
	authServer                  *http.Server

	// Persistent progression for this Windows user/profile.
	playerProgress PlayerProgress
	progressFile   string

	// v40 profile/settings/achievement persistence.
	gameMeta       GameMeta
	metaFile       string
	gameRoot       string
	assetRoot      string
	textureRoot    string
	cacheRoot      string
	dataRoot       string
	logRoot        string
	sessionStarted time.Time
	tutorialPage   int
	tutorialMode   int // -1 mode chooser, 0 Precision, 1 Endurance, 2 Survival

	// UI motion / notifications.
	uiTransitionStart               time.Time
	lastAchievement                 string
	lastAchievementRewardEXP        int
	achievementAt                   time.Time
	dailyRewardAt                   time.Time
	weeklyRewardAt                  time.Time
	lastDailyRewardEXP              int
	achievementScroll               int32
	achievementFilter               int // 0=all, 1=unlocked, 2=locked
	achievementDragging             bool
	achievementDragOffset           int32
	achievementShowcaseTarget       int // 0..2: next public-profile showcase slot set from Achievements
	achievementShowcaseConfirmID    string
	achievementShowcaseConfirmSlot  int
	achievementShowcaseConfirmUntil time.Time

	// Brief level-up celebration.
	levelUpAt   time.Time
	levelUpFrom string
	levelUpTo   string

	// Cinematic startup sequence.
	introStart               time.Time
	introRechamberPlayed     bool
	introVideoHwnd           uintptr
	introVideoPlaying        bool
	particleEpoch            time.Time
	enduranceFailureVisualAt time.Time
	particleDCs              [3]uintptr
	particleBmps             [3]uintptr
	particleOlds             [3]uintptr
	hudIconDCs               [5]uintptr
	hudIconBmps              [5]uintptr
	hudIconOlds              [5]uintptr
	hudTopTextureDC          uintptr
	hudTopTextureBmp         uintptr
	hudTopTextureOld         uintptr
	hudBottomTextureDC       uintptr
	hudBottomTextureBmp      uintptr
	hudBottomTextureOld      uintptr
	arenaBgDC                uintptr
	arenaBgBmp               uintptr
	arenaBgOld               uintptr

	// Persistent full-window backbuffer. Reused every frame instead of allocating
	// a new DC + bitmap on every WM_PAINT.
	backBufferDC  uintptr
	backBufferBmp uintptr
	backBufferOld uintptr
	backBufferW   int32
	backBufferH   int32

	// Endurance rail cache: expensive multi-stroke GDI+ rail is rasterized once
	// per chunk/style change and then alpha-blitted while it scrolls.
	enduranceRailDC      uintptr
	enduranceRailBmp     uintptr
	enduranceRailOld     uintptr
	enduranceRailBits    uintptr
	enduranceRailW       int32
	enduranceRailH       int32
	enduranceRailOriginX float64
	enduranceRailOriginY float64
	enduranceRailDirty   bool
	enduranceRailBase    int

	enduranceBgDC  uintptr
	enduranceBgBmp uintptr
	enduranceBgOld uintptr

	// v105 Direct2D Endurance renderer. The hardware render target is used only
	// for the active playfield; the existing GDI UI remains as a low-frequency shell.
	d2dFactory                uintptr
	d2dRenderTarget           uintptr
	d2dChildHwnd              uintptr
	d2dChildCallback          uintptr
	d2dChildVisible           bool
	d2dReady                  bool
	d2dRetryAfter             time.Time
	d2dBackgroundBitmap       uintptr
	d2dHazardBlueBitmap       uintptr
	d2dHazardOrangeBitmap     uintptr
	d2dRocketBitmap           uintptr
	d2dWarpPortalTopBitmap    uintptr
	d2dAlienMinionBitmap      uintptr
	d2dAlienBossBitmap        uintptr
	d2dUFOWarningBitmap       uintptr
	d2dPowerupKeyQBitmap      uintptr
	d2dPowerupKeyWBitmap      uintptr
	d2dPowerupBitmaps         [3]uintptr
	d2dShipBitmaps            [13]uintptr
	d2dRailGlowBrush          uintptr
	d2dRailDarkBrush          uintptr
	d2dRailMidBrush           uintptr
	d2dRailSilverBrush        uintptr
	d2dRailSafetyBrush        uintptr
	d2dRailCoreBrush          uintptr
	d2dRailStrokeStyle        uintptr
	d2dParticleBrushes        [3]uintptr
	d2dTargetGlowBrush        uintptr
	d2dTargetBrush            uintptr
	d2dTargetDarkBrush        uintptr
	d2dExplosionCoreBrush     uintptr
	d2dExplosionHotBrush      uintptr
	d2dExplosionFireBrush     uintptr
	d2dExplosionEmberBrush    uintptr
	d2dThrusterColorBrushes   [7][4]uintptr // ember, flame, core, flare for purchasable fire colors
	d2dCrosshairBrush         uintptr
	d2dPowerupGlowBrush       uintptr
	d2dPowerupBlueBrush       uintptr
	d2dPowerupLightBrush      uintptr
	d2dPowerupYellowBrush     uintptr
	d2dPowerupRedBrush        uintptr
	d2dShieldFlashBrush       uintptr
	d2dShieldAuraBrush        uintptr
	d2dShieldEdgeBrush        uintptr
	d2dShieldCoreBrush        uintptr
	d2dShipHitboxBrush        uintptr
	d2dShipHitboxGlowBrush    uintptr
	d2dWarpWarmBrush          uintptr
	d2dWarpMagentaBrush       uintptr
	d2dWarpBlueBrush          uintptr
	d2dAlienWarningBrush      uintptr
	d2dAlienLaserGlowBrush    uintptr
	d2dAlienLaserCoreBrush    uintptr
	d2dAlienChargeFlashBrush  uintptr
	d2dAlienCautionFlashBrush uintptr
	d2dPowerupFullPulseBrush  uintptr
	enduranceD2DGeometries    []EnduranceD2DGeometry
	d2dMergedRailGeometry     uintptr
	d2dMergedRailDirty        bool

	enduranceLoopAccumulator float64
	enduranceLastLoopTime    time.Time
	enduranceLastRenderTime  time.Time
	enduranceLastHUDPaint    time.Time

	timeBonus float64
)

var diffs = []DiffSettings{
	{"EASY", 5, 34, 90, 0},
	{"NORMAL", 7, 24, 135, 0},
	{"HARD", 9, 18, 170, 0},
	{"INSANE", 12, 14, 205, 0},
}

// Runtime resources are intentionally external in v111.
// The executable loads them from the master project folder beside the EXE.
var (
	uiBaseBGRA                []byte
	cursorControlLogoBGRA     []byte
	cursorControlLogoHUDBGRA  []byte
	hudIconTimeBGRA           []byte
	hudIconScoreBGRA          []byte
	hudIconStreakBGRA         []byte
	hudIconBestBGRA           []byte
	hudIconDifficultyBGRA     []byte
	discordLoginButtonBGRA    []byte
	discordLoggedInButtonBGRA []byte
	bugReportButtonBGRA       []byte
	supportDevButtonBGRA      []byte
	profileButtonBGRA         []byte
	localButtonBGRA           []byte
	globalButtonBGRA          []byte
	precisionModeButtonBGRA   []byte
	enduranceModeButtonBGRA   []byte
	selectModeButtonBGRA      []byte
	modePrecisionCardBGRA     []byte
	modeEnduranceCardBGRA     []byte
	modeSurvivalCardBGRA      []byte
	modeStarbaseCardBGRA      []byte
	starbaseBackgroundBGRA    []byte
	starbaseSingularityBGRA   []byte
	starbaseMoonRockBGRA      []byte
	starbaseLogoWordmarkBGRA  []byte
	hudNetworkTopBGRA         []byte
	hudNetworkBottomBGRA      []byte
)

var (
	spaceCoinBGRA        []byte
	powerupShieldBGRA    []byte
	powerupTimeBGRA      []byte
	hazardBlueBGRA       []byte
	hazardOrangeBGRA     []byte
	powerupShieldSprite  CachedBGRASprite
	powerupTimeSprite    CachedBGRASprite
	hazardBlueSprite     CachedBGRASprite
	hazardOrangeSprite   CachedBGRASprite
	spaceCacheClosedBGRA []byte
	spaceCacheOpenBGRA   []byte
	starCacheBGRA        []byte
	expeditionHangarBGRA []byte
	spaceCoinBarBGRA     []byte
	spaceCacheButtonBGRA []byte
	garageButtonBGRA     []byte
	defaultShipBGRA      []byte
	spaceShipBGRA        [13][]byte
	// v302: preserve original ship artwork resolution for Garage/Profile rendering.
	shipTextureW = [13]int32{282, 269, 237, 247, 236, 227, 269, 239, 229, 255, 251, 32, 249}
	shipTextureH = [13]int32{229, 242, 210, 197, 213, 211, 199, 209, 222, 253, 221, 22, 224}
)

var (
	audioDir                   string
	audioReady                 bool
	bossPaused                 bool
	bossStarted                bool
	enduranceMusicReady        bool
	starbaseMusicReady         bool
	hitAudioReady              bool
	levelAudioReady            bool
	rechamberAudioReady        bool
	warpCueAudioReady          bool
	warpRocketAudioReady       bool
	enduranceExplodeAudioReady bool
	enduranceFailAudioReady    bool
	spaceCacheAudioReady       bool
	alienChargeAudioReady      bool
	alienImpactAudioReady      bool
	shieldProtectAudioReady    bool
	powerupPickupAudioReady    bool
	buttonClickAudioReady      bool
	buyAudioReady              bool
	survivalHitAudioReady      bool
	bossClickAudioReady        bool
	boss1RoarAudioReady        bool
	shieldProtectSoundPlaying  bool
	uiPixels                   []byte
	rankBadgeCache             = map[string][]byte{}
)

var (
	hitFeedbackPen uintptr
)

func blendColor(a, b uintptr, t float64) uintptr {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ar := float64(byte(a & 0xff))
	ag := float64(byte((a >> 8) & 0xff))
	ab := float64(byte((a >> 16) & 0xff))
	br := float64(byte(b & 0xff))
	bg := float64(byte((b >> 8) & 0xff))
	bb := float64(byte((b >> 16) & 0xff))
	return rgb(byte(ar+(br-ar)*t), byte(ag+(bg-ag)*t), byte(ab+(bb-ab)*t))
}

func rgb(r, g, b byte) uintptr {
	return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16)
}
func loword(v uintptr) int32 { return int32(int16(v & 0xffff)) }
func hiword(v uintptr) int32 { return int32(int16((v >> 16) & 0xffff)) }
func utf16ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}
func randf(a, b float64) float64 { return a + rand.Float64()*(b-a) }

func shipTextureDataAndSize(id int) ([]byte, int32, int32) {
	if id == 0 {
		return defaultShipBGRA, shipTextureW[0], shipTextureH[0]
	}
	if id > 0 && id < len(spaceShipBGRA) {
		return spaceShipBGRA[id], shipTextureW[id], shipTextureH[id]
	}
	return nil, 0, 0
}

func drawShipTextureFit(hdc uintptr, id int, dst RECT) {
	data, sw, sh := shipTextureDataAndSize(id)
	if sw <= 0 || sh <= 0 || len(data) < int(sw*sh*4) {
		return
	}
	drawRawBGRAFit(hdc, data, sw, sh, dst)
}

type SpaceShipDef struct {
	Name   string
	Rarity string
	Chance float64 // percentage inside the 10% spaceship cache category
}

var spaceShipDefs = [13]SpaceShipDef{
	{Name: "DEFAULT", Rarity: "DEFAULT", Chance: 0},
	{Name: "SOLAR WARDEN", Rarity: "ORBITAL", Chance: 22.7},
	{Name: "NEON RIFT", Rarity: "NOVA", Chance: 2.3},
	{Name: "COBALT COMET", Rarity: "ORBITAL", Chance: 22.7},
	{Name: "ION VIPER", Rarity: "ORBITAL", Chance: 22.6},
	{Name: "CRIMSON NOVA", Rarity: "NEBULA", Chance: 6},
	{Name: "GOLD RUSH", Rarity: "NEBULA", Chance: 6},
	{Name: "EMBER HAWK", Rarity: "NEBULA", Chance: 6},
	{Name: "ORCHID PRISM", Rarity: "NOVA", Chance: 2.4},
	{Name: "AURORA BLOOM", Rarity: "CELESTIAL", Chance: 1},
	{Name: "TOXIC PHANTOM", Rarity: "NEBULA", Chance: 6},
	{}, // ship 11 removed: HYPERNOVA duplicated ORCHID PRISM
	{Name: "CELESTIAL PEARL", Rarity: "NOVA", Chance: 2.3},
}

var garageShipOrder = [12]int{0, 1, 3, 4, 5, 6, 7, 10, 2, 8, 12, 9}

type FireColorDef struct {
	Name  string
	Cost  int
	Color uintptr
}

var fireColorDefs = [8]FireColorDef{
	{Name: "RED", Cost: 0, Color: rgb(255, 55, 30)},
	{Name: "GREEN", Cost: 200, Color: rgb(55, 245, 85)},
	{Name: "BLUE", Cost: 200, Color: rgb(55, 145, 255)},
	{Name: "PINK", Cost: 200, Color: rgb(255, 80, 190)},
	{Name: "PURPLE", Cost: 200, Color: rgb(170, 70, 255)},
	{Name: "GOLD", Cost: 750, Color: rgb(255, 205, 45)},
	{Name: "SILVER", Cost: 500, Color: rgb(230, 240, 255)},
	{Name: "RAINBOW", Cost: 1500, Color: rgb(255, 255, 255)},
}

// Display order is deliberately separate from the persisted colour IDs so old saves keep their unlocks.
// Gold follows Silver in the garage without remapping selected/unlocked IDs.
var garageFireColorOrder = [8]int{0, 1, 2, 3, 4, 6, 5, 7}

func fireColorUnlocked(id int) bool {
	if id == 0 {
		return true
	}
	for _, v := range gameMeta.UnlockedFireColors {
		if v == id {
			return true
		}
	}
	return false
}

func unlockFireColor(id int) {
	if id <= 0 || id >= len(fireColorDefs) || fireColorUnlocked(id) {
		return
	}
	gameMeta.UnlockedFireColors = append(gameMeta.UnlockedFireColors, id)
}

type FireSizeDef struct {
	Name       string
	Cost       int
	Multiplier float32
}

var fireSizeDefs = [3]FireSizeDef{
	{Name: "SMALL", Cost: 0, Multiplier: 1.00},
	{Name: "MEDIUM", Cost: 250, Multiplier: 1.75},
	{Name: "LARGE", Cost: 1000, Multiplier: 2.50},
}

func fireSizeUnlocked(id int) bool {
	if id == 0 {
		return true
	}
	for _, v := range gameMeta.UnlockedFireSizes {
		if v == id {
			return true
		}
	}
	return false
}

func unlockFireSize(id int) {
	if id <= 0 || id >= len(fireSizeDefs) || fireSizeUnlocked(id) {
		return
	}
	gameMeta.UnlockedFireSizes = append(gameMeta.UnlockedFireSizes, id)
}

func selectedFireSizeMultiplier() float32 {
	id := gameMeta.SelectedFireSize
	if id < 0 || id >= len(fireSizeDefs) || !fireSizeUnlocked(id) {
		id = 0
	}
	return fireSizeDefs[id].Multiplier
}

func shipUnlocked(id int) bool {
	if id == 0 {
		return true
	}
	for _, v := range gameMeta.UnlockedShips {
		if v == id {
			return true
		}
	}
	return false
}

func unlockShip(id int) {
	if id <= 0 || id > 12 || shipUnlocked(id) {
		return
	}
	gameMeta.UnlockedShips = append(gameMeta.UnlockedShips, id)
	requestPlayerProfileSync()
}

func rollSpaceShip() int {
	return rollSpaceShipProtected()
}

func resolveSpaceCacheReward() {
	if spaceCacheOpened {
		return
	}
	spaceCacheOpened = true
	gameMeta.SpaceCachesOpened++
	playSpaceCacheSound()

	r := randf(0, 1)
	// Space Caches primarily reward account progression. Space Coins are an
	// intentionally rare consolation/troll drop; spaceship odds remain unchanged.
	// Loot split: 85% EXP / 5% Space Coins / 10% spaceship.
	if r < 0.85 {
		exp := (1 + rand.Intn(10)) * 100
		oldRank := rankIndexForEXP(playerProgress.EXP)
		playerProgress.EXP += exp
		savePlayerProgress()
		if rankIndexForEXP(playerProgress.EXP) > oldRank {
			levelUpAt = time.Now()
			levelUpTo = rankForEXP(playerProgress.EXP)
			playLevelUpSound()
		}
		spaceCacheRewardCoins = 0
		spaceCacheRewardShip = 0
		spaceCacheRewardText = fmt.Sprintf("YOU RECEIVED +%d EXP", exp)
	} else if r < 0.90 {
		coinRoll := randf(0, 1)
		coins := 15
		if coinRoll >= 0.50 && coinRoll < 0.85 {
			coins = 50
		} else if coinRoll >= 0.85 {
			coins = 75
		}
		gameMeta.SpaceCoins += coins
		spaceCacheRewardCoins = coins
		spaceCacheRewardShip = 0
		spaceCacheRewardText = "YOU RECEIVED"
	} else {
		ship := rollSpaceShip()
		updateShipPityAfterRoll(ship)
		spaceCacheRewardShip = ship
		duplicate := shipUnlocked(ship)
		if duplicate {
			coins := duplicateCompensation(ship)
			gameMeta.SpaceCoins += coins
			spaceCacheRewardCoins = coins
			spaceCacheRewardText = fmt.Sprintf("DUPLICATE %s", spaceShipDefs[ship].Name)
		} else {
			unlockShip(ship)
			spaceCacheRewardCoins = 0
			spaceCacheRewardText = "NEW SPACESHIP UNLOCKED"
			go publishSpaceCacheShipUnlock(ship)
			// Reward audio hierarchy: rare cosmetics add a progression sting while
			// ordinary ship drops keep the normal cache sound. No gameplay effect.
			tier := shipRarityTier(ship)
			if tier >= shipTierCovert {
				playLevelUpSound()
			}
		}
		analyticsEvent("space_cache_ship", map[string]any{"ship": ship, "tier": shipRarityTier(ship), "duplicate": duplicate, "pity_red": gameMeta.ShipDropsSinceRedPlus, "pity_gold": gameMeta.ShipDropsSinceGold})
	}
	evaluateEnduranceAchievements()
	saveGameMeta()
}

func showSpaceCacheWarning(h uintptr, text string) {
	spaceCacheWarningText = text
	spaceCacheWarningUntil = time.Now().Add(2 * time.Second)
	setTimer.Call(h, TIMER_UI, 16, 0)
	invalidateRect.Call(h, 0, 0)
}

func startSpaceCacheOpening(h uintptr) {
	spaceCacheWarningUntil = time.Time{}
	spaceCacheWarningText = ""
	spaceCacheOpenStarted = time.Now()
	spaceCacheOpened = false
	spaceCacheRewardCoins = 0
	spaceCacheRewardShip = 0
	spaceCacheRewardText = "OPENING SPACE CACHE..."
	setOverlay(OverlaySpaceCache)
	analyticsEvent("space_cache_open_started", map[string]any{"coins_after_cost": gameMeta.SpaceCoins})
	setTimer.Call(h, TIMER_UI, 16, 0)
}

func beginSpaceCacheOpen(h uintptr) {
	if gameMeta.SpaceCoins < 100 {
		showSpaceCacheWarning(h, "You need 100 Space Coins to open a Cache!")
		return
	}
	gameMeta.SpaceCoins -= 100
	saveGameMeta()
	startSpaceCacheOpening(h)
}

func serverNow() time.Time {
	liveMu.Lock()
	offset := liveServerOffset
	liveMu.Unlock()
	return time.Now().Add(offset)
}

func freeCacheCountdownText() string {
	liveMu.Lock()
	known := freeCacheStatusKnown
	available := freeCacheAvailable
	next := freeCacheNextClaimAt
	liveMu.Unlock()
	if !discordConnected {
		return "LOG IN WITH DISCORD TO CLAIM"
	}
	if !known {
		return "CHECKING SERVER TIMER..."
	}
	if available {
		return "AVAILABLE NOW"
	}
	remaining := next.Sub(serverNow())
	if remaining <= 0 {
		return "AVAILABLE NOW"
	}
	total := int(remaining.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	sec := total % 60
	return fmt.Sprintf("NEXT FREE CACHE (JKT)  %02d:%02d:%02d", h, m, sec)
}

func freeCacheClaimRect(w, hgt int32) RECT {
	_, cacheR, _ := enduranceSpaceUIRects(w, hgt)
	return RECT{cacheR.Left - sx(90, w), cacheR.Bottom + sy(2, hgt), cacheR.Right + sx(90, w), cacheR.Bottom + sy(45, hgt)}
}

func beginFreeSpaceCacheClaim(h uintptr) {
	if !discordConnected {
		showSpaceCacheWarning(h, "Log in with Discord to claim your free Space Cache.")
		return
	}
	liveMu.Lock()
	if freeCacheClaimInFlight {
		liveMu.Unlock()
		return
	}
	known := freeCacheStatusKnown
	available := freeCacheAvailable
	if known && !available {
		liveMu.Unlock()
		showSpaceCacheWarning(h, "You need 100 Space Coins to open a Cache!")
		return
	}
	freeCacheClaimInFlight = true
	liveMu.Unlock()
	go claimFreeSpaceCache(h)
}

func handleEndurancePowerupClick(p FPoint) bool {
	if !enduranceActive() || state != StatePlaying || len(endurancePowerups) == 0 {
		return false
	}
	kept := endurancePowerups[:0]
	hit := false
	for _, pu := range endurancePowerups {
		screen := pu.Point
		screen.X -= enduranceCameraX
		if !hit && dist(p, screen) <= 21.0 {
			if pu.Kind == endurancePowerupDistance {
				// +100m remains the only instant-use pickup.
				applyEndurancePowerup(pu.Kind)
				gameMeta.EndurancePowerups++
				evaluateEnduranceAchievements()
				saveGameMeta()
				playPowerupPickupSound()
				if mainHwnd != 0 {
					w, hgt := getClient(mainHwnd)
					ar := arenaRect(w, hgt)
					addPolishVFX(polishVFXPickup, float32(screen.X-float64(ar.Left)), float32(screen.Y-float64(ar.Top)), 0.36)
				}
				hit = true
				continue
			}
			if storeEndurancePowerup(pu.Kind) {
				// Shield/Time are banked instead of consumed immediately.
				gameMeta.EndurancePowerups++
				evaluateEnduranceAchievements()
				saveGameMeta()
				playPowerupPickupSound()
				if mainHwnd != 0 {
					w, hgt := getClient(mainHwnd)
					ar := arenaRect(w, hgt)
					addPolishVFX(polishVFXPickup, float32(screen.X-float64(ar.Left)), float32(screen.Y-float64(ar.Top)), 0.36)
				}
				hit = true
				continue
			}
			// Inventory full: leave the pickup on the rail so the player can
			// activate a stored charge and collect it afterward.
			hit = true
		}
		kept = append(kept, pu)
	}
	endurancePowerups = kept
	return hit
}
func dist(a, b FPoint) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

func sx(v float64, w int32) int32 { return int32(v * float64(w) / 1536.0) }
func sy(v float64, h int32) int32 { return int32(v * float64(h) / 1024.0) }

func getClient(h uintptr) (int32, int32) {
	var r RECT
	getClientRect.Call(h, uintptr(unsafe.Pointer(&r)))
	return r.Right - r.Left, r.Bottom - r.Top
}

func textOut(hdc uintptr, x, y int32, s string) {
	u := syscall.StringToUTF16(s)
	if len(u) > 1 {
		textOutW.Call(hdc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1))
	}
}

func centeredTextOut(hdc uintptr, left, right, y int32, s string) {
	u := syscall.StringToUTF16(s)
	if len(u) <= 1 {
		return
	}
	var sz SIZE
	getTextExtentPoint32W.Call(hdc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&sz)))
	x := left + ((right-left)-sz.Cx)/2
	textOut(hdc, x, y, s)
}

func mci(command string) bool {
	u := syscall.StringToUTF16(command)
	if len(u) == 0 {
		return false
	}
	r, _, _ := mciSendStringW.Call(uintptr(unsafe.Pointer(&u[0])), 0, 0, 0)
	return r == 0
}

func mciQuery(command string) string {
	u := syscall.StringToUTF16(command)
	if len(u) == 0 {
		return ""
	}
	buf := make([]uint16, 128)
	r, _, _ := mciSendStringW.Call(
		uintptr(unsafe.Pointer(&u[0])),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0,
	)
	if r != 0 {
		return ""
	}
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return syscall.UTF16ToString(buf[:n])
}

func failureSoundName() string {
	names := []string{"Default", "Fortnite", "Roblox", "Minecraft", "Among Us"}
	i := gameMeta.FailureSound
	if i < 0 || i >= len(names) {
		i = 0
	}
	return names[i]
}

func failureSoundAlias() string {
	aliases := []string{"buzzer", "buzz_fortnite", "buzz_roblox", "buzz_minecraft", "buzz_amongus"}
	i := gameMeta.FailureSound
	if i < 0 || i >= len(aliases) {
		i = 0
	}
	return aliases[i]
}

func stopAllFailureSounds() {
	stopAlienChargeSound()
	stopShieldProtectSound()
}

// playFailureSoundPreview gives immediate audio feedback when the player cycles
// the Precision failure sound in Settings. It deliberately uses the same loaded
// MCI alias and Effects volume as real gameplay, so the preview is representative.
func playFailureSoundPreview() {
	if !audioReady {
		return
	}
	stopAllFailureSounds()
	alias := failureSoundAlias()
	playSFX(alias)
}

func initGameFolders() {
	exe, err := os.Executable()
	if err != nil {
		gameRoot = "."
	} else {
		gameRoot = filepath.Dir(exe)
	}
	assetRoot = filepath.Join(gameRoot, "assets")
	textureRoot = filepath.Join(assetRoot, "textures")
	cacheRoot = filepath.Join(gameRoot, "cache")
	dataRoot = filepath.Join(gameRoot, "data")
	logRoot = filepath.Join(gameRoot, "logs")
	pendingEnduranceSubmitFile = filepath.Join(dataRoot, "pending_endurance_pb.json")
	syncedEndurancePBFile = filepath.Join(dataRoot, "synced_endurance_pb.json")

	for _, dir := range []string{
		assetRoot,
		filepath.Join(assetRoot, "audio"),
		filepath.Join(assetRoot, "backgrounds"),
		filepath.Join(assetRoot, "ui"),
		filepath.Join(assetRoot, "ranks"),
		textureRoot,
		cacheRoot,
		filepath.Join(cacheRoot, "endurance"),
		dataRoot,
		logRoot,
	} {
		_ = os.MkdirAll(dir, 0755)
	}

}

func defaultHUDLayoutConfig() HUDLayoutConfig {
	return HUDLayoutConfig{
		ModeSwitch: HUDLayoutRect{274, 28, 574, 98},
		SpaceCache: HUDLayoutRect{680, 852, 856, 912},
		Garage:     HUDLayoutRect{866, 852, 1053, 912},
		Profile:    HUDLayoutRect{1014, 799, 1174, 847},
		Local:      HUDLayoutRect{1184, 799, 1344, 851},
		Global:     HUDLayoutRect{1354, 799, 1514, 851},
		// v426: keep the utility stack in a dedicated lower-right safe column.
		// The legacy fallback overlapped PROFILE/LOCAL/GLOBAL at 1536x1024 and
		// became especially obvious on laptops with different DPI/aspect scaling.
		Bug:        HUDLayoutRect{1324, 862, 1514, 908},
		SupportDev: HUDLayoutRect{1324, 914, 1514, 960},
		Discord:    HUDLayoutRect{1324, 966, 1514, 1012},
	}
}

func normalizeHUDLayoutRect(r HUDLayoutRect, fallback HUDLayoutRect) HUDLayoutRect {
	if r.Right <= r.Left || r.Bottom <= r.Top {
		return fallback
	}
	if r.Left < -500 || r.Top < -500 || r.Right > 2200 || r.Bottom > 1600 {
		return fallback
	}
	return r
}

func hudLayoutRectsOverlap(a, b HUDLayoutRect) bool {
	return a.Left < b.Right && a.Right > b.Left && a.Top < b.Bottom && a.Bottom > b.Top
}

func hudUtilityLayoutConflicts(cfg HUDLayoutConfig) bool {
	utility := []HUDLayoutRect{cfg.Bug, cfg.SupportDev, cfg.Discord}
	primary := []HUDLayoutRect{cfg.Profile, cfg.Local, cfg.Global}
	for _, u := range utility {
		for _, p := range primary {
			if hudLayoutRectsOverlap(u, p) {
				return true
			}
		}
	}
	for i := 0; i < len(utility); i++ {
		for j := i + 1; j < len(utility); j++ {
			if hudLayoutRectsOverlap(utility[i], utility[j]) {
				return true
			}
		}
	}
	return false
}

func matchHUDLayoutRectSize(r HUDLayoutRect, reference HUDLayoutRect) HUDLayoutRect {
	targetW := reference.Right - reference.Left
	targetH := reference.Bottom - reference.Top
	if targetW <= 0 || targetH <= 0 {
		return r
	}
	cx := (r.Left + r.Right) / 2
	cy := (r.Top + r.Bottom) / 2
	r.Left = cx - targetW/2
	r.Top = cy - targetH/2
	r.Right = r.Left + targetW
	r.Bottom = r.Top + targetH
	return r
}

func alignNearHUDRow(a, b, c HUDLayoutRect) (HUDLayoutRect, HUDLayoutRect, HUDLayoutRect) {
	ca := (a.Top + a.Bottom) / 2
	cb := (b.Top + b.Bottom) / 2
	cc := (c.Top + c.Bottom) / 2
	minC, maxC := ca, ca
	for _, v := range []int32{cb, cc} {
		if v < minC {
			minC = v
		}
		if v > maxC {
			maxC = v
		}
	}
	if maxC-minC > 6 {
		return a, b, c
	}
	center := (ca + cb + cc) / 3
	height := a.Bottom - a.Top
	if h := b.Bottom - b.Top; h > height {
		height = h
	}
	if h := c.Bottom - c.Top; h > height {
		height = h
	}
	if height <= 0 {
		height = 78
	}
	set := func(r HUDLayoutRect) HUDLayoutRect {
		r.Top = center - height/2
		r.Bottom = r.Top + height
		return r
	}
	return set(a), set(b), set(c)
}

func persistentHUDLayoutPath() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "CursorControl", "hud_layout.json")
	}
	if dataRoot != "" {
		return filepath.Join(dataRoot, "hud_layout.json")
	}
	return ""
}

func localHUDLayoutPath() string {
	if dataRoot == "" {
		return ""
	}
	return filepath.Join(dataRoot, "hud_layout.json")
}

func applyNormalizedHUDLayout(cfg HUDLayoutConfig) HUDLayoutConfig {
	def := defaultHUDLayoutConfig()
	cfg.ModeSwitch = normalizeHUDLayoutRect(cfg.ModeSwitch, def.ModeSwitch)
	cfg.SpaceCache = normalizeHUDLayoutRect(cfg.SpaceCache, def.SpaceCache)
	cfg.Garage = normalizeHUDLayoutRect(cfg.Garage, def.Garage)
	cfg.Profile = normalizeHUDLayoutRect(cfg.Profile, def.Profile)
	cfg.Local = normalizeHUDLayoutRect(cfg.Local, def.Local)
	cfg.Global = normalizeHUDLayoutRect(cfg.Global, def.Global)
	cfg.Discord = normalizeHUDLayoutRect(cfg.Discord, def.Discord)
	cfg.Bug = normalizeHUDLayoutRect(cfg.Bug, def.Bug)
	cfg.SupportDev = normalizeHUDLayoutRect(cfg.SupportDev, def.SupportDev)

	// v241: new compact pixel-art action buttons keep their native aspect ratios.
	// Do not normalize them back to the legacy long 224x54 button footprint.
	cfg.Profile, cfg.Local, cfg.Global = alignNearHUDRow(cfg.Profile, cfg.Local, cfg.Global)

	// v426: repair only layouts that are objectively conflicting. This preserves
	// a good customised desktop layout while automatically fixing the old/bad
	// fallback exposed on another PC/laptop or a fresh install.
	if hudUtilityLayoutConflicts(cfg) {
		cfg.Bug = def.Bug
		cfg.SupportDev = def.SupportDev
		cfg.Discord = def.Discord
	}
	return cfg
}

func loadHUDLayoutConfig() {
	hudLayoutConfig = defaultHUDLayoutConfig()
	hudLayoutLoaded = true

	// v174: use a user-level config file so the layout survives game updates,
	// moving/extracting the MASTER folder, and launching a newer version.
	paths := []string{persistentHUDLayoutPath(), localHUDLayoutPath()}
	for _, path := range paths {
		if path == "" {
			continue
		}
		var cfg HUDLayoutConfig
		if err := readJSONWithRecovery(path, &cfg); err != nil {
			continue
		}
		hudLayoutConfig = applyNormalizedHUDLayout(cfg)

		// v185 one-time action-button layout repair based on the user's
		// supplied red-box positioning example.
		marker := ""
		if dir, err := os.UserConfigDir(); err == nil && dir != "" {
			marker = filepath.Join(dir, "CursorControl", "action_buttons_v185_migrated.txt")
		}
		if marker != "" {
			if _, err := os.Stat(marker); os.IsNotExist(err) {
				hudLayoutConfig.Bug = HUDLayoutRect{1100, 806, 1324, 860}
				hudLayoutConfig.SupportDev = HUDLayoutRect{1100, 874, 1324, 928}
				hudLayoutConfig.Discord = HUDLayoutRect{1100, 942, 1324, 996}
				_ = os.MkdirAll(filepath.Dir(marker), 0755)
				_ = atomicWriteFile(marker, []byte("v185"), 0644)
				saveHUDLayoutConfig()
			}
		}

		hudLayoutConfigFile = persistentHUDLayoutPath()
		return
	}

	hudLayoutConfigFile = persistentHUDLayoutPath()
}

func saveHUDLayoutConfig() {
	hudLayoutConfig = applyNormalizedHUDLayout(hudLayoutConfig)

	data, err := json.MarshalIndent(hudLayoutConfig, "", "  ")
	if err != nil {
		return
	}

	// Save to a persistent Windows user-config location.
	persistent := persistentHUDLayoutPath()
	if persistent != "" {
		_ = os.MkdirAll(filepath.Dir(persistent), 0755)
		if err := atomicWriteFile(persistent, data, 0644); err == nil {
			hudLayoutConfigFile = persistent
		}
	}

	// Also mirror beside the game for easy inspection/editing.
	local := localHUDLayoutPath()
	if local != "" && local != persistent {
		_ = os.MkdirAll(filepath.Dir(local), 0755)
		_ = atomicWriteFile(local, data, 0644)
	}
}

func designToScreenRect(r HUDLayoutRect, w, h int32) RECT {
	return RECT{
		sx(float64(r.Left), w),
		sy(float64(r.Top), h),
		sx(float64(r.Right), w),
		sy(float64(r.Bottom), h),
	}
}

func screenToDesignX(x, w int32) int32 {
	if w <= 0 {
		return x
	}
	return int32(float64(x) * 1536.0 / float64(w))
}

func screenToDesignY(y, h int32) int32 {
	if h <= 0 {
		return y
	}
	return int32(float64(y) * 1024.0 / float64(h))
}

func hudLayoutRectByIndex(i int) HUDLayoutRect {
	switch i {
	case 0:
		return hudLayoutConfig.ModeSwitch
	case 1:
		return hudLayoutConfig.SpaceCache
	case 2:
		return hudLayoutConfig.Garage
	case 3:
		return hudLayoutConfig.Profile
	case 4:
		return hudLayoutConfig.Local
	case 5:
		return hudLayoutConfig.Global
	case 6:
		return hudLayoutConfig.Discord
	case 7:
		return hudLayoutConfig.Bug
	case 8:
		return hudLayoutConfig.SupportDev
	}
	return HUDLayoutRect{}
}

func setHUDLayoutRectByIndex(i int, r HUDLayoutRect) {
	switch i {
	case 0:
		hudLayoutConfig.ModeSwitch = r
	case 1:
		hudLayoutConfig.SpaceCache = r
	case 2:
		hudLayoutConfig.Garage = r
	case 3:
		hudLayoutConfig.Profile = r
	case 4:
		hudLayoutConfig.Local = r
	case 5:
		hudLayoutConfig.Global = r
	case 6:
		hudLayoutConfig.Discord = r
	case 7:
		hudLayoutConfig.Bug = r
	case 8:
		hudLayoutConfig.SupportDev = r
	}
}

func hudLayoutName(i int) string {
	switch i {
	case 0:
		return "MODE SWITCH"
	case 1:
		return "OPEN SPACE CACHE"
	case 2:
		return "GARAGE"
	case 3:
		return "PROFILE"
	case 4:
		return "LOCAL"
	case 5:
		return "GLOBAL"
	case 6:
		return "DISCORD"
	case 7:
		return "REPORT BUG"
	case 8:
		return "SUPPORT DEV"
	}
	return "NONE"
}

func hudLayoutHitTest(p FPoint, w, h int32) int {
	// Top mode button is always available.
	if pointInRect(p, enduranceModeButtonRect(w, h)) {
		return 0
	}

	// Endurance utility buttons exist only in Endurance waiting state.
	if enduranceActive() {
		_, cacheR, garageR := enduranceSpaceUIRects(w, h)
		if pointInRect(p, cacheR) {
			return 1
		}
		if pointInRect(p, garageR) {
			return 2
		}
	}

	rects := quickAccessRects(w, h)
	for i, r := range rects {
		if pointInRect(p, r) {
			return i + 3
		}
	}
	if pointInRect(p, quickDiscordLoginRect(w, h)) {
		return 6
	}
	if pointInRect(p, quickBugReportRect(w, h)) {
		return 7
	}
	if pointInRect(p, quickSupportDevRect(w, h)) {
		return 8
	}
	return -1
}

func moveHUDLayoutSelection(dx, dy int32) {
	if hudLayoutSelected < 0 {
		return
	}
	r := hudLayoutRectByIndex(hudLayoutSelected)
	r.Left += dx
	r.Right += dx
	r.Top += dy
	r.Bottom += dy
	setHUDLayoutRectByIndex(hudLayoutSelected, r)
	saveHUDLayoutConfig()
}

func externalAsset(parts ...string) string {
	if assetRoot == "" {
		return ""
	}
	all := append([]string{assetRoot}, parts...)
	return filepath.Join(all...)
}

func initAudio() {
	audioAssets := externalAsset("audio")

	// v306: initialise SFX first on the dedicated PCM/waveOut bus. Missing optional
	// effects never disable music or any other sound.
	initSFXBus(audioAssets)

	// MUSIC BUS: MCI is now reserved exclusively for long-looping MP3 tracks.
	bossPath := filepath.Join(audioAssets, "precision_theme.mp3")
	if st, err := os.Stat(bossPath); err == nil && !st.IsDir() {
		mci("stop boss")
		mci("close boss")
		if mci(`open "` + bossPath + `" type mpegvideo alias boss`) {
			audioReady = true
		}
	}

	endurancePath := filepath.Join(audioAssets, "endurance_theme.mp3")
	if st, err := os.Stat(endurancePath); err == nil && !st.IsDir() {
		enduranceMusicReady = reopenMusicAlias("endurance_music", "", endurancePath)
	}
	starbasePath := filepath.Join(audioAssets, "starbase_theme.mp3")
	if st, err := os.Stat(starbasePath); err == nil && !st.IsDir() {
		starbaseMusicReady = reopenMusicAlias("starbase_music", "", starbasePath)
	}
	if survivalPath := survivalSectionMusicPath(1); survivalPath != "" {
		survivalMusicReady = reopenMusicAlias("survival_music", "", survivalPath)
	}
	if survivalPath := survivalSectionMusicPath(2); survivalPath != "" {
		survivalSection2MusicReady = reopenMusicAlias("survival_section2", "", survivalPath)
	}
	if survivalPath := survivalSectionMusicPath(3); survivalPath != "" {
		survivalSection3MusicReady = reopenMusicAlias("survival_section3", "", survivalPath)
	}

	// SFX can operate even if one music file failed to open.
	if !audioReady {
		audioReady = len(sfxBus.effects) > 0 || enduranceMusicReady || survivalMusicReady || starbaseMusicReady
	}

	initSurvivalBoss1Audio()
	initSurvivalBoss2Audio()
	applyAudioVolumes()
}

func reopenMusicAlias(alias, wavPath, mp3Path string) bool {
	if alias == "" {
		return false
	}
	mci("stop " + alias)
	mci("close " + alias)

	// Preferred path: PCM WAV through waveaudio for clean looping.
	if wavPath != "" {
		if st, err := os.Stat(wavPath); err == nil && !st.IsDir() {
			if mci(`open "` + wavPath + `" type waveaudio alias ` + alias) {
				return true
			}
			// Some Windows builds are happier when MCI auto-detects the WAV driver.
			if mci(`open "` + wavPath + `" alias ` + alias) {
				return true
			}
		}
	}

	// Compatibility fallback: keep the CORRECT mode theme rather than silently
	// falling through to Precision music if the WAV driver refuses the file.
	if mp3Path != "" {
		if st, err := os.Stat(mp3Path); err == nil && !st.IsDir() {
			if mci(`open "` + mp3Path + `" type mpegvideo alias ` + alias) {
				return true
			}
		}
	}
	return false
}

func ensureEnduranceMusicAlias() bool {
	if enduranceMusicReady {
		return true
	}
	audio := externalAsset("audio")
	enduranceMusicReady = reopenMusicAlias(
		"endurance_music",
		"",
		filepath.Join(audio, "endurance_theme.mp3"),
	)
	return enduranceMusicReady
}

func ensureStarbaseMusicAlias() bool {
	if starbaseMusicReady {
		return true
	}
	audio := externalAsset("audio")
	starbaseMusicReady = reopenMusicAlias(
		"starbase_music",
		"",
		filepath.Join(audio, "starbase_theme.mp3"),
	)
	return starbaseMusicReady
}

func ensureSurvivalSectionAlias(section int) string {
	alias := "survival_music"
	ready := &survivalMusicReady
	if section == 2 {
		alias = "survival_section2"
		ready = &survivalSection2MusicReady
	} else if section == 3 {
		alias = "survival_section3"
		ready = &survivalSection3MusicReady
	}
	if *ready {
		return alias
	}
	*ready = reopenMusicAlias(alias, "", survivalSectionMusicFallbackPath(section))
	if *ready {
		return alias
	}
	return ""
}

func currentMusicAlias() string {
	if overlayMode == OverlayAFKSingularity {
		if ensureStarbaseMusicAlias() {
			return "starbase_music"
		}
		return ""
	}
	if survivalActive() {
		if survivalBoss1Active() || survivalBoss2Active() || survivalBoss3Active() {
			return survivalActiveBossMusicAlias()
		}
		section := survivalMusicSectionForWave(survivalDisplayWave())
		return ensureSurvivalSectionAlias(section)
	}
	if enduranceActive() {
		if ensureEnduranceMusicAlias() {
			return "endurance_music"
		}
		return ""
	}
	// Precision is the only mode allowed to use the legacy boss_nova track.
	return "boss"
}
func switchModeMusic() {
	// Do not gate mode music on the cached global audioReady flag. A mode-specific
	// MCI alias can be reopened independently even after another mode invalidated it.
	// Stop every possible music owner before selecting the new mode.
	for _, alias := range []string{
		"boss",
		"endurance_music",
		"survival_music",
		"survival_section2",
		"survival_section3",
		"survival_boss1_music",
		"survival_boss2_music",
		"survival_boss3_music",
		"starbase_music",
	} {
		mci("stop " + alias)
	}

	alias := currentMusicAlias()
	if alias == "" {
		// Never substitute Precision music when a mode-specific track fails.
		status = "MODE MUSIC ERROR // THEME COULD NOT BE OPENED"
		bossStarted = false
		bossPaused = false
		return
	}

	mci("seek " + alias + " to start")
	mci(fmt.Sprintf("setaudio %s volume to %d", alias, gameMeta.MusicVolume*10))
	if !mci("play " + alias + " repeat") {
		// One reopen/retry for the selected mode only.
		if overlayMode == OverlayAFKSingularity {
			starbaseMusicReady = false
			if ensureStarbaseMusicAlias() {
				alias = "starbase_music"
			}
		} else if enduranceActive() {
			enduranceMusicReady = false
			if ensureEnduranceMusicAlias() {
				alias = "endurance_music"
			}
		} else if survivalActive() && !survivalBoss1Active() && !survivalBoss2Active() && !survivalBoss3Active() {
			section := survivalMusicSectionForWave(survivalDisplayWave())
			switch section {
			case 2:
				survivalSection2MusicReady = false
			case 3:
				survivalSection3MusicReady = false
			default:
				survivalMusicReady = false
			}
			alias = ensureSurvivalSectionAlias(section)
		}
		if alias == "" {
			status = "MODE MUSIC ERROR // THEME COULD NOT BE OPENED"
			bossStarted = false
			return
		}
		mci("seek " + alias + " to start")
		mci(fmt.Sprintf("setaudio %s volume to %d", alias, gameMeta.MusicVolume*10))
		if !mci("play " + alias + " repeat") {
			status = "MODE MUSIC ERROR // PLAYBACK FAILED"
			bossStarted = false
			return
		}
	}

	bossStarted = true
	bossPaused = false
}
func startBossMusic() {
	if !audioReady || bossStarted {
		return
	}
	alias := currentMusicAlias()
	mci("seek " + alias + " to start")
	mci("play " + alias + " repeat")
	bossStarted = true
}

func playHitSound() {
	if !audioReady || !hitAudioReady || !hitFXEnabled {
		return
	}
	playSFX("hit")
}

func playLevelUpSound() {
	if audioReady && levelAudioReady {
		playSFX("levelup")
	}
}

func playRechamberSound() {
	if audioReady && rechamberAudioReady {
		playSFX("rechamber")
	}
}

func playKongIntroSound() {
	if audioReady {
		playSFX("konggames_intro")
	}
}

func stopAlienChargeSound() { stopLoopSFX("alien_charge") }

func playAlienChargeSound() {
	if audioReady && alienChargeAudioReady {
		playSFX("alien_charge")
	}
}

func playAlienImpactSound() {
	if mainHwnd != 0 && enduranceActive() {
		w, hgt := getClient(mainHwnd)
		ar := arenaRect(w, hgt)
		addPolishVFX(polishVFXBoss, float32(alienBossCannonX()-float64(ar.Left)), float32(alienBossCannonY()-float64(ar.Top)), 0.30)
	}
	if !audioReady || !alienImpactAudioReady {
		return
	}
	playOneShotAsync("alien_impact")
}

func playPowerupPickupSound() {
	if audioReady && powerupPickupAudioReady {
		playSFX("powerup_pickup")
	}
}

func playUIButtonClickSound() {
	if audioReady && buttonClickAudioReady {
		playSFX("ui_button_click")
	}
}

func playGarageBuySound() {
	if audioReady && buyAudioReady {
		playSFX("garage_buy")
	}
}

func playSurvivalHitSound() {
	if audioReady && survivalHitAudioReady {
		playSFX("survival_hit")
	}
}

func playSurvivalDamageTakenSound() {
	if audioReady {
		playSFX("survival_damage_taken")
	}
}

func playBossClickEffect() {
	if audioReady && bossClickAudioReady {
		playSFX("boss_click_effect")
	}
}

func playBoss1RoarSound() {
	if !audioReady {
		return
	}
	// Do not permanently trust the startup readiness snapshot. On some systems
	// the Boss 1 roar was checked before the external audio tree was fully ready,
	// leaving every later roar request silently gated off for the whole session.
	// Re-check the live SFX registry and lazily load the valid PCM asset if needed.
	if !sfxLoaded("boss1_roar") {
		path := filepath.Join(externalAsset("audio"), "boss_1_roar.wav")
		boss1RoarAudioReady = loadPCMEffect("boss1_roar", path)
	} else {
		boss1RoarAudioReady = true
	}
	if boss1RoarAudioReady {
		playSFX("boss1_roar")
	}
}

func startShieldProtectSound() {
	if !audioReady || !shieldProtectAudioReady || shieldProtectSoundPlaying {
		return
	}
	startLoopSFX("shield_protect")
	shieldProtectSoundPlaying = true
}

func stopShieldProtectSound() {
	stopLoopSFX("shield_protect")
	shieldProtectSoundPlaying = false
}

func syncShieldProtectSound() {
	if enduranceShieldActive() && enduranceActive() && state == StatePlaying {
		startShieldProtectSound()
	} else {
		stopShieldProtectSound()
	}
}

func playWarpReadySound() {
	if audioReady && warpCueAudioReady {
		playSFX("warp_ready")
	}
}

func playWarpRocketSound() {
	if audioReady && warpRocketAudioReady {
		playSFX("warp_rocket")
	}
}

func playEnduranceExplodeSound() {
	if audioReady && enduranceExplodeAudioReady && hitFXEnabled {
		playSFX("endurance_explode")
	}
}

// Survival destruction audio uses the same meteorite explosion asset, but all MCI
// work is kept off the click/game thread so rapid target elimination never hitches input.
func playSurvivalExplodeSound() {
	if audioReady && enduranceExplodeAudioReady {
		playSFX("endurance_explode")
	}
}

func playEnduranceFailSound() {
	if audioReady && enduranceFailAudioReady {
		playSFX("endurance_fail")
	}
}

func playSpaceCacheSound() {
	if audioReady && spaceCacheAudioReady {
		playSFX("space_cache")
	}
}

func stopWarpSounds() { /* one-shots self-terminate on the SFX bus */ }

func pauseBossForFailure() {
	if !audioReady {
		return
	}
	if bossStarted && !bossPaused {
		mci("pause " + currentMusicAlias())
		bossPaused = true
	}
	// Endurance has one fixed failure sound. The user-selectable failure-sound
	// rotation is intentionally Standard-mode only.
	stopAllFailureSounds()
	if enduranceActive() || survivalActive() {
		playEnduranceFailSound()
		return
	}
	alias := failureSoundAlias()
	playSFX(alias)
}

func resumeBossAfterFailure() {
	if !audioReady {
		return
	}
	stopAllFailureSounds()
	if bossPaused {
		alias := currentMusicAlias()
		if !mci("resume " + alias) {
			mci("play " + alias + " repeat")
		}
		bossPaused = false
		bossStarted = true
	}
}

func shutdownAudio() {
	stopAllSFXLoops()
	if audioReady {
		for _, alias := range allMCIAliases {
			mci("stop " + alias)
			mci("close " + alias)
		}
	}
	audioReady = false
	bossStarted = false
	bossPaused = false
	enduranceMusicReady = false
	starbaseMusicReady = false
	survivalMusicReady = false
	survivalSection2MusicReady = false
	survivalSection3MusicReady = false
	survivalBoss1MusicReady = false
	survivalBoss2MusicReady = false
	survivalBoss3MusicReady = false
	if audioDir != "" {
		os.RemoveAll(audioDir)
		audioDir = ""
	}
}

func initGDIPlus() bool {
	input := GDIPStartupInput{GdiplusVersion: 1}
	st, _, _ := gdiplusStartup.Call(uintptr(unsafe.Pointer(&gdipToken)), uintptr(unsafe.Pointer(&input)), 0)
	return st == 0 && gdipToken != 0
}
func shutdownGDIPlus() {
	if gdipToken != 0 {
		gdiplusShutdown.Call(gdipToken)
		gdipToken = 0
	}
}
func gdipARGB(a, r, g, b byte) uintptr {
	return uintptr(uint32(a)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b))
}
func colorRefToARGB(c uintptr, a byte) uintptr {
	return gdipARGB(a, byte(c&255), byte((c>>8)&255), byte((c>>16)&255))
}
func gdipGraphics(hdc uintptr) (uintptr, bool) {
	if gdipToken == 0 {
		return 0, false
	}
	var g uintptr
	st, _, _ := gdipCreateFromHDC.Call(hdc, uintptr(unsafe.Pointer(&g)))
	if st != 0 || g == 0 {
		return 0, false
	}
	gdipSetSmoothingMode.Call(g, 4)
	gdipSetPixelOffsetMode.Call(g, 4)
	return g, true
}
func gdipPen(c uintptr, w float32) uintptr {
	var p uintptr
	gdipCreatePen1.Call(c, uintptr(math.Float32bits(w)), 2, uintptr(unsafe.Pointer(&p)))
	if p != 0 {
		gdipSetPenLineJoin.Call(p, 2)
		gdipSetPenStartCap.Call(p, 2)
		gdipSetPenEndCap.Call(p, 2)
	}
	return p
}
func gdipBrush(c uintptr) uintptr {
	var b uintptr
	gdipCreateSolidFill.Call(c, uintptr(unsafe.Pointer(&b)))
	return b
}
func gdipFillCircle(g uintptr, x, y, r int32, c uintptr) {
	b := gdipBrush(c)
	if b == 0 {
		return
	}
	defer gdipDeleteBrush.Call(b)
	gdipFillEllipseI.Call(g, b, uintptr(x-r), uintptr(y-r), uintptr(r*2), uintptr(r*2))
}
func gdipStrokeCircle(g uintptr, x, y, r int32, c uintptr, w float32) {
	p := gdipPen(c, w)
	if p == 0 {
		return
	}
	defer gdipDeletePen.Call(p)
	gdipDrawEllipseI.Call(g, p, uintptr(x-r), uintptr(y-r), uintptr(r*2), uintptr(r*2))
}

func screenShakeScale() float64 {
	if gameMeta.ReducedMotion {
		return 0
	}
	switch gameMeta.ScreenShakeStrength {
	case 1:
		return 0
	case 2:
		return 0.50
	case 3:
		return 1.45
	default:
		return 1.0
	}
}

func arcadeBlue() uintptr       { return rgb(18, 112, 224) }
func arcadeBlueLight() uintptr  { return rgb(58, 166, 255) }
func arcadeBlueDark() uintptr   { return rgb(4, 56, 127) }
func arcadeGreen() uintptr      { return rgb(115, 219, 42) }
func arcadeGreenLight() uintptr { return rgb(173, 247, 76) }
func arcadeGreenDark() uintptr  { return rgb(46, 127, 20) }
func arcadeInk() uintptr        { return rgb(12, 20, 34) }
func drawBevelPanel(hdc uintptr, r RECT, face, light, dark uintptr, t int32) {
	style := gameMeta.HUDCornerStyle
	shadowDX, shadowDY := sx(5, 1536), sy(6, 1024)
	if style == 1 {
		shadowDX, shadowDY = sx(2, 1536), sy(3, 1024)
		if t > 2 {
			t = 2
		}
	}
	if style == 3 {
		shadowDX, shadowDY = sx(7, 1536), sy(8, 1024)
		if t < 4 {
			t = 4
		}
	}
	shadow := RECT{r.Left + shadowDX, r.Top + shadowDY, r.Right + shadowDX, r.Bottom + shadowDY}
	fillSolidRect(hdc, shadow, rgb(1, 11, 31))
	fillSolidRect(hdc, r, rgb(3, 12, 31))
	in := RECT{r.Left + t, r.Top + t, r.Right - t, r.Bottom - t}
	fillSolidRect(hdc, in, face)
	drawLineSimple(hdc, in.Left, in.Top, in.Right, in.Top, 2, light)
	drawLineSimple(hdc, in.Left, in.Top, in.Left, in.Bottom, 2, light)
	drawLineSimple(hdc, in.Left, in.Bottom, in.Right, in.Bottom, 3, dark)
	drawLineSimple(hdc, in.Right, in.Top, in.Right, in.Bottom, 3, dark)
	if in.Right-in.Left > 12 && in.Bottom-in.Top > 12 {
		rim := blendColor(face, rgb(255, 255, 255), 0.10)
		drawLineSimple(hdc, in.Left+4, in.Top+4, in.Right-4, in.Top+4, 1, rim)
	}
	// Corner-style overrides are decorative only: hitboxes and panel dimensions never change.
	switch style {
	case 1: // compact
		// restrained corner ticks
		drawLineSimple(hdc, in.Left+3, in.Top+3, in.Left+9, in.Top+3, 1, light)
		drawLineSimple(hdc, in.Right-9, in.Bottom-3, in.Right-3, in.Bottom-3, 1, dark)
	case 2: // rounded-ish sci-fi: bright curved-looking bracket approximation without changing rectangular hitboxes
		c := blendColor(light, rgb(255, 255, 255), 0.18)
		k := int32(7)
		drawLineSimple(hdc, in.Left, in.Top+k, in.Left+2, in.Top+2, 2, c)
		drawLineSimple(hdc, in.Left+2, in.Top+2, in.Left+k, in.Top, 2, c)
		drawLineSimple(hdc, in.Right-k, in.Top, in.Right-2, in.Top+2, 2, c)
		drawLineSimple(hdc, in.Right-2, in.Top+2, in.Right, in.Top+k, 2, c)
		drawLineSimple(hdc, in.Left, in.Bottom-k, in.Left+2, in.Bottom-2, 2, dark)
		drawLineSimple(hdc, in.Left+2, in.Bottom-2, in.Left+k, in.Bottom, 2, dark)
		drawLineSimple(hdc, in.Right-k, in.Bottom, in.Right-2, in.Bottom-2, 2, dark)
		drawLineSimple(hdc, in.Right-2, in.Bottom-2, in.Right, in.Bottom-k, 2, dark)
	case 3: // heavy industrial
		inner := RECT{in.Left + 5, in.Top + 5, in.Right - 5, in.Bottom - 5}
		if inner.Right > inner.Left && inner.Bottom > inner.Top {
			drawOutlineRect(hdc, inner, blendColor(dark, rgb(255, 255, 255), 0.18), 2)
		}
		// tiny rivets
		for _, pt := range [][2]int32{{in.Left + 6, in.Top + 6}, {in.Right - 7, in.Top + 6}, {in.Left + 6, in.Bottom - 7}, {in.Right - 7, in.Bottom - 7}} {
			fillSolidRect(hdc, RECT{pt[0], pt[1], pt[0] + 2, pt[1] + 2}, light)
		}
	}
}
func themedHUDPanelPalette(active bool) (uintptr, uintptr, uintptr) {
	face := arcadeBlue()
	light := arcadeBlueLight()
	dark := arcadeBlueDark()
	switch gameMeta.HUDBackgroundTheme {
	case 1: // dark glass
		face = rgb(24, 31, 41)
		light = rgb(116, 165, 194)
		dark = rgb(5, 10, 17)
	case 2: // military
		face = rgb(55, 67, 49)
		light = rgb(168, 190, 123)
		dark = rgb(18, 25, 17)
	case 3: // terminal
		face = rgb(7, 31, 16)
		light = rgb(73, 239, 146)
		dark = rgb(1, 12, 6)
	case 4: // minimal
		face = rgb(35, 42, 51)
		light = rgb(193, 205, 216)
		dark = rgb(12, 16, 22)
	case 5: // industrial
		face = rgb(62, 68, 76)
		light = rgb(225, 175, 82)
		dark = rgb(22, 25, 30)
	}
	if !active {
		face = blendColor(face, rgb(0, 0, 0), 0.24)
		light = blendColor(light, face, 0.18)
		dark = blendColor(dark, rgb(0, 0, 0), 0.22)
	}
	return face, light, dark
}

func themedHUDCardPalette(variant int) (uintptr, uintptr, uintptr) {
	face, light, dark := themedHUDPanelPalette(false)
	switch variant {
	case 1:
		face = blendColor(face, light, 0.10)
	case 2:
		face = blendColor(face, rgb(0, 0, 0), 0.18)
	}
	return face, light, dark
}

func themedHUDHeaderColor() uintptr {
	face, _, dark := themedHUDPanelPalette(true)
	return blendColor(dark, face, 0.28)
}

func drawStudioPanel(hdc uintptr, r RECT, active bool) {
	f, l, d := themedHUDPanelPalette(active)
	drawBevelPanel(hdc, r, f, l, d, 3)
}
func drawStudioButtonBase(hdc uintptr, r RECT, active bool) {
	baseFace, baseLight, baseDark := themedHUDPanelPalette(false)
	face := blendColor(baseFace, baseLight, 0.08)
	light := baseLight
	dark := baseDark
	if active {
		// Active/primary buttons retain a warm identity but are still harmonised with the selected HUD theme.
		face = blendColor(rgb(227, 103, 7), baseFace, 0.22)
		light = blendColor(rgb(255, 221, 52), baseLight, 0.14)
		dark = blendColor(rgb(111, 38, 0), baseDark, 0.20)
	}
	hovered := pointInRect(cursorPos, r)
	pressed := polishButtonPressed(r)
	if hovered {
		switch gameMeta.ButtonHoverEffect {
		case 0: // brighten
			face = blendColor(face, rgb(255, 255, 255), 0.18)
			light = blendColor(light, rgb(255, 255, 255), 0.30)
		case 1: // outline - preserve fill and make the edge unmistakable
			light = blendColor(light, rgb(255, 255, 255), 0.20)
		case 2: // glow - animated luminance so it reads as a real glow, not just another outline
			pulse := 0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())/1000.0*5.8)
			face = blendColor(face, light, 0.16+0.12*pulse)
			light = blendColor(light, rgb(255, 255, 255), 0.40+0.20*pulse)
		case 3: // pulse - whole button visibly breathes
			pulse := 0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())/1000.0*7.0)
			face = blendColor(face, light, 0.12+0.30*pulse)
			light = blendColor(light, rgb(255, 255, 255), 0.18+0.42*pulse)
		}
	}
	if pressed {
		face = blendColor(face, rgb(0, 0, 0), 0.20)
		light = blendColor(light, rgb(255, 235, 92), 0.18)
	}
	// Glow is intentionally drawn outside the normal button edge. The hitbox is unchanged.
	if hovered && gameMeta.ButtonHoverEffect == 2 {
		pulse := 0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())/1000.0*5.8)
		outer := blendColor(light, rgb(255, 255, 255), 0.24+0.30*pulse)
		drawOutlineRect(hdc, RECT{r.Left - 4, r.Top - 4, r.Right + 4, r.Bottom + 4}, outer, 2)
		drawOutlineRect(hdc, RECT{r.Left - 2, r.Top - 2, r.Right + 2, r.Bottom + 2}, light, 2)
	}
	drawBevelPanel(hdc, r, face, light, dark, 3)
	drawLineSimple(hdc, r.Left+6, r.Top+7, r.Left+6, r.Bottom-7, 3, light)
	if hovered {
		switch gameMeta.ButtonHoverEffect {
		case 0:
			// Brighten also lays a soft top sheen so the difference is obvious on dark themes.
			drawLineSimple(hdc, r.Left+8, r.Top+3, r.Right-8, r.Top+3, 2, blendColor(light, rgb(255, 255, 255), 0.35))
		case 1:
			drawOutlineRect(hdc, RECT{r.Left + 1, r.Top + 1, r.Right - 1, r.Bottom - 1}, light, 3)
			drawOutlineRect(hdc, RECT{r.Left + 5, r.Top + 5, r.Right - 5, r.Bottom - 5}, blendColor(light, rgb(255, 255, 255), 0.22), 1)
		case 2:
			drawOutlineRect(hdc, RECT{r.Left + 2, r.Top + 2, r.Right - 2, r.Bottom - 2}, blendColor(light, rgb(255, 255, 255), 0.50), 2)
		case 3:
			pulse := 0.5 + 0.5*math.Sin(float64(time.Now().UnixMilli())/1000.0*7.0)
			c := blendColor(light, rgb(255, 255, 255), 0.20+0.55*pulse)
			drawOutlineRect(hdc, RECT{r.Left + 2, r.Top + 2, r.Right - 2, r.Bottom - 2}, c, 3)
		}
	}
}
func drawArcadeLabel(hdc uintptr, r RECT, label string, c uintptr) {
	if hudSmallFont == 0 {
		return
	}
	o, _, _ := selectObject.Call(hdc, hudSmallFont)
	defer selectObject.Call(hdc, o)
	setBkMode.Call(hdc, TRANSPARENT)
	setTextColor.Call(hdc, c)
	centeredTextOut(hdc, r.Left, r.Right, verticallyCenteredTextY(hdc, hudSmallFont, r, label), label)
}

func readExternalBytes(parts ...string) []byte {
	f := externalAsset(parts...)
	if f == "" {
		return nil
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return nil
	}
	return b
}

func initUI() {
	uiBaseBGRA = readExternalBytes("ui", "ui_base.bgra")
	cursorControlLogoBGRA = readExternalBytes("ui", "cursor_control_logo.bgra")
	cursorControlLogoHUDBGRA = readExternalBytes("ui", "cursor_control_logo_hud.bgra")
	hudIconTimeBGRA = readExternalBytes("ui", "hud_icon_time.bgra")
	hudIconScoreBGRA = readExternalBytes("ui", "hud_icon_score.bgra")
	hudIconStreakBGRA = readExternalBytes("ui", "hud_icon_streak.bgra")
	hudIconBestBGRA = readExternalBytes("ui", "hud_icon_best.bgra")
	hudIconDifficultyBGRA = readExternalBytes("ui", "hud_icon_difficulty.bgra")
	discordLoginButtonBGRA = readExternalBytes("ui", "discord_login_button.bgra")
	discordLoggedInButtonBGRA = readExternalBytes("ui", "discord_logged_in_button.bgra")
	bugReportButtonBGRA = readExternalBytes("ui", "bug_report_button.bgra")
	supportDevButtonBGRA = readExternalBytes("ui", "support_dev_button.bgra")
	profileButtonBGRA = readExternalBytes("ui", "profile_button.bgra")
	localButtonBGRA = readExternalBytes("ui", "local_button.bgra")
	globalButtonBGRA = readExternalBytes("ui", "global_button.bgra")
	precisionModeButtonBGRA = readExternalBytes("ui", "precision_mode_button.bgra")
	enduranceModeButtonBGRA = readExternalBytes("ui", "endurance_mode_button.bgra")
	selectModeButtonBGRA = readExternalBytes("ui", "select_mode_button.bgra")
	modePrecisionCardBGRA = readExternalBytes("ui", "mode_precision_card.bgra")
	modeEnduranceCardBGRA = readExternalBytes("ui", "mode_endurance_card.bgra")
	modeSurvivalCardBGRA = readExternalBytes("ui", "mode_survival_card.bgra")
	modeStarbaseCardBGRA = readExternalBytes("ui", "mode_starbase_card.bgra")
	starbaseBackgroundBGRA = readExternalBytes("ui", "starbase_background.bgra")
	starbaseSingularityBGRA = readExternalBytes("ui", "starbase_singularity_backdrop.bgra")
	starbaseMoonRockBGRA = readExternalBytes("ui", "starbase_moon_rock.bgra")
	starbaseLogoWordmarkBGRA = readExternalBytes("ui", "starbase_logo_wordmark.bgra")
	loadAFKFacilityAssets()
	loadAFKStarbaseControlAssets()
	loadAFKResearchAssets()
	loadAFKModuleAssets()
	loadAFKCraftMaterialAssets()
	loadAFKOperatorAssets()
	hudNetworkTopBGRA = readExternalBytes("ui", "hud_network_top.bgra")
	hudNetworkBottomBGRA = readExternalBytes("ui", "hud_network_bottom.bgra")
	failedNormalBGRA = readExternalBytes("ui", "failed_normal.bgra")
	failedEnduranceBGRA = readExternalBytes("ui", "failed_endurance.bgra")
	spaceCoinBGRA = readExternalBytes("ui", "space_coin.bgra")
	powerupShieldBGRA, _ = os.ReadFile(filepath.Join(textureRoot, "powerup_shield.bgra"))
	powerupTimeBGRA, _ = os.ReadFile(filepath.Join(textureRoot, "powerup_slow.bgra"))

	if cacheRoot != "" {
		hazardBlueBGRA, _ = os.ReadFile(filepath.Join(textureRoot, "hazard_blue.bgra"))
		hazardOrangeBGRA, _ = os.ReadFile(filepath.Join(textureRoot, "hazard_orange.bgra"))
	}
	spaceCacheClosedBGRA = readExternalBytes("ui", "space_cache_closed.bgra")
	spaceCacheOpenBGRA = readExternalBytes("ui", "space_cache_open.bgra")
	starCacheBGRA = readExternalBytes("ui", "star_cache.bgra")
	expeditionHangarBGRA = readExternalBytes("backgrounds", "starbase_expedition_hangar.bgra")
	spaceCoinBarBGRA = readExternalBytes("ui", "space_coin_bar.bgra")
	spaceCacheButtonBGRA = readExternalBytes("ui", "space_cache_button.bgra")
	garageButtonBGRA = readExternalBytes("ui", "garage_button.bgra")
	defaultShipBGRA, _ = os.ReadFile(filepath.Join(textureRoot, "rocket_cursor.bgra"))
	for i := 1; i <= 12; i++ {
		spaceShipBGRA[i], _ = os.ReadFile(filepath.Join(textureRoot, fmt.Sprintf("ship_%d.bgra", i)))
	}
	uiPixels = uiBaseBGRA
	diffs[0].color = rgb(43, 210, 104)
	diffs[1].color = rgb(255, 155, 38)
	diffs[2].color = rgb(235, 55, 64)
	diffs[3].color = rgb(190, 52, 246)
	initSurvivalAssets()
}

func drawCursorControlLogo(hdc uintptr, left, top int32, compact bool) {}

func ensureCachedBGRASprite(hdc uintptr, sprite *CachedBGRASprite, data []byte, w, h int32) bool {
	if sprite == nil || w <= 0 || h <= 0 || len(data) < int(w*h*4) {
		return false
	}
	if sprite.Ready && sprite.DC != 0 && sprite.Bitmap != 0 && sprite.W == w && sprite.H == h {
		return true
	}
	if sprite.DC != 0 {
		if sprite.Old != 0 {
			selectObject.Call(sprite.DC, sprite.Old)
		}
		if sprite.Bitmap != 0 {
			deleteObject.Call(sprite.Bitmap)
		}
		deleteDC.Call(sprite.DC)
		*sprite = CachedBGRASprite{}
	}
	dc, _, _ := createCompatibleDC.Call(hdc)
	if dc == 0 {
		return false
	}
	var bits uintptr
	bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{
		BiSize:  uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth: w, BiHeight: -h,
		BiPlanes: 1, BiBitCount: 32, BiCompression: BI_RGB,
	}}
	bmp, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		deleteDC.Call(dc)
		return false
	}
	old, _, _ := selectObject.Call(dc, bmp)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(bits)), int(w*h*4)), data)
	sprite.DC = dc
	sprite.Bitmap = bmp
	sprite.Old = old
	sprite.W = w
	sprite.H = h
	sprite.Ready = true
	return true
}

func drawCachedBGRASprite(hdc uintptr, sprite *CachedBGRASprite, data []byte, srcW, srcH int32, box RECT) RECT {
	if box.Right <= box.Left || box.Bottom <= box.Top || !ensureCachedBGRASprite(hdc, sprite, data, srcW, srcH) {
		return RECT{}
	}
	boxW := box.Right - box.Left
	boxH := box.Bottom - box.Top
	dstW := boxW
	dstH := int32(float64(dstW) * float64(srcH) / float64(srcW))
	if dstH > boxH {
		dstH = boxH
		dstW = int32(float64(dstH) * float64(srcW) / float64(srcH))
	}
	dstX := box.Left + (boxW-dstW)/2
	dstY := box.Top + (boxH-dstH)/2
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(
		hdc,
		uintptr(dstX), uintptr(dstY), uintptr(dstW), uintptr(dstH),
		sprite.DC, 0, 0, uintptr(srcW), uintptr(srcH),
		blend,
	)
	return RECT{dstX, dstY, dstX + dstW, dstY + dstH}
}

func releaseCachedBGRASprite(sprite *CachedBGRASprite) {
	if sprite == nil || sprite.DC == 0 {
		return
	}
	if sprite.Old != 0 {
		selectObject.Call(sprite.DC, sprite.Old)
	}
	if sprite.Bitmap != 0 {
		deleteObject.Call(sprite.Bitmap)
	}
	deleteDC.Call(sprite.DC)
	*sprite = CachedBGRASprite{}
}

func drawRawBGRAFit(hdc uintptr, data []byte, srcW, srcH int32, box RECT) RECT {
	if srcW <= 0 || srcH <= 0 || box.Right <= box.Left || box.Bottom <= box.Top || len(data) < int(srcW*srcH*4) {
		return RECT{}
	}

	sprite := ensureRuntimeSprite(hdc, data, srcW, srcH)
	if sprite == nil || sprite.dc == 0 {
		return RECT{}
	}

	boxW := box.Right - box.Left
	boxH := box.Bottom - box.Top
	dstW := boxW
	dstH := int32(float64(dstW) * float64(srcH) / float64(srcW))
	if dstH > boxH {
		dstH = boxH
		dstW = int32(float64(dstH) * float64(srcW) / float64(srcH))
	}
	dstX := box.Left + (boxW-dstW)/2
	dstY := box.Top + (boxH-dstH)/2

	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(
		hdc,
		uintptr(dstX), uintptr(dstY), uintptr(dstW), uintptr(dstH),
		sprite.dc,
		0, 0, uintptr(srcW), uintptr(srcH),
		blend,
	)
	return RECT{dstX, dstY, dstX + dstW, dstY + dstH}
}

type alphaBoundsKey struct {
	Ptr       uintptr
	Len       int
	W, H      int32
	Threshold byte
}

var alphaBoundsCache sync.Map

func alphaBoundsBGRA(data []byte, w, h int32, threshold byte) RECT {
	if w <= 0 || h <= 0 || len(data) < int(w*h*4) {
		return RECT{0, 0, w, h}
	}
	key := alphaBoundsKey{
		Ptr: uintptr(unsafe.Pointer(&data[0])),
		Len: len(data),
		W:   w, H: h, Threshold: threshold,
	}
	if cached, ok := alphaBoundsCache.Load(key); ok {
		return cached.(RECT)
	}
	left, top := w, h
	right, bottom := int32(0), int32(0)
	found := false
	for y := int32(0); y < h; y++ {
		row := int(y * w * 4)
		for x := int32(0); x < w; x++ {
			if data[row+int(x*4)+3] <= threshold {
				continue
			}
			found = true
			if x < left {
				left = x
			}
			if y < top {
				top = y
			}
			if x+1 > right {
				right = x + 1
			}
			if y+1 > bottom {
				bottom = y + 1
			}
		}
	}
	result := RECT{0, 0, w, h}
	if found {
		result = RECT{left, top, right, bottom}
	}
	alphaBoundsCache.Store(key, result)
	return result
}

func drawRawBGRATrimmedFit(hdc uintptr, data []byte, srcW, srcH int32, box RECT) RECT {
	if srcW <= 0 || srcH <= 0 || box.Right <= box.Left || box.Bottom <= box.Top || len(data) < int(srcW*srcH*4) {
		return RECT{}
	}
	sprite := ensureRuntimeSprite(hdc, data, srcW, srcH)
	if sprite == nil || sprite.dc == 0 {
		return RECT{}
	}
	crop := alphaBoundsBGRA(data, srcW, srcH, 5)
	cropW := crop.Right - crop.Left
	cropH := crop.Bottom - crop.Top
	if cropW <= 0 || cropH <= 0 {
		return RECT{}
	}
	boxW := box.Right - box.Left
	boxH := box.Bottom - box.Top
	dstW := boxW
	dstH := int32(float64(dstW) * float64(cropH) / float64(cropW))
	if dstH > boxH {
		dstH = boxH
		dstW = int32(float64(dstH) * float64(cropW) / float64(cropH))
	}
	dstX := box.Left + (boxW-dstW)/2
	dstY := box.Top + (boxH-dstH)/2
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(dstX), uintptr(dstY), uintptr(dstW), uintptr(dstH),
		sprite.dc, uintptr(crop.Left), uintptr(crop.Top), uintptr(cropW), uintptr(cropH), blend)
	return RECT{dstX, dstY, dstX + dstW, dstY + dstH}
}

var failureOverlaySpritesPrewarmed bool
var endurancePathSafetyCheckAt time.Time

func prewarmFailureOverlaySprites(hdc uintptr) {
	if failureOverlaySpritesPrewarmed || hdc == 0 {
		return
	}
	// These are large PNG-derived BGRA assets. Building their GDI sprite lazily
	// on the exact failure frame caused an avoidable one-frame hitch.
	n := ensureRuntimeSprite(hdc, failedNormalBGRA, 1499, 933)
	e := ensureRuntimeSprite(hdc, failedEnduranceBGRA, 1254, 1219)
	sv := ensureRuntimeSprite(hdc, survivalFailedBGRA, 1672, 941)
	if n != nil && e != nil && (sv != nil || len(survivalFailedBGRA) == 0) {
		failureOverlaySpritesPrewarmed = true
	}
}

func drawFailedOverlay(hdc uintptr, w, hgt int32) {
	if state != StateFailed {
		return
	}
	ar := arenaRect(w, hgt)
	var data []byte
	var srcW, srcH int32
	maxW := sx(1000, w)
	maxH := sy(430, hgt)
	if survivalActive() && len(survivalFailedBGRA) > 0 {
		data = survivalFailedBGRA
		srcW, srcH = 1672, 941
		maxW = sx(900, w)
		maxH = sy(400, hgt)
	} else if enduranceActive() {
		data = failedEnduranceBGRA
		srcW, srcH = 1254, 1219
		maxW = sx(650, w)
		maxH = sy(470, hgt)
	} else {
		data = failedNormalBGRA
		srcW, srcH = 1499, 933
	}
	centerY := ar.Top + (ar.Bottom-ar.Top)/2 - sy(40, hgt)
	failReveal := polishFailureProgress()
	centerY += int32(float64(sy(20, hgt)) * (1.0 - failReveal))
	box := RECT{(ar.Left + ar.Right - maxW) / 2, centerY - maxH/2, (ar.Left + ar.Right + maxW) / 2, centerY + maxH/2}
	drawn := drawRawBGRAFit(hdc, data, srcW, srcH, box)

	// The supplied PNGs contain substantial transparent padding below the visible
	// artwork. Position the status from the visible alpha edge, not the full image
	// canvas, so the message sits immediately underneath the FAILED graphic.
	visibleBottomSrc := int32(588) // Normal PNG: last visible alpha row is 587.
	if survivalActive() {
		visibleBottomSrc = 900
	} else if enduranceActive() {
		visibleBottomSrc = 818 // Endurance PNG: last visible alpha row is 817.
	}
	textY := centerY + maxH/2 + sy(4, hgt)
	if drawn.Bottom > drawn.Top && srcH > 0 {
		drawnH := drawn.Bottom - drawn.Top
		visibleBottom := drawn.Top + int32((int64(visibleBottomSrc)*int64(drawnH)+int64(srcH)-1)/int64(srcH))
		textY = visibleBottom - sy(5, hgt)
	}
	if textY > ar.Bottom-sy(30, hgt) {
		textY = ar.Bottom - sy(30, hgt)
	}
	if failedReasonFont != 0 && polishFailureReasonVisible() {
		msg := strings.ToUpper(strings.TrimSpace(status))
		if msg == "" {
			msg = "RUN FAILED"
		}

		old, _, _ := selectObject.Call(hdc, failedReasonFont)
		setBkMode.Call(hdc, TRANSPARENT)

		// Massive visibility upgrade: give the failure reason its own dark panel
		// directly below the FAILED artwork, then render a strong shadow + bright
		// high-contrast text. This applies equally to Precision and Endurance.
		sz := textPixelSize(hdc, failedReasonFont, msg)
		panelPadX := sx(26, w)
		panelPadY := sy(8, hgt)
		panelW := sz.Cx + panelPadX*2
		maxPanelW := (ar.Right - ar.Left) - sx(70, w)
		if panelW > maxPanelW {
			panelW = maxPanelW
		}
		panelH := sz.Cy + panelPadY*2
		if panelH < sy(36, hgt) {
			panelH = sy(36, hgt)
		}
		panelLeft := (ar.Left + ar.Right - panelW) / 2
		panelTop := textY - sy(7, hgt)
		panel := RECT{panelLeft, panelTop, panelLeft + panelW, panelTop + panelH}

		// Deep navy/black plate with red alert rails.
		fillSolidRect(hdc, panel, rgb(2, 10, 28))
		fillSolidRect(hdc, RECT{panel.Left, panel.Top, panel.Right, panel.Top + sy(3, hgt)}, rgb(255, 47, 62))
		fillSolidRect(hdc, RECT{panel.Left, panel.Bottom - sy(3, hgt), panel.Right, panel.Bottom}, rgb(255, 47, 62))

		textTop := panel.Top + (panelH-sz.Cy)/2 - sy(1, hgt)
		textLeft := ar.Left + sx(30, w)
		textRight := ar.Right - sx(30, w)

		// Heavy black offset shadow for separation from any background/PNG glow.
		setTextColor.Call(hdc, rgb(0, 0, 0))
		centeredTextOut(hdc, textLeft+sx(2, w), textRight+sx(2, w), textTop+sy(2, hgt), msg)

		// Bright near-white main reason text with a slight warm alert tint.
		setTextColor.Call(hdc, rgb(255, 248, 250))
		centeredTextOut(hdc, textLeft, textRight, textTop, msg)

		selectObject.Call(hdc, old)
	}
}

func drawCursorControlImage(hdc uintptr, r RECT) {
	drawRawBGRAFit(hdc, cursorControlLogoBGRA, 1400, 900, r)
}

func drawCursorControlHUDImage(hdc uintptr, r RECT) {
	drawRawBGRAFit(hdc, cursorControlLogoHUDBGRA, 640, 341, r)
}

func hudIconData(index int) []byte {
	switch index {
	case 0:
		return hudIconTimeBGRA
	case 1:
		return hudIconScoreBGRA
	case 2:
		return hudIconStreakBGRA
	case 3:
		return hudIconBestBGRA
	case 4:
		return hudIconDifficultyBGRA
	default:
		return nil
	}
}

func ensureHUDIconResources(hdc uintptr) bool {
	const iconSize = 64
	for i := 0; i < 5; i++ {
		if hudIconDCs[i] != 0 && hudIconBmps[i] != 0 {
			continue
		}
		data := hudIconData(i)
		if len(data) < iconSize*iconSize*4 {
			return false
		}

		dc, _, _ := createCompatibleDC.Call(hdc)
		if dc == 0 {
			return false
		}
		var bits uintptr
		bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{
			BiSize:  uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
			BiWidth: iconSize, BiHeight: -iconSize,
			BiPlanes: 1, BiBitCount: 32, BiCompression: BI_RGB,
		}}
		bmp, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
		if bmp == 0 || bits == 0 {
			deleteDC.Call(dc)
			return false
		}
		old, _, _ := selectObject.Call(dc, bmp)
		copy(unsafe.Slice((*byte)(unsafe.Pointer(bits)), iconSize*iconSize*4), data)
		hudIconDCs[i] = dc
		hudIconBmps[i] = bmp
		hudIconOlds[i] = old
	}
	return true
}

func releaseHUDIconResources() {
	for i := 0; i < 5; i++ {
		if hudIconDCs[i] != 0 {
			if hudIconOlds[i] != 0 {
				selectObject.Call(hudIconDCs[i], hudIconOlds[i])
			}
			if hudIconBmps[i] != 0 {
				deleteObject.Call(hudIconBmps[i])
			}
			deleteDC.Call(hudIconDCs[i])
		}
		hudIconDCs[i] = 0
		hudIconBmps[i] = 0
		hudIconOlds[i] = 0
	}
}

func createHUDTextureResource(hdc uintptr, data []byte, tw, th int32) (uintptr, uintptr, uintptr, bool) {
	if len(data) < int(tw*th*4) {
		return 0, 0, 0, false
	}
	dc, _, _ := createCompatibleDC.Call(hdc)
	if dc == 0 {
		return 0, 0, 0, false
	}
	var bits uintptr
	bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{
		BiSize:  uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth: tw, BiHeight: -th,
		BiPlanes: 1, BiBitCount: 32, BiCompression: BI_RGB,
	}}
	bmp, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		deleteDC.Call(dc)
		return 0, 0, 0, false
	}
	old, _, _ := selectObject.Call(dc, bmp)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(bits)), int(tw*th*4)), data)
	return dc, bmp, old, true
}

func ensureHUDTextureResources(hdc uintptr) bool {
	if hudTopTextureDC == 0 || hudTopTextureBmp == 0 {
		dc, bmp, old, ok := createHUDTextureResource(hdc, hudNetworkTopBGRA, 1536, 139)
		if !ok {
			return false
		}
		hudTopTextureDC, hudTopTextureBmp, hudTopTextureOld = dc, bmp, old
	}
	if hudBottomTextureDC == 0 || hudBottomTextureBmp == 0 {
		dc, bmp, old, ok := createHUDTextureResource(hdc, hudNetworkBottomBGRA, 1536, 243)
		if !ok {
			return false
		}
		hudBottomTextureDC, hudBottomTextureBmp, hudBottomTextureOld = dc, bmp, old
	}
	return true
}

func releaseHUDTextureResources() {
	if hudTopTextureDC != 0 {
		if hudTopTextureOld != 0 {
			selectObject.Call(hudTopTextureDC, hudTopTextureOld)
		}
		if hudTopTextureBmp != 0 {
			deleteObject.Call(hudTopTextureBmp)
		}
		deleteDC.Call(hudTopTextureDC)
	}
	hudTopTextureDC, hudTopTextureBmp, hudTopTextureOld = 0, 0, 0

	if hudBottomTextureDC != 0 {
		if hudBottomTextureOld != 0 {
			selectObject.Call(hudBottomTextureDC, hudBottomTextureOld)
		}
		if hudBottomTextureBmp != 0 {
			deleteObject.Call(hudBottomTextureBmp)
		}
		deleteDC.Call(hudBottomTextureDC)
	}
	hudBottomTextureDC, hudBottomTextureBmp, hudBottomTextureOld = 0, 0, 0
}

func drawHUDTopTexture(hdc uintptr, r RECT, alpha byte) {
	if r.Right <= r.Left || r.Bottom <= r.Top || alpha == 0 {
		return
	}
	if !ensureHUDTextureResources(hdc) {
		return
	}
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(alpha)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(
		hdc,
		uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top),
		hudTopTextureDC, 0, 0, 1536, 139, blend,
	)
}

func drawHUDBottomTexture(hdc uintptr, r RECT, alpha byte) {
	if r.Right <= r.Left || r.Bottom <= r.Top || alpha == 0 {
		return
	}
	if !ensureHUDTextureResources(hdc) {
		return
	}
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(alpha)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(
		hdc,
		uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top),
		hudBottomTextureDC, 0, 0, 1536, 243, blend,
	)
}

func drawHUDIcon(hdc uintptr, index int, r RECT) {
	if index < 0 || index >= 5 || r.Right <= r.Left || r.Bottom <= r.Top {
		return
	}
	if !ensureHUDIconResources(hdc) {
		return
	}
	const iconSize = 64
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(
		hdc,
		uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top),
		hudIconDCs[index], 0, 0, iconSize, iconSize, blend,
	)
}

func drawHUDCardTitle(hdc uintptr, tag RECT, index int, label string) {
	if hudTinyFont == 0 {
		return
	}
	iconSize := tag.Bottom - tag.Top - 6
	if iconSize < 14 {
		iconSize = 14
	}
	if iconSize > 20 {
		iconSize = 20
	}

	old, _, _ := selectObject.Call(hdc, hudTinyFont)
	defer selectObject.Call(hdc, old)
	setBkMode.Call(hdc, TRANSPARENT)
	setTextColor.Call(hdc, rgb(4, 20, 49))

	textSize := textPixelSize(hdc, hudTinyFont, label)
	gap := int32(5)
	total := iconSize + gap + textSize.Cx
	startX := tag.Left + (tag.Right-tag.Left-total)/2
	iconY := tag.Top + (tag.Bottom-tag.Top-iconSize)/2
	drawHUDIcon(hdc, index, RECT{startX, iconY, startX + iconSize, iconY + iconSize})

	textY := verticallyCenteredTextY(hdc, hudTinyFont, tag, label)
	textOut(hdc, startX+iconSize+gap, textY, label)
}

func drawUIBase(hdc uintptr, w, hgt int32) {
	setBkMode.Call(hdc, TRANSPARENT)
	ink := rgb(3, 9, 24)
	deep := rgb(2, 17, 48)
	cyan := rgb(45, 214, 255)
	orange := rgb(255, 145, 15)
	silver := rgb(236, 244, 250)

	fillSolidRect(hdc, RECT{0, 0, w, hgt}, deep)
	fillSolidRect(hdc, RECT{0, 0, w, sy(164, hgt)}, rgb(4, 38, 96))
	fillSolidRect(hdc, RECT{0, sy(139, hgt), w, sy(164, hgt)}, ink)
	drawLineSimple(hdc, 0, sy(139, hgt), w, sy(139, hgt), 2, cyan)
	drawLineSimple(hdc, 0, sy(159, hgt), w, sy(159, hgt), 2, orange)

	// v178: subtle circuit-board texture across the top blue HUD only.
	drawHUDTopTexture(hdc, RECT{0, 0, w, sy(139, hgt)}, 34)

	// Survival critical alarm: at 1 HP the HUD backgrounds pulse red while
	// preserving card/text readability above the tint.
	if survivalActive() && !survivalBoss1Active() && !survivalBoss2Active() && !survivalBoss3Active() && survivalHP == 1 && state == StatePlaying {
		alarm := 0.5 + 0.5*math.Sin(float64(time.Now().UnixNano())/1e9*9.5)
		if alarm > .28 {
			alpha := byte(28 + alarm*92)
			alphaSolidRect(hdc, RECT{0, 0, w, sy(139, hgt)}, rgb(220, 0, 18), alpha)
		}
	}

	drawCursorControlHUDImage(hdc, RECT{sx(2, w), sy(5, hgt), sx(255, w), sy(137, hgt)})
	drawEnduranceModeButton(hdc, w, hgt)

	ar := arenaRect(w, hgt)
	fillSolidRect(hdc, ar, rgb(242, 248, 252))
	drawPlayfieldPattern(hdc, w, hgt)

	fillSolidRect(hdc, RECT{0, ar.Bottom + 3, w, hgt}, rgb(2, 23, 61))
	drawLineSimple(hdc, 0, ar.Bottom+3, w, ar.Bottom+3, 4, ink)
	drawLineSimple(hdc, 0, ar.Bottom+8, w, ar.Bottom+8, 2, cyan)

	// v178: same subtle circuit-board texture across the bottom blue HUD only.
	drawHUDBottomTexture(hdc, RECT{0, ar.Bottom + 9, w, hgt}, 34)
	if survivalActive() && !survivalBoss1Active() && !survivalBoss2Active() && !survivalBoss3Active() && survivalHP == 1 && state == StatePlaying {
		alarm := 0.5 + 0.5*math.Sin(float64(time.Now().UnixNano())/1e9*9.5)
		if alarm > .28 {
			alpha := byte(32 + alarm*105)
			alphaSolidRect(hdc, RECT{0, ar.Bottom + 9, w, hgt}, rgb(225, 0, 20), alpha)
		}
	}

	type hudCard struct {
		r      RECT
		label  string
		accent uintptr
		icon   int
	}
	// Bottom edge is 124, leaving a clean 15 design-pixel gap before the cyan divider at 139.
	thirdHUDLabel := "STREAK"
	firstHUDLabel := "TIME"
	fourthHUDLabel := "BEST"
	fifthHUDLabel := "DIFFICULTY"
	if enduranceActive() {
		thirdHUDLabel = "DISTANCE"
	}
	if survivalActive() {
		firstHUDLabel = "WAVE"
		thirdHUDLabel = "STATION HP"
		fourthHUDLabel = "CHECKPOINT"
		fifthHUDLabel = "MODE"
	}
	cards := []hudCard{
		{RECT{sx(585, w), sy(7, hgt), sx(755, w), sy(124, hgt)}, firstHUDLabel, rgb(56, 218, 255), 0},
		{RECT{sx(770, w), sy(7, hgt), sx(940, w), sy(124, hgt)}, "SCORE", rgb(84, 173, 255), 1},
		{RECT{sx(955, w), sy(7, hgt), sx(1125, w), sy(124, hgt)}, thirdHUDLabel, rgb(255, 74, 68), 2},
		{RECT{sx(1140, w), sy(7, hgt), sx(1320, w), sy(124, hgt)}, fourthHUDLabel, rgb(255, 169, 24), 3},
		{RECT{sx(1335, w), sy(7, hgt), sx(1513, w), sy(124, hgt)}, fifthHUDLabel, rgb(51, 214, 255), 4},
	}
	for _, c := range cards {
		drawBevelPanel(hdc, c.r, rgb(6, 54, 125), c.accent, rgb(2, 27, 68), 4)
		tag := RECT{c.r.Left + 14, c.r.Top + 10, c.r.Right - 14, c.r.Top + 35}
		fillSolidRect(hdc, tag, silver)
		drawLineSimple(hdc, tag.Left, tag.Bottom, tag.Right, tag.Bottom, 2, c.accent)
		drawHUDCardTitle(hdc, tag, c.icon, c.label)
		drawLineSimple(hdc, c.r.Left+10, c.r.Top+45, c.r.Left+10, c.r.Bottom-9, 3, c.accent)
	}
}

func arenaRectForInvalidate(h uintptr) RECT {
	w, hgt := getClient(h)
	return arenaRect(w, hgt)
}

func arenaRect(w, h int32) RECT {
	return RECT{sx(14, w), sy(180, h), sx(1522, w), sy(772, h)}
}

func pointInArena(p FPoint, w, h int32) bool {
	r := arenaRect(w, h)
	return p.X >= float64(r.Left) && p.X <= float64(r.Right) &&
		p.Y >= float64(r.Top) && p.Y <= float64(r.Bottom)
}

var courseNames = []string{"RANDOM", "SMOOTH", "PRECISION", "FLICK", "TRACKING", "CHAOS"}
var movingNames = []string{"AUTO", "ON", "OFF"}

func rankForScore(v int) string {
	// Legacy wrapper now maps to EXP-based progression.
	// New gameplay code calls rankForEXP directly.
	return rankForEXP(v)
}

func recordAccuracySegment(from, to FPoint) {
	if state != StatePlaying || len(path) < 2 {
		return
	}
	d := dist(from, to)
	steps := int(math.Ceil(d / 3.0))
	if steps < 1 {
		steps = 1
	}
	allowed := allowedTrackDistance()

	if enduranceActive() {
		for i := 1; i <= steps; i++ {
			f := float64(i) / float64(steps)
			q := FPoint{from.X + (to.X-from.X)*f, from.Y + (to.Y-from.Y)*f}
			pd := distanceToEndurancePathFast(q)
			a := 1.0 - pd/allowed
			if a < 0 {
				a = 0
			}
			if a > 1 {
				a = 1
			}
			trackAccuracySum += a
			trackAccuracySamples++
		}
		return
	}

	local := progressIndex
	for i := 1; i <= steps; i++ {
		f := float64(i) / float64(steps)
		q := FPoint{from.X + (to.X-from.X)*f, from.Y + (to.Y-from.Y)*f}
		idx := nearestPathIndexLocal(q, local)
		if idx > local {
			local = idx
		}
		pd := distanceToPathLocal(q, idx)
		a := 1.0 - pd/allowed
		if a < 0 {
			a = 0
		}
		if a > 1 {
			a = 1
		}
		trackAccuracySum += a
		trackAccuracySamples++
	}
}

func adjustAdaptiveSuccess(acc float64, elapsed float64) {
	if !adaptiveMode {
		return
	}

	if acc >= 90 && elapsed <= 8.5 {
		adaptiveMeter++
	} else if acc < 75 {
		adaptiveMeter--
	}

	if adaptiveMeter >= 2 && adaptiveTier < 3 {
		next := adaptiveTier + 1
		if difficultyUnlocked(next) {
			adaptiveTier = next
			adaptiveMeter = 0
		} else {
			// Hold at the highest unlocked tier instead of bypassing progression.
			adaptiveMeter = 0
		}
	}

	if adaptiveMeter <= -2 && adaptiveTier > 0 {
		adaptiveTier--
		adaptiveMeter = 0
	}
}

func adjustAdaptiveFail() {
	if !adaptiveMode {
		return
	}
	adaptiveMeter--
	if adaptiveMeter <= -2 && adaptiveTier > 0 {
		adaptiveTier--
		adaptiveMeter = 0
	}
}

func enduranceModeButtonRect(w, h int32) RECT {
	if !hudLayoutLoaded {
		hudLayoutConfig = defaultHUDLayoutConfig()
		hudLayoutLoaded = true
	}
	return designToScreenRect(hudLayoutConfig.ModeSwitch, w, h)
}

func modeSelectorCardRects(w, h int32) [4]RECT {
	anchor := enduranceModeButtonRect(w, h)
	cardW := sx(410, w)
	cardH := sy(78, h)
	gap := sy(8, h)
	left := anchor.Left
	top := anchor.Bottom + sy(7, h)
	return [4]RECT{
		{left, top, left + cardW, top + cardH},
		{left, top + cardH + gap, left + cardW, top + cardH*2 + gap},
		{left, top + (cardH+gap)*2, left + cardW, top + cardH*3 + gap*2},
		{left, top + (cardH+gap)*3, left + cardW, top + cardH*4 + gap*3},
	}
}

func modeSelectorPanelRect(w, h int32) RECT {
	cards := modeSelectorCardRects(w, h)
	padX := sx(16, w)
	padTop := sy(14, h)
	padBottom := sy(16, h)
	return RECT{cards[0].Left - padX, cards[0].Top - padTop, cards[3].Right + padX, cards[3].Bottom + padBottom}
}

func drawEnduranceModeButton(hdc uintptr, w, hgt int32) {
	r := enduranceModeButtonRect(w, hgt)
	if len(selectModeButtonBGRA) >= 775*181*4 {
		drawRawBGRATrimmedFit(hdc, selectModeButtonBGRA, 775, 181, r)
	} else {
		drawBevelPanel(hdc, r, rgb(7, 63, 139), rgb(48, 216, 255), rgb(2, 25, 65), 4)
		drawArcadeLabel(hdc, r, "SELECT MODE", rgb(244, 251, 255))
	}
	if pointInRect(cursorPos, r) {
		drawOutlineRect(hdc, r, rgb(78, 225, 255), 2)
	}
}

func drawModeSelectorDropdown(hdc uintptr, w, hgt int32) {
	if !modeSelectorOpen || (state != StateWaiting && state != StateResult) || overlayMode != OverlayNone {
		return
	}
	rects := modeSelectorCardRects(w, hgt)
	panel := modeSelectorPanelRect(w, hgt)

	// Premium drop-down housing: one coherent frame around the existing mode art.
	// The mode buttons remain unchanged; this simply gives them a deliberate panel,
	// shadow, layered edge and illuminated rails instead of floating over the arena.
	shadow := RECT{panel.Left + sx(7, w), panel.Top + sy(8, hgt), panel.Right + sx(7, w), panel.Bottom + sy(8, hgt)}
	overlaySolidAlphaRect(hdc, shadow, rgb(0, 2, 10), 180)
	drawBevelPanel(hdc, panel, rgb(3, 19, 48), rgb(42, 214, 255), rgb(0, 7, 22), 4)
	inner := RECT{panel.Left + sx(6, w), panel.Top + sy(6, hgt), panel.Right - sx(6, w), panel.Bottom - sy(6, hgt)}
	drawOutlineRect(hdc, inner, rgb(13, 86, 145), 1)
	fillSolidRect(hdc, RECT{panel.Left + sx(14, w), panel.Top + sy(7, hgt), panel.Right - sx(14, w), panel.Top + sy(10, hgt)}, rgb(50, 225, 255))
	fillSolidRect(hdc, RECT{panel.Left + sx(14, w), panel.Bottom - sy(10, hgt), panel.Right - sx(14, w), panel.Bottom - sy(8, hgt)}, rgb(20, 99, 189))
	// Small corner hardware ties the container into the rest of Cursor Control's HUD.
	for _, x := range []int32{panel.Left + sx(8, w), panel.Right - sx(14, w)} {
		fillSolidRect(hdc, RECT{x, panel.Top + sy(16, hgt), x + sx(6, w), panel.Top + sy(34, hgt)}, rgb(31, 155, 216))
		fillSolidRect(hdc, RECT{x, panel.Bottom - sy(34, hgt), x + sx(6, w), panel.Bottom - sy(16, hgt)}, rgb(31, 155, 216))
	}

	assets := [][]byte{modePrecisionCardBGRA, modeEnduranceCardBGRA, modeSurvivalCardBGRA, modeStarbaseCardBGRA}
	widths := []int32{1368, 1368, 1371, 1368}
	heights := []int32{267, 259, 260, 260}
	for i, r := range rects {
		overlaySolidAlphaRect(hdc, RECT{r.Left - 3, r.Top - 3, r.Right + 3, r.Bottom + 3}, rgb(1, 8, 24), 225)
		if len(assets[i]) >= int(widths[i]*heights[i]*4) {
			drawRawBGRATrimmedFit(hdc, assets[i], widths[i], heights[i], r)
		}
		lockedStarbase := false
		if i == 3 {
			authMu.Lock()
			lockedStarbase = !discordConnected
			authMu.Unlock()
			if lockedStarbase {
				// Deliberately simple hardware-padlock silhouette so the lock remains crisp
				// with every UI font and Windows DPI setting.
				overlaySolidAlphaRect(hdc, r, rgb(0, 3, 12), 170)
				cx := (r.Left + r.Right) / 2
				cy := (r.Top + r.Bottom) / 2
				lockW := sx(34, w)
				lockH := sy(26, hgt)
				body := RECT{cx - lockW/2, cy - sy(9, hgt), cx + lockW/2, cy - sy(9, hgt) + lockH}
				fillSolidRect(hdc, body, rgb(10, 28, 55))
				drawOutlineRect(hdc, body, rgb(255, 205, 74), 2)
				drawSurvivalCircleWithPen(hdc, survivalPen(3, rgb(255, 205, 74)), cx, body.Top, sx(14, w))
				old, _, _ := selectObject.Call(hdc, hudSmallFont)
				setTextColor.Call(hdc, uintptr(rgb(255, 224, 120)))
				setBkMode.Call(hdc, TRANSPARENT)
				centeredTextOut(hdc, r.Left+sx(8, w), r.Right-sx(8, w), r.Bottom-sy(25, hgt), "LOCKED // DISCORD LOGIN REQUIRED")
				selectObject.Call(hdc, old)
			}
		}
		if gameMode == i {
			drawOutlineRect(hdc, r, rgb(255, 244, 82), 3)
		} else if pointInRect(cursorPos, r) {
			col := rgb(83, 226, 255)
			if lockedStarbase {
				col = rgb(255, 205, 74)
			}
			drawOutlineRect(hdc, r, col, 2)
		}
	}
}

func selectGameMode(h uintptr, requested int) {
	if requested < 0 || requested > 3 {
		return
	}

	// STARBASE is a management overlay, not a fourth active run simulation. It must
	// therefore be able to take ownership of the front end cleanly regardless of
	// which gameplay mode supplied the waiting/result screen. Survival in particular
	// owns additional HUD/transient state, so normalize that state before opening the
	// overlay instead of merely hiding its quick-access buttons.
	if requested == 3 {
		// v410: Starbase is a Discord/cloud-only mode. Never open the world against an
		// empty local balance while authentication or the authoritative cloud claim is
		// still pending; the player only enters after the server state is loaded.
		if ok, why := afkCloudStarbaseAccess(time.Now()); !ok {
			showSpaceCacheWarning(h, why)
			return
		}
		// Starbase is always reachable from Survival. If a Survival run is active,
		// choosing Starbase deliberately abandons that transient run and returns the
		// simulation to its waiting state before the Starbase overlay takes ownership.
		// Other active modes retain their existing protection against accidental exits.
		if state == StatePlaying {
			if !survivalActive() {
				return
			}
			resetToWaiting(h)
		}
		if overlayMode != OverlayNone {
			closeOverlay()
		}
		if survivalActive() {
			resetSurvivalRun()
		}
		if state != StateWaiting {
			resetToWaiting(h)
		}
		modeSelectorOpen = false
		menuOpen = false
		setOverlay(OverlayAFKSingularity)
		analyticsEvent("mode_selected", map[string]any{"mode": "STARBASE"})
		invalidateRect.Call(h, 0, 0)
		return
	}

	if state != StateWaiting {
		return
	}
	// v356 QoL: mode selection is a global escape hatch from front-end tabs.
	// Choosing a mode always closes any open overlay/menu first so the player
	// lands directly back on that mode's waiting screen.
	if overlayMode != OverlayNone {
		closeOverlay()
	}
	modeSelectorOpen = false
	menuOpen = false
	if requested == gameMode {
		resetToWaiting(h)
		invalidateRect.Call(h, 0, 0)
		return
	}
	clearEnduranceTransientObjects()
	resetAllStats()
	hasPreviousEnd = false
	gameMode = requested
	adaptiveMode = false
	pathMode = 0
	if survivalActive() {
		resetSurvivalRun()
	}
	switchModeMusic()
	resetToWaiting(h)
	runtimeLifecycleSnapshot("mode_select_complete")
	analyticsEvent("mode_selected", map[string]any{"mode": analyticsModeName()})
}

// Legacy developer/internal call: cycle remains available internally, but the player HUD
// now uses direct selection rather than cycling through three modes.
func toggleEnduranceMode(h uintptr) {
	selectGameMode(h, (gameMode+1)%3)
}

func menuButtonRect(w, h int32) RECT {
	ar := arenaRect(w, h)
	bar := announcementBarRect(w, h)
	// Keep MENU directly beneath and aligned with the GLOBAL LIVE tag.
	return RECT{
		bar.Left,
		ar.Top + sy(5, h),
		bar.Left + sx(138, w),
		ar.Top + sy(48, h),
	}
}

func menuPanelRect(w, h int32) RECT {
	ar := arenaRect(w, h)
	return RECT{
		ar.Left + sx(5, w),
		ar.Top + sy(55, h),
		ar.Left + sx(463, w),
		ar.Bottom - sy(8, h),
	}
}

func pointInRect(p FPoint, r RECT) bool {
	return p.X >= float64(r.Left) && p.X <= float64(r.Right) &&
		p.Y >= float64(r.Top) && p.Y <= float64(r.Bottom)
}

func pointInMenuUI(p FPoint, w, h int32) bool {
	if pointInRect(p, enduranceModeButtonRect(w, h)) {
		return true
	}
	if modeSelectorOpen && (state == StateWaiting || state == StateResult) && overlayMode == OverlayNone {
		for _, r := range modeSelectorCardRects(w, h) {
			if pointInRect(p, r) {
				return true
			}
		}
	}
	if state == StateWaiting && overlayMode == OverlayNone {
		for _, r := range quickAccessRects(w, h) {
			if pointInRect(p, r) {
				return true
			}
		}
	}
	return false
}

func menuLayout(w, h int32) (RECT, int32, [8]int32, [4]int32) {
	r := menuPanelRect(w, h)
	rowH := sy(38, h)
	if rowH < 30 {
		rowH = 30
	}
	gap := sy(5, h)
	if gap < 3 {
		gap = 3
	}
	headerGap := sy(22, h)
	if headerGap < 18 {
		headerGap = 18
	}
	sectionGap := sy(12, h)
	if sectionGap < 8 {
		sectionGap = 8
	}

	section1 := r.Top + sy(48, h)
	row0 := section1 + headerGap
	row1 := row0 + rowH + gap

	section2 := row1 + rowH + sectionGap
	row2 := section2 + headerGap
	row3 := row2 + rowH + gap

	section3 := row3 + rowH + sectionGap
	row4 := section3 + headerGap
	row5 := row4 + rowH + gap

	return r, rowH,
		[8]int32{row0, row1, row2, row3, row4, row5, -9999, -9999},
		[4]int32{section1, section2, section3, -9999}
}

func menuRowAt(p FPoint, w, h int32) int {
	r, rowH, rows, _ := menuLayout(w, h)
	if !pointInRect(p, r) {
		return -1
	}
	for i, y := range rows {
		if int32(p.Y) >= y && int32(p.Y) < y+rowH {
			return i
		}
	}
	return -1
}

func confirmGameplaySettingReset(h uintptr, titleText string) bool {
	if score <= 0 && streak <= 0 && bestScore <= 0 && bestStreak <= 0 {
		return true
	}
	msg := utf16ptr("Changing this gameplay setting will reset your stats. Continue?")
	title := utf16ptr(titleText)
	r, _, _ := messageBoxW.Call(
		h,
		uintptr(unsafe.Pointer(msg)),
		uintptr(unsafe.Pointer(title)),
		MB_YESNO|MB_ICONWARNING,
	)
	return r == IDYES
}

func openLeaderboard() {
	loadLeaderboard()
	menuOpen = false
	localLeaderboardScroll = 0
	if enduranceActive() || (state == StateResult && lastResult.Course == "ENDURANCE") {
		localLeaderboardFilter = 5
	}
	// Always go through the shared overlay transition path so opening Local
	// Leaderboard invalidates immediately and never depends on WM_MOUSEMOVE.
	setOverlay(OverlayLeaderboard)
}

func resetToWaiting(h uintptr) {
	clearPolishResult()
	stopTransientGameplayAudio()
	if !enduranceActive() {
		clearEnduranceTransientObjects()
	}
	runtimeLifecycleSnapshot("reset_to_waiting")
	killTimer.Call(h, TIMER_GAME)
	killTimer.Call(h, TIMER_FAIL_RESET)
	killTimer.Call(h, TIMER_RESULT_RESET)
	releaseCapture.Call()
	resumeBossAfterFailure()

	if len(path) >= 2 {
		previousEnd = path[len(path)-1]
		hasPreviousEnd = true
	}

	state = StateWaiting
	overlayMode = OverlayNone
	menuOpen = false
	timeBonus = 0
	if enduranceActive() || survivalActive() {
		lastTime = 0
	} else {
		lastTime = 10
	}
	hasLastMouse = false
	progressIndex = 0
	if survivalActive() {
		status = "CLICK THE SPACE STATION TO BEGIN"
		resetSurvivalRun()
	} else {
		status = "Click START to begin"
	}
	generatePath(h)
	invalidateRect.Call(h, 0, 0)
}

func targetCountForStreak(s int) int {
	n := 1 + s/5

	// Insane begins immediately with three required targets.
	if activeDifficultyIndex() == 3 && n < 3 {
		n = 3
	}

	if n > 4 {
		n = 4
	}
	if n < 1 {
		n = 1
	}
	return n
}

func pathChallengeMetrics(p []FPoint, top, bottom float64) (lengthRatio, verticalCoverage float64, turns int) {
	if len(p) < 3 {
		return 0, 0, 0
	}

	total := 0.0
	for i := 1; i < len(p); i++ {
		total += dist(p[i-1], p[i])
	}

	direct := dist(p[0], p[len(p)-1])
	if direct < 1 {
		direct = 1
	}
	lengthRatio = total / direct

	minY := p[0].Y
	maxY := p[0].Y
	for _, pt := range p {
		if pt.Y < minY {
			minY = pt.Y
		}
		if pt.Y > maxY {
			maxY = pt.Y
		}
	}

	span := bottom - top
	if span < 1 {
		span = 1
	}
	verticalCoverage = (maxY - minY) / span

	stride := 55
	if len(p) < stride*3 {
		stride = 20
	}

	lastSign := 0
	lastY := p[0].Y

	for i := stride; i < len(p); i += stride {
		dy := p[i].Y - lastY
		sign := 0

		if dy > 14 {
			sign = 1
		} else if dy < -14 {
			sign = -1
		}

		if sign != 0 {
			if lastSign != 0 && sign != lastSign {
				turns++
			}
			lastSign = sign
			lastY = p[i].Y
		}
	}

	return
}

func insanePathIsChallenging(p []FPoint, top, bottom float64) bool {
	ratio, coverage, turns := pathChallengeMetrics(p, top, bottom)
	return ratio >= 1.27 && coverage >= 0.40 && turns >= 4
}

func generatePath(h uintptr) {
	if enduranceActive() {
		generateEndurancePath(h)
		return
	}
	w, hgt := getClient(h)
	if w < 700 {
		w = 700
	}
	if hgt < 500 {
		hgt = 500
	}

	di := activeDifficultyIndex()
	ds := diffs[di]
	ar := arenaRect(w, hgt)

	leftBound := float64(ar.Left) + math.Max(58, float64(w)*0.05)
	rightBound := float64(ar.Right) - math.Max(58, float64(w)*0.05)
	topBound := float64(ar.Top) + math.Max(58, float64(hgt)*0.055)
	bottomBound := float64(ar.Bottom) - math.Max(44, float64(hgt)*0.045)

	var start FPoint
	if hasPreviousEnd {
		start = previousEnd
		start.X = math.Max(leftBound, math.Min(rightBound, start.X))
		start.Y = math.Max(topBound, math.Min(bottomBound, start.Y))
	} else {
		start.X = leftBound
		if startSide > 0 {
			start.X = rightBound
		}
		start.Y = randf(topBound, bottomBound)
	}

	currentCourse = effectiveCourse()

	maxAttempts := 1
	if di == 3 {
		maxAttempts = 20
	}

	accepted := false
	chosenEnd := FPoint{}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		end := FPoint{X: rightBound, Y: randf(topBound, bottomBound)}
		if start.X > (leftBound+rightBound)/2 {
			end.X = leftBound
		}
		chosenEnd = end

		controlCount := ds.controls
		wiggle := ds.wiggle
		jitterX := 32.0

		switch currentCourse {
		case 1:
			controlCount = int(math.Max(4, float64(ds.controls-2)))
			wiggle *= 0.52
			jitterX = 18
		case 2:
			controlCount = ds.controls + 2
			wiggle *= 0.78
			jitterX = 20
		case 3:
			controlCount = ds.controls + 1
			wiggle *= 0.92
			jitterX = 14
		case 4:
			controlCount = ds.controls
			wiggle *= 0.68
			jitterX = 12
		case 5:
			controlCount = ds.controls + 3
			wiggle *= 1.18
			jitterX = 42
		}

		if controlCount < 4 {
			controlCount = 4
		}

		// Hard floor for Insane so no archetype can generate a gentle course.
		if di == 3 {
			if controlCount < 12 {
				controlCount = 12
			}
			if wiggle < 170 {
				wiggle = 170
			}
			if jitterX < 20 {
				jitterX = 20
			}
		}

		controls := make([]FPoint, 0, controlCount)
		controls = append(controls, start)

		phase := randf(0, math.Pi*2)

		for i := 1; i < controlCount-1; i++ {
			t := float64(i) / float64(controlCount-1)
			x := start.X + (end.X-start.X)*t + randf(-jitterX, jitterX)
			baseY := start.Y + (end.Y-start.Y)*t
			y := baseY

			switch currentCourse {
			case 3:
				sign := 1.0
				if i%2 == 0 {
					sign = -1
				}
				y += sign*wiggle*0.78 + randf(-wiggle*0.18, wiggle*0.18)

			case 4:
				y += math.Sin(t*math.Pi*4+phase) * wiggle * 0.9

			default:
				y += randf(-wiggle, wiggle)
			}

			if di == 3 {
				// Secondary oscillation prevents random cancellation from
				// flattening several consecutive Insane sections.
				y += math.Sin(t*math.Pi*7+phase) * wiggle * 0.30
			}

			x = math.Max(leftBound, math.Min(rightBound, x))
			y = math.Max(topBound, math.Min(bottomBound, y))
			controls = append(controls, FPoint{x, y})
		}

		controls = append(controls, end)

		candidate := sampleCatmull(controls, 110)
		for i := range candidate {
			candidate[i].X = math.Max(leftBound, math.Min(rightBound, candidate[i].X))
			candidate[i].Y = math.Max(topBound, math.Min(bottomBound, candidate[i].Y))
		}

		path = candidate

		if di != 3 || insanePathIsChallenging(path, topBound, bottomBound) {
			accepted = true
			break
		}
	}

	// Fallback guarantees an Insane course can never accept a flat layout.
	if di == 3 && !accepted {
		controls := []FPoint{start}
		points := 10

		for i := 1; i < points-1; i++ {
			t := float64(i) / float64(points-1)
			x := start.X + (chosenEnd.X-start.X)*t
			y := topBound + (bottomBound-topBound)*0.18

			if i%2 == 0 {
				y = topBound + (bottomBound-topBound)*0.82
			}

			controls = append(controls, FPoint{x, y})
		}

		controls = append(controls, chosenEnd)
		path = sampleCatmull(controls, 110)

		for i := range path {
			path[i].X = math.Max(leftBound, math.Min(rightBound, path[i].X))
			path[i].Y = math.Max(topBound, math.Min(bottomBound, path[i].Y))
		}
	}

	count := targetCountForStreak(streak)
	targets = make([]Target, 0, count)

	baseIndices := make([]int, count)
	usedBase := make([]int, 0, count)

	for i := 0; i < count; i++ {
		segmentStart := int(float64(len(path))*float64(i+1)/float64(count+1)) - 45
		segmentEnd := int(float64(len(path))*float64(i+1)/float64(count+1)) + 45

		if segmentStart < 20 {
			segmentStart = 20
		}
		if segmentEnd > len(path)-21 {
			segmentEnd = len(path) - 21
		}

		desired := (segmentStart+segmentEnd)/2 + rand.Intn(25) - 12
		idx := findSeparatedTargetIndex(desired, segmentStart, segmentEnd, usedBase, 62.0)

		baseIndices[i] = idx
		usedBase = append(usedBase, idx)
	}

	minPathSeparation := 42

	for i, base := range baseIndices {
		leftLimit := 20
		rightLimit := len(path) - 21

		if i > 0 {
			mid := (baseIndices[i-1] + base) / 2
			leftLimit = mid + minPathSeparation/2
		}

		if i+1 < count {
			mid := (base + baseIndices[i+1]) / 2
			rightLimit = mid - minPathSeparation/2
		}

		if leftLimit > base {
			leftLimit = base
		}
		if rightLimit < base {
			rightLimit = base
		}

		leftRoom := base - leftLimit
		rightRoom := rightLimit - base
		maxMove := leftRoom

		if rightRoom < maxMove {
			maxMove = rightRoom
		}

		moveRange := 20
		if maxMove < moveRange {
			moveRange = maxMove
		}
		if moveRange < 0 {
			moveRange = 0
		}

		targets = append(targets, Target{
			Point:     path[base],
			Index:     base,
			MoveRange: moveRange,
			MinIndex:  base - moveRange,
			MaxIndex:  base + moveRange,
			Phase:     randf(0, math.Pi*2),
			Speed:     randf(0.8, 1.25),
		})
	}
}

func sampleCatmull(pts []FPoint, perSeg int) []FPoint {
	if len(pts) < 2 {
		return nil
	}
	out := make([]FPoint, 0, (len(pts)-1)*perSeg+1)
	for i := 0; i < len(pts)-1; i++ {
		p0 := pts[i]
		if i > 0 {
			p0 = pts[i-1]
		}
		p1 := pts[i]
		p2 := pts[i+1]
		p3 := p2
		if i+2 < len(pts) {
			p3 = pts[i+2]
		}
		for j := 0; j < perSeg; j++ {
			t := float64(j) / float64(perSeg)
			t2 := t * t
			t3 := t2 * t
			x := 0.5 * ((2 * p1.X) + (-p0.X+p2.X)*t + (2*p0.X-5*p1.X+4*p2.X-p3.X)*t2 + (-p0.X+3*p1.X-3*p2.X+p3.X)*t3)
			y := 0.5 * ((2 * p1.Y) + (-p0.Y+p2.Y)*t + (2*p0.Y-5*p1.Y+4*p2.Y-p3.Y)*t2 + (-p0.Y+3*p1.Y-3*p2.Y+p3.Y)*t3)
			out = append(out, FPoint{x, y})
		}
	}
	return append(out, pts[len(pts)-1])
}

func pointSegDist(p, a, b FPoint) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	den := dx*dx + dy*dy
	if den == 0 {
		return dist(p, a)
	}
	t := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / den
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	q := FPoint{a.X + t*dx, a.Y + t*dy}
	return dist(p, q)
}

func nearestPathIndexLocal(p FPoint, center int) int {
	if len(path) == 0 {
		return 0
	}
	if center < 0 {
		center = 0
	}
	if center >= len(path) {
		center = len(path) - 1
	}
	lo := center - 110
	hi := center + 170
	if lo < 0 {
		lo = 0
	}
	if hi >= len(path) {
		hi = len(path) - 1
	}
	best := math.MaxFloat64
	idx := center
	for i := lo; i <= hi; i++ {
		d := dist(p, path[i])
		if d < best {
			best = d
			idx = i
		}
	}
	return idx
}

func distanceToEndurancePathFast(p FPoint) float64 {
	if len(path) < 2 {
		return math.MaxFloat64
	}
	if enduranceActive() {
		p.X += enduranceCameraX
	}

	const spacing = 9.0
	guess := int(math.Floor((p.X - path[0].X) / spacing))
	if guess < 0 {
		guess = 0
	}
	if guess > len(path)-2 {
		guess = len(path) - 2
	}

	start := guess - 3
	if start < 0 {
		start = 0
	}
	end := guess + 3
	if end > len(path)-2 {
		end = len(path) - 2
	}

	best := math.MaxFloat64
	for i := start; i <= end; i++ {
		if d := pointSegDist(p, path[i], path[i+1]); d < best {
			best = d
		}
	}
	return best
}

func distanceToEntirePath(p FPoint) float64 {
	if len(path) < 2 {
		return math.MaxFloat64
	}
	best := math.MaxFloat64
	for i := 0; i < len(path)-1; i++ {
		d := pointSegDist(p, path[i], path[i+1])
		if d < best {
			best = d
		}
	}
	return best
}

func validateEnduranceMovement(from, to FPoint) bool {
	d := dist(from, to)
	steps := int(math.Ceil(d / 2.0))
	if steps < 1 {
		steps = 1
	}
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		q := FPoint{
			X: from.X + (to.X-from.X)*t,
			Y: from.Y + (to.Y-from.Y)*t,
		}
		if !enduranceShipInsideTrack(q) {
			target := currentRequiredTarget()
			if target >= 0 && dist(q, targetCurrentPoint(target)) <= 14 {
				continue
			}
			return false
		}
	}
	return true
}

func distanceToPathLocal(p FPoint, center int) float64 {
	if len(path) < 2 {
		return math.MaxFloat64
	}
	lo := center - 100
	hi := center + 100
	if lo < 0 {
		lo = 0
	}
	if hi > len(path)-2 {
		hi = len(path) - 2
	}
	best := math.MaxFloat64
	for i := lo; i <= hi; i++ {
		if d := pointSegDist(p, path[i], path[i+1]); d < best {
			best = d
		}
	}
	return best
}

func currentRequiredTarget() int {
	for i := range targets {
		if !targets[i].Clicked {
			return i
		}
	}
	return -1
}

const (
	enduranceShipHitboxHalfW = 9.0 // body only; fins/wings remain cosmetic
	enduranceShipHitboxHalfH = 5.0
)

func enduranceShipHitboxPointsAt(p FPoint) [9]FPoint {
	hw, hh := enduranceShipHitboxHalfW, enduranceShipHitboxHalfH
	return [9]FPoint{
		p,
		{X: p.X - hw, Y: p.Y - hh}, {X: p.X, Y: p.Y - hh}, {X: p.X + hw, Y: p.Y - hh},
		{X: p.X - hw, Y: p.Y}, {X: p.X + hw, Y: p.Y},
		{X: p.X - hw, Y: p.Y + hh}, {X: p.X, Y: p.Y + hh}, {X: p.X + hw, Y: p.Y + hh},
	}
}

func enduranceShipInsideTrack(p FPoint) bool {
	limit := allowedTrackDistance()
	for _, q := range enduranceShipHitboxPointsAt(p) {
		if distanceToEndurancePathFast(q) > limit {
			return false
		}
	}
	return true
}

func enduranceShipRectIntersects(p FPoint, left, top, right, bottom float64) bool {
	return p.X+enduranceShipHitboxHalfW >= left && p.X-enduranceShipHitboxHalfW <= right &&
		p.Y+enduranceShipHitboxHalfH >= top && p.Y-enduranceShipHitboxHalfH <= bottom
}

func d2dDrawShipHitbox(x, y float32) {
	if !gameMeta.ShowShipHitbox || d2dRenderTarget == 0 {
		return
	}
	l := x - float32(enduranceShipHitboxHalfW)
	r := x + float32(enduranceShipHitboxHalfW)
	t := y - float32(enduranceShipHitboxHalfH)
	b := y + float32(enduranceShipHitboxHalfH)
	if d2dShipHitboxGlowBrush != 0 {
		d2dFillRect(D2D1RectF{Left: l - 1, Top: t - 1, Right: r + 1, Bottom: b + 1}, d2dShipHitboxGlowBrush)
	}
	br := d2dShipHitboxBrush
	if br == 0 {
		br = d2dShieldCoreBrush
	}
	d2dFillRect(D2D1RectF{Left: l, Top: t, Right: r, Bottom: t + 1}, br)
	d2dFillRect(D2D1RectF{Left: l, Top: b - 1, Right: r, Bottom: b}, br)
	d2dFillRect(D2D1RectF{Left: l, Top: t, Right: l + 1, Bottom: b}, br)
	d2dFillRect(D2D1RectF{Left: r - 1, Top: t, Right: r, Bottom: b}, br)
}

func drawGDIShipHitbox(hdc uintptr, x, y int32) {
	if !gameMeta.ShowShipHitbox {
		return
	}
	hw, hh := int32(enduranceShipHitboxHalfW), int32(enduranceShipHitboxHalfH)
	c := rgb(62, 255, 110)
	drawLineSimple(hdc, x-hw, y-hh, x+hw, y-hh, 1, c)
	drawLineSimple(hdc, x+hw, y-hh, x+hw, y+hh, 1, c)
	drawLineSimple(hdc, x+hw, y+hh, x-hw, y+hh, 1, c)
	drawLineSimple(hdc, x-hw, y+hh, x-hw, y-hh, 1, c)
}

func allowedTrackDistance() float64 {
	if enduranceActive() {
		base := enduranceTrackWidth()
		// Match the visible structural side boundary drawn by drawPath.
		return (base + 19.0) / 2.0
	}
	return diffs[activeDifficultyIndex()].width/2 + 4
}

func developerBoundaryGraceActive() bool {
	return isDeveloperOwner() && !developerBoundaryGraceUntil.IsZero() && time.Now().Before(developerBoundaryGraceUntil)
}

func pointIsValid(p FPoint, center int) (bool, int) {
	idx := nearestPathIndexLocal(p, center)
	if distanceToPathLocal(p, idx) <= allowedTrackDistance() {
		return true, idx
	}
	// The current target itself is a valid aiming zone, so narrow tracks stay playable.
	t := currentRequiredTarget()
	if t >= 0 && dist(p, targetCurrentPoint(t)) <= 14 {
		return true, targetCurrentIndex(t)
	}
	return false, idx
}

func validateMovement(from, to FPoint) (bool, int) {
	d := dist(from, to)
	steps := int(math.Ceil(d / 2.5))
	if steps < 1 {
		steps = 1
	}
	progress := progressIndex
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		q := FPoint{from.X + (to.X-from.X)*t, from.Y + (to.Y-from.Y)*t}
		ok, idx := pointIsValid(q, progress)
		if !ok {
			return false, progress
		}
		if idx > progress {
			progress = idx
		}
	}
	return true, progress
}

func updateRealTimeClock() float64 {
	if state != StatePlaying || !developerPauseStarted.IsZero() {
		return lastTime
	}
	if enduranceActive() {
		lastTime = time.Since(startTime).Seconds()
		return lastTime
	}
	lastTime = 10 + timeBonus - time.Since(startTime).Seconds()
	if lastTime < 0 {
		lastTime = 0
	}
	return lastTime
}

func startGame(h uintptr) {
	if state != StateWaiting || len(path) < 2 {
		return
	}
	state = StatePlaying
	stopTransientGameplayAudio()
	resetPerfForRun()
	runtimeLifecycleSnapshot("run_start")
	enduranceFailureVisualAt = time.Time{}
	gameMeta.TotalRuns++
	if !enduranceActive() && !survivalActive() {
		diffName, _ := difficultyDisplay()
		gameMeta.PrecisionCompetitionActiveDifficulty = canonicalOnlineDifficulty(diffName)
	}
	saveGameMeta()
	analyticsEvent("run_started", map[string]any{"mode": analyticsModeName(), "difficulty": analyticsDifficultyName()})
	startTime = time.Now()
	timeBonus = 0
	if enduranceActive() {
		gameMeta.EnduranceRuns++
		evaluateEnduranceAchievements()
		clearEnduranceTransientObjects()
		resetPolishSystems()
		polishLastSector = 0
		endurancePostEncounterRecoveryUntil = 0
		lastTime = 0
		enduranceDistance = 0
		enduranceCameraX = 0
		resetEndurancePowerups()
		enduranceBlocks = enduranceBlocks[:0]
		enduranceBlockSpawnTime = time.Time{}
		enduranceWarpCheckpoints = []float64{enduranceWarpFirstMeters}
		enduranceNextWarpAt = enduranceWarpFirstMeters
		enduranceWarpCueActive = false
		enduranceWarpActive = false
		enduranceWarpCueStarted = time.Time{}
		enduranceWarpStartDistance = 0
		stopWarpSounds()
		enduranceBonusScore = 0
		enduranceTargetsHit = 0
		enduranceNextTargetAt = 18
		enduranceLastTick = time.Now()
		score = 0
		streak = 0
	} else {
		lastTime = 10
	}
	progressIndex = 0
	trackAccuracySum = 0
	trackAccuracySamples = 0
	targetPrecisionSum = 0
	targetPrecisionHits = 0
	lastHitAt = time.Time{}
	lastMouse = path[0]
	hasLastMouse = true
	if enduranceActive() {
		status = "ENDURANCE — SURVIVE AS LONG AS POSSIBLE"
	} else {
		status = "Click target, then continue"
	}
	setCapture.Call(h)
	if enduranceActive() {
		enduranceLoopAccumulator = 0
		enduranceLastLoopTime = time.Now()
		enduranceLastRenderTime = time.Time{}
		enduranceLastHUDPaint = time.Time{}
		if !d2dReady {
			// Safe software fallback only.
			setTimer.Call(h, TIMER_GAME, 8, 0)
		}
	} else {
		setTimer.Call(h, TIMER_GAME, gameTimerInterval(), 0)
	}
	invalidateRect.Call(h, 0, 0)
}

func loseGame(h uintptr, reason string) {
	if state != StatePlaying {
		return
	}
	// Stop the active Shield loop immediately on failure, before any failure
	// animation, score submission or state transition can run.
	stopTransientGameplayAudio()
	beginPolishFailure()
	if developerGodMode && isDeveloperOwner() {
		status = "GOD MODE BLOCKED FAILURE — " + reason
		invalidateRect.Call(h, 0, 0)
		return
	}
	if enduranceActive() {
		enduranceFailureVisualAt = time.Now()
		enduranceWarpCueActive = false
		enduranceWarpActive = false
		stopWarpSounds()
	}
	killTimer.Call(h, TIMER_GAME)
	releaseCapture.Call()

	if !enduranceActive() {
		adjustAdaptiveFail()
	}
	gameMeta.TotalFailures++

	if enduranceActive() {
		trackingAcc := trackingAccuracyPercent()
		targetAcc := targetAccuracyPercent()
		combined := combinedAccuracyPercent()
		newEndurancePB := enduranceDistance > gameMeta.BestEnduranceDistance
		if newEndurancePB {
			gameMeta.BestEnduranceDistance = enduranceDistance
		}
		if enduranceTargetsHit > gameMeta.EnduranceBestTargets {
			gameMeta.EnduranceBestTargets = enduranceTargetsHit
		}
		evaluateEnduranceAchievements()
		polishEnduranceNewPB = newEndurancePB
		lastResult = ResultData{
			Time:        time.Since(startTime).Seconds(),
			TrackingAcc: trackingAcc,
			TargetAcc:   targetAcc,
			CombinedAcc: combined,
			TargetsHit:  enduranceTargetsHit,
			TargetCount: enduranceTargetSerial,
			RoundPoints: score,
			TotalScore:  score,
			Streak:      enduranceTargetsHit,
			Combo:       1,
			Rating:      ratingForAccuracy(combined),
			Rank:        rankForEXP(playerProgress.EXP),
			EXPEarned:   0,
			TotalEXP:    playerProgress.EXP,
			Course:      "ENDURANCE",
			Difficulty:  "ENDURANCE",
			Distance:    enduranceDistance,
		}
		saveLocalBestAutomatically()
		authMu.Lock()
		onlineConnected := discordConnected
		authMu.Unlock()
		if onlineConnected && !developerGodMode {
			resultCopy := lastResult
			go submitGlobalClear(resultCopy, "ENDURANCE")
		}
	} else {
		// A failed Precision run breaks only this difficulty's Competition streak.
		failedDifficulty := gameMeta.PrecisionCompetitionActiveDifficulty
		if strings.TrimSpace(failedDifficulty) == "" {
			diffName, _ := difficultyDisplay()
			failedDifficulty = canonicalOnlineDifficulty(diffName)
		}
		gameMeta.PrecisionCompetitionActiveDifficulty = ""
		if failedDifficulty != "" {
			go reportPrecisionCompetitionFailure(failedDifficulty)
		}
		// Standard loss ends the current local combo/streak score.
		score = 0
		streak = 0
	}
	saveGameMeta()
	analyticsEvent("run_failed", map[string]any{"mode": analyticsModeName(), "reason": analyticsFailureCategory(reason), "distance": enduranceDistance, "wave": survivalWave})

	state = StateFailed
	hasLastMouse = false
	status = reason
	pauseBossForFailure()

	// TIMER_GAME stops on failure, so the failure reveal needs its own repaint
	// heartbeat. Without this, the PNG/reason only advanced when another Windows
	// message (often WM_MOUSEMOVE) happened to trigger a repaint.
	killTimer.Call(h, TIMER_FAIL_ANIM)
	setTimer.Call(h, TIMER_FAIL_RESET, 2000, 0)
	invalidateRect.Call(h, 0, 0)
	updateWindow.Call(h)
}

func winGame(h uintptr) {
	if state != StatePlaying {
		return
	}
	elapsed := time.Since(startTime).Seconds()
	if elapsed >= 10 {
		lastTime = 0
		loseGame(h, "Time up")
		return
	}
	if currentRequiredTarget() >= 0 {
		loseGame(h, "Target missed")
		return
	}

	killTimer.Call(h, TIMER_GAME)
	releaseCapture.Call()

	trackingAcc := trackingAccuracyPercent()
	targetAcc := targetAccuracyPercent()
	combined := combinedAccuracyPercent()
	remaining := math.Max(0, 10-elapsed)
	nextStreak := streak + 1
	combo := comboForStreak(nextStreak)

	base := 1000
	timeBonus := int(math.Round(remaining * 120))
	trackingBonus := int(math.Round(trackingAcc * 12))
	targetBonus := int(math.Round(targetAcc*5)) + len(targets)*250
	raw := base + timeBonus + trackingBonus + targetBonus
	roundPoints := int(math.Round(float64(raw) * combo))

	score += roundPoints
	streak = nextStreak
	if score > bestScore {
		bestScore = score
	}
	if streak > bestStreak {
		bestStreak = streak
	}

	diffName, _ := difficultyDisplay()
	gameMeta.PrecisionCompetitionActiveDifficulty = ""

	// Persistent progression: every successful puzzle awards EXP according
	// to the difficulty actually played and increments that difficulty's clear count.
	expEarned := expForDifficultyName(diffName)

	oldRankIndex := rankIndexForEXP(playerProgress.EXP)
	oldRankName := rankForEXP(playerProgress.EXP)

	addDifficultyCompletion(diffName)
	playerProgress.EXP += expEarned

	newRankIndex := rankIndexForEXP(playerProgress.EXP)
	newRankName := rankForEXP(playerProgress.EXP)

	// Rank-up celebration triggers only when crossing an EXP rank threshold.
	if newRankIndex > oldRankIndex {
		levelUpAt = time.Now()
		levelUpFrom = oldRankName
		levelUpTo = newRankName
		playLevelUpSound()
		setTimer.Call(h, TIMER_LEVELUP, 33, 0)
	}

	savePlayerProgress()
	analyticsEvent("run_completed", map[string]any{"mode": "PRECISION", "difficulty": diffName, "accuracy": combined, "streak": streak, "exp": expEarned})

	onlineDifficulty := canonicalOnlineDifficulty(diffName)

	lastResult = ResultData{
		Time:        elapsed,
		TrackingAcc: trackingAcc,
		TargetAcc:   targetAcc,
		CombinedAcc: combined,
		TargetsHit:  len(targets),
		TargetCount: len(targets),
		RoundPoints: roundPoints,
		TotalScore:  score,
		Streak:      streak,
		Combo:       combo,
		Rating:      ratingForAccuracy(combined),
		Rank:        rankForEXP(playerProgress.EXP),
		EXPEarned:   expEarned,
		TotalEXP:    playerProgress.EXP,
		Course:      courseNames[currentCourse],
		Difficulty:  diffName,
	}

	// Logged-in players automatically submit every successful clear.
	// The server increments progression and only replaces the PB if this run is better.
	authMu.Lock()
	onlineConnected := discordConnected
	authMu.Unlock()
	if onlineConnected && !developerGodMode {
		resultCopy := lastResult
		go submitGlobalClear(resultCopy, onlineDifficulty)
	}

	adjustAdaptiveSuccess(combined, elapsed)

	gameMeta.TotalClears++
	gameMeta.TargetsHit += len(targets)
	if combined > gameMeta.BestAccuracy {
		gameMeta.BestAccuracy = combined
	}
	if streak > gameMeta.BestStreakEver {
		gameMeta.BestStreakEver = streak
	}
	updateChallengesAfterClear(diffName, combined, streak)
	evaluateAchievements(elapsed, trackingAcc, targetAcc, combined, diffName, streak)
	saveGameMeta()

	saveLocalBestAutomatically()

	state = StateResult
	beginPolishResult(false)
	writeRunPerfSnapshot("precision_result")
	status = "Mission complete"
	killTimer.Call(h, TIMER_RESULT_RESET)
	invalidateRect.Call(h, 0, 0)
}

func resetAllStats() {
	score = 0
	streak = 0
	bestScore = 0
	bestStreak = 0
}

func difficultyUnlocked(index int) bool {
	switch index {
	case 0:
		return true
	case 1:
		return playerProgress.EasyCompleted >= 15
	case 2:
		return playerProgress.NormalCompleted >= 25
	case 3:
		return playerProgress.HardCompleted >= 40
	default:
		return false
	}
}

func difficultyUnlockRequirement(index int) (label string, current, required int) {
	switch index {
	case 1:
		return "EASY CLEARS", playerProgress.EasyCompleted, 15
	case 2:
		return "NORMAL CLEARS", playerProgress.NormalCompleted, 25
	case 3:
		return "HARD CLEARS", playerProgress.HardCompleted, 40
	default:
		return "", 0, 0
	}
}

func difficultyLockedText(index int) string {
	if difficultyUnlocked(index) {
		return ""
	}
	label, current, required := difficultyUnlockRequirement(index)
	return fmt.Sprintf("LOCKED  %d/%d %s", current, required, label)
}

func changeDifficulty(h uintptr, next int) {
	if next < 0 || next >= len(diffs) {
		return
	}
	if !difficultyUnlocked(next) {
		lockedDifficultyPopup = next
		label, current, required := difficultyUnlockRequirement(next)
		status = fmt.Sprintf("%s LOCKED — %d/%d %s", diffs[next].name, current, required, label)
		setOverlay(OverlayDifficultyLocked)
		invalidateRect.Call(h, 0, 0)
		return
	}
	if next == difficulty && !adaptiveMode {
		return
	}
	if score > 0 || streak > 0 || bestScore > 0 || bestStreak > 0 {
		msg := utf16ptr("Changing difficulty will reset your stats. Continue?")
		title := utf16ptr("Difficulty")
		r, _, _ := messageBoxW.Call(h, uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), MB_YESNO|MB_ICONWARNING)
		if r != IDYES {
			return
		}
	}
	resetAllStats()
	adaptiveMode = false
	difficulty = next
	adaptiveTier = next
	adaptiveMeter = 0
	resetToWaiting(h)
}

func drawSmoothStroke(hdc uintptr, width int, color uintptr) {
	if len(path) < 2 {
		return
	}
	g, ok := gdipGraphics(hdc)
	if !ok {
		p, _, _ := createPen.Call(PS_SOLID, uintptr(width), color)
		if p == 0 {
			return
		}
		old, _, _ := selectObject.Call(hdc, p)
		moveToEx.Call(hdc, uintptr(int32(path[0].X)), uintptr(int32(path[0].Y)), 0)
		for i := 1; i < len(path); i++ {
			lineTo.Call(hdc, uintptr(int32(path[i].X)), uintptr(int32(path[i].Y)))
		}
		selectObject.Call(hdc, old)
		deleteObject.Call(p)
		return
	}
	defer gdipDeleteGraphics.Call(g)
	pts := make([]GDIPPointF, len(path))
	for i, p := range path {
		pts[i] = GDIPPointF{float32(p.X), float32(p.Y)}
	}
	pen := gdipPen(colorRefToARGB(color, 255), float32(width))
	if pen == 0 {
		return
	}
	defer gdipDeletePen.Call(pen)
	gdipDrawLines.Call(g, pen, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
}

func drawSmoothStrokeAlpha(hdc uintptr, width int, color uintptr, alpha byte) {
	if len(path) < 2 {
		return
	}
	g, ok := gdipGraphics(hdc)
	if !ok {
		return
	}
	defer gdipDeleteGraphics.Call(g)
	pts := make([]GDIPPointF, len(path))
	for i, p := range path {
		pts[i] = GDIPPointF{float32(p.X), float32(p.Y)}
	}
	pen := gdipPen(colorRefToARGB(color, alpha), float32(width))
	if pen == 0 {
		return
	}
	defer gdipDeletePen.Call(pen)
	gdipDrawLines.Call(g, pen, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
}

func pathSafetyColor() uintptr {
	if state != StatePlaying || !hasLastMouse {
		return rgb(46, 126, 247)
	}
	t := distanceToPathLocal(lastMouse, progressIndex) / allowedTrackDistance()
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	t = t * t * (3 - 2*t)
	r := byte(math.Round(46 + (232-46)*t))
	g := byte(math.Round(126 + (48-126)*t))
	b := byte(math.Round(247 + (52-247)*t))
	return rgb(r, g, b)
}

func drawMarker(hdc uintptr, p FPoint, start bool) {
	x, y := int32(math.Round(p.X)), int32(math.Round(p.Y))
	accent := rgb(47, 225, 128)
	label := "START"
	if !start {
		accent = rgb(255, 164, 30)
		label = "END"
	}
	g, ok := gdipGraphics(hdc)
	if ok {
		defer gdipDeleteGraphics.Call(g)
		gdipFillCircle(g, x+3, y+4, 29, gdipARGB(62, 0, 0, 0))
		gdipFillCircle(g, x, y, 27, colorRefToARGB(accent, 38))
		gdipStrokeCircle(g, x, y, 23, colorRefToARGB(accent, 120), 4)
		gdipFillCircle(g, x, y, 18, gdipARGB(255, 4, 20, 48))
		gdipStrokeCircle(g, x, y, 18, colorRefToARGB(accent, 255), 3)
		gdipFillCircle(g, x, y, 9, colorRefToARGB(accent, 255))
		gdipFillCircle(g, x, y, 3, gdipARGB(255, 250, 253, 255))
	}
	tw := int32(76)
	if !start {
		tw = 62
	}
	tag := RECT{x - tw/2, y - 57, x + tw/2, y - 34}
	fillSolidRect(hdc, RECT{tag.Left + 2, tag.Top + 3, tag.Right + 2, tag.Bottom + 3}, rgb(2, 12, 29))
	fillSolidRect(hdc, tag, rgb(5, 27, 58))
	drawLineSimple(hdc, tag.Left, tag.Top, tag.Right, tag.Top, 2, accent)
	drawLineSimple(hdc, tag.Left, tag.Top, tag.Left, tag.Bottom, 3, accent)
	drawLineSimple(hdc, x, tag.Bottom, x, y-29, 1, accent)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(246, 251, 254))
		centeredTextOut(hdc, tag.Left, tag.Right, verticallyCenteredTextY(hdc, hudTinyFont, tag, label), label)
		selectObject.Call(hdc, old)
	}
}

func drawTarget(hdc uintptr, t Target, index int) {
	p := targetCurrentPoint(index)
	if t.Clicked {
		p = t.Point
	}
	x, y := int32(math.Round(p.X)), int32(math.Round(p.Y))
	accent := rgb(244, 53, 66)
	label := fmt.Sprintf("TARGET %d", index+1)
	if t.Clicked {
		accent = rgb(48, 221, 124)
		label = fmt.Sprintf("HIT %d", index+1)
	}
	g, ok := gdipGraphics(hdc)
	if ok {
		defer gdipDeleteGraphics.Call(g)
		gdipFillCircle(g, x+2, y+3, 23, gdipARGB(58, 0, 0, 0))
		gdipFillCircle(g, x, y, 21, colorRefToARGB(accent, 40))
		gdipStrokeCircle(g, x, y, 17, colorRefToARGB(accent, 110), 4)
		gdipFillCircle(g, x, y, 14, gdipARGB(255, 5, 20, 46))
		gdipStrokeCircle(g, x, y, 14, colorRefToARGB(accent, 255), 3)
		pen := gdipPen(colorRefToARGB(accent, 255), 2)
		if pen != 0 {
			gdipDrawLineI.Call(g, pen, uintptr(x-9), uintptr(y), uintptr(x-4), uintptr(y))
			gdipDrawLineI.Call(g, pen, uintptr(x+4), uintptr(y), uintptr(x+9), uintptr(y))
			gdipDrawLineI.Call(g, pen, uintptr(x), uintptr(y-9), uintptr(x), uintptr(y-4))
			gdipDrawLineI.Call(g, pen, uintptr(x), uintptr(y+4), uintptr(x), uintptr(y+9))
			gdipDeletePen.Call(pen)
		}
		gdipFillCircle(g, x, y, 3, colorRefToARGB(accent, 255))
	}
	tw := int32(78)
	tag := RECT{x - tw/2, y - 50, x + tw/2, y - 29}
	fillSolidRect(hdc, RECT{tag.Left + 2, tag.Top + 3, tag.Right + 2, tag.Bottom + 3}, rgb(2, 12, 29))
	fillSolidRect(hdc, tag, rgb(5, 24, 52))
	drawLineSimple(hdc, tag.Left, tag.Top, tag.Right, tag.Top, 2, accent)
	drawLineSimple(hdc, tag.Left, tag.Top, tag.Left, tag.Bottom, 3, accent)
	drawLineSimple(hdc, x, tag.Bottom, x, y-22, 1, accent)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, accent)
		centeredTextOut(hdc, tag.Left, tag.Right, verticallyCenteredTextY(hdc, hudTinyFont, tag, label), label)
		selectObject.Call(hdc, old)
	}
}

func drawPathInsideArena(hdc uintptr, w, hgt int32) {
	ar := arenaRect(w, hgt)
	saved, _, _ := saveDC.Call(hdc)
	if saved == 0 {
		drawPath(hdc)
		return
	}
	// One-pixel inner clip prevents antialias/glow bleed into HUD borders.
	intersectClipRect.Call(
		hdc,
		uintptr(ar.Left+1),
		uintptr(ar.Top+1),
		uintptr(ar.Right-1),
		uintptr(ar.Bottom-1),
	)
	drawPath(hdc)
	restoreDC.Call(hdc, saved)
}

func releaseEnduranceRailCache() {
	if enduranceRailDC != 0 {
		if enduranceRailOld != 0 {
			selectObject.Call(enduranceRailDC, enduranceRailOld)
		}
		if enduranceRailBmp != 0 {
			deleteObject.Call(enduranceRailBmp)
		}
		deleteDC.Call(enduranceRailDC)
	}
	enduranceRailDC = 0
	enduranceRailBmp = 0
	enduranceRailOld = 0
	enduranceRailBits = 0
	enduranceRailW = 0
	enduranceRailH = 0
	enduranceRailOriginX = 0
	enduranceRailOriginY = 0
	enduranceRailBase = 0
}

func rebuildEnduranceRailCache(hdc uintptr) bool {
	if len(path) < 2 {
		return false
	}
	w, hgt := getClient(mainHwnd)
	ar := arenaRect(w, hgt)
	if ar.Right <= ar.Left || ar.Bottom <= ar.Top {
		return false
	}

	minX, maxX := path[0].X, path[0].X
	for _, p := range path {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
	}
	const pad = 42.0
	cacheW := int32(math.Ceil(maxX - minX + pad*2))
	cacheH := int32(ar.Bottom-ar.Top) + 84
	if cacheW < 64 || cacheH < 64 {
		return false
	}

	releaseEnduranceRailCache()
	dc, _, _ := createCompatibleDC.Call(hdc)
	if dc == 0 {
		return false
	}
	var bits uintptr
	bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{
		BiSize:  uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth: cacheW, BiHeight: -cacheH,
		BiPlanes: 1, BiBitCount: 32, BiCompression: BI_RGB,
	}}
	bmp, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		deleteDC.Call(dc)
		return false
	}
	old, _, _ := selectObject.Call(dc, bmp)
	buf := unsafe.Slice((*byte)(unsafe.Pointer(bits)), int(cacheW*cacheH*4))
	clear(buf)

	originX := minX - pad
	originY := float64(ar.Top) - 42

	g, ok := gdipGraphics(dc)
	if !ok {
		selectObject.Call(dc, old)
		deleteObject.Call(bmp)
		deleteDC.Call(dc)
		return false
	}
	pts := make([]GDIPPointF, len(path))
	for i, p := range path {
		pts[i] = GDIPPointF{
			X: float32(p.X - originX),
			Y: float32(p.Y - originY),
		}
	}

	base := int(math.Round(enduranceTrackWidth()))
	draw := func(width int, color uintptr, alpha byte) {
		pen := gdipPen(colorRefToARGB(color, alpha), float32(width))
		if pen == 0 {
			return
		}
		gdipDrawLines.Call(g, pen, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
		gdipDeletePen.Call(pen)
	}
	draw(base+26, rgb(42, 196, 255), 38)
	draw(base+19, rgb(8, 20, 40), 255)
	draw(base+14, rgb(62, 82, 108), 255)
	draw(base+8, rgb(226, 237, 246), 255)
	draw(base+2, pathSafetyColor(), 255)
	core := base / 6
	if core < 2 {
		core = 2
	}
	draw(core, rgb(220, 243, 255), 255)
	gdipDeleteGraphics.Call(g)

	enduranceRailDC = dc
	enduranceRailBmp = bmp
	enduranceRailOld = old
	enduranceRailBits = bits
	enduranceRailW = cacheW
	enduranceRailH = cacheH
	enduranceRailOriginX = originX
	enduranceRailOriginY = originY
	enduranceRailBase = base
	enduranceRailDirty = false

	// Optional inspectable runtime cache metadata. Never read during gameplay.
	if cacheRoot != "" {
		meta := fmt.Sprintf("v110\nwidth=%d\nheight=%d\npoints=%d\nbase=%d\n", cacheW, cacheH, len(path), base)
		_ = os.WriteFile(filepath.Join(cacheRoot, "endurance", "rail_cache.meta"), []byte(meta), 0644)
	}
	return true
}

func drawCachedEnduranceRail(hdc uintptr) {
	if len(path) < 2 {
		return
	}
	base := int(math.Round(enduranceTrackWidth()))
	if enduranceRailDirty || enduranceRailDC == 0 || enduranceRailBase != base {
		if !rebuildEnduranceRailCache(hdc) {
			drawEndurancePathFast(hdc)
			return
		}
	}

	w, hgt := getClient(mainHwnd)
	ar := arenaRect(w, hgt)

	dstLeft := int32(math.Round(enduranceRailOriginX - enduranceCameraX))
	dstTop := int32(math.Round(enduranceRailOriginY))
	dstRight := dstLeft + enduranceRailW
	dstBottom := dstTop + enduranceRailH

	// Intersect the cached bitmap with the actual game arena before AlphaBlend.
	// This turns a multi-thousand-pixel blit into approximately one arena-width blit.
	visLeft := dstLeft
	if visLeft < ar.Left {
		visLeft = ar.Left
	}
	visTop := dstTop
	if visTop < ar.Top {
		visTop = ar.Top
	}
	visRight := dstRight
	if visRight > ar.Right {
		visRight = ar.Right
	}
	visBottom := dstBottom
	if visBottom > ar.Bottom {
		visBottom = ar.Bottom
	}
	if visRight <= visLeft || visBottom <= visTop {
		return
	}

	srcX := visLeft - dstLeft
	srcY := visTop - dstTop
	visW := visRight - visLeft
	visH := visBottom - visTop

	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(
		hdc,
		uintptr(visLeft), uintptr(visTop), uintptr(visW), uintptr(visH),
		enduranceRailDC,
		uintptr(srcX), uintptr(srcY), uintptr(visW), uintptr(visH),
		blend,
	)
}

func drawEndurancePathFast(hdc uintptr) {
	if len(path) < 2 {
		return
	}
	base := int(math.Round(enduranceTrackWidth()))

	g, ok := gdipGraphics(hdc)
	if !ok {
		// Safe fallback to the established renderer.
		drawSmoothStrokeAlpha(hdc, base+26, rgb(42, 196, 255), 38)
		drawSmoothStroke(hdc, base+19, rgb(8, 20, 40))
		drawSmoothStroke(hdc, base+14, rgb(62, 82, 108))
		drawSmoothStroke(hdc, base+8, rgb(226, 237, 246))
		drawSmoothStroke(hdc, base+2, pathSafetyColor())
		core := base / 6
		if core < 2 {
			core = 2
		}
		drawSmoothStroke(hdc, core, rgb(220, 243, 255))
		return
	}
	defer gdipDeleteGraphics.Call(g)

	// Draw only a lightly decimated point list. With 9 px source spacing this
	// remains visually smooth while making hard lateral sweeps cheaper to rasterize.
	step := 2
	n := (len(path) + step - 1) / step
	if (len(path)-1)%step != 0 {
		n++
	}
	pts := make([]GDIPPointF, 0, n)
	for i := 0; i < len(path); i += step {
		pts = append(pts, GDIPPointF{float32(path[i].X), float32(path[i].Y)})
	}
	last := path[len(path)-1]
	if len(pts) == 0 || pts[len(pts)-1].X != float32(last.X) || pts[len(pts)-1].Y != float32(last.Y) {
		pts = append(pts, GDIPPointF{float32(last.X), float32(last.Y)})
	}
	if len(pts) < 2 {
		return
	}

	draw := func(width int, color uintptr, alpha byte) {
		pen := gdipPen(colorRefToARGB(color, alpha), float32(width))
		if pen == 0 {
			return
		}
		gdipDrawLines.Call(g, pen, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
		gdipDeletePen.Call(pen)
	}

	draw(base+26, rgb(42, 196, 255), 38)
	draw(base+19, rgb(8, 20, 40), 255)
	draw(base+14, rgb(62, 82, 108), 255)
	draw(base+8, rgb(226, 237, 246), 255)
	draw(base+2, pathSafetyColor(), 255)
	core := base / 6
	if core < 2 {
		core = 2
	}
	draw(core, rgb(220, 243, 255), 255)
}

func drawPath(hdc uintptr) {
	if len(path) < 2 {
		return
	}
	base := 0
	if enduranceActive() {
		base = int(math.Round(enduranceTrackWidth()))
	} else {
		d := diffs[activeDifficultyIndex()]
		base = int(math.Round(d.width))
	}

	if enduranceActive() {
		drawCachedEnduranceRail(hdc)
	} else {
		// Precision path: keep the playable centre readable while making the
		// collision edge unmistakable. A soft red halo sits behind a saturated
		// red boundary ring, followed by the existing bright interior treatment.
		drawSmoothStrokeAlpha(hdc, base+30, rgb(255, 18, 28), 70)
		drawSmoothStrokeAlpha(hdc, base+24, rgb(255, 22, 32), 120)
		drawSmoothStroke(hdc, base+20, rgb(255, 24, 36))
		drawSmoothStroke(hdc, base+12, rgb(24, 8, 14))
		drawSmoothStroke(hdc, base+8, rgb(245, 247, 252))
		drawSmoothStroke(hdc, base+2, pathSafetyColor())
		core := base / 6
		if core < 2 {
			core = 2
		}
		drawSmoothStroke(hdc, core, rgb(220, 243, 255))
	}

	for i, t := range targets {
		if enduranceActive() && t.Clicked {
			continue
		}
		drawTarget(hdc, t, i)
	}
	if enduranceActive() {
		startMarker := path[0]
		startMarker.X -= enduranceCameraX
		drawMarker(hdc, startMarker, true)
	} else {
		drawMarker(hdc, path[0], true)
		drawMarker(hdc, path[len(path)-1], false)
	}
}

func drawOutlineRect(hdc uintptr, r RECT, color uintptr, width int32) {
	if width < 1 {
		width = 1
	}
	fillSolidRect(hdc, RECT{r.Left, r.Top, r.Right, r.Top + width}, color)
	fillSolidRect(hdc, RECT{r.Left, r.Bottom - width, r.Right, r.Bottom}, color)
	fillSolidRect(hdc, RECT{r.Left, r.Top, r.Left + width, r.Bottom}, color)
	fillSolidRect(hdc, RECT{r.Right - width, r.Top, r.Right, r.Bottom}, color)
}

func fillSolidRect(hdc uintptr, r RECT, color uintptr) {
	b, _, _ := createSolidBrush.Call(color)
	if b != 0 {
		fillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), b)
		deleteObject.Call(b)
	}
}

func drawLineSimple(hdc uintptr, x1, y1, x2, y2 int32, width int, color uintptr) {
	p, _, _ := createPen.Call(PS_SOLID, uintptr(width), color)
	if p == 0 {
		return
	}
	old, _, _ := selectObject.Call(hdc, p)
	moveToEx.Call(hdc, uintptr(x1), uintptr(y1), 0)
	lineTo.Call(hdc, uintptr(x2), uintptr(y2))
	selectObject.Call(hdc, old)
	deleteObject.Call(p)
}

func drawPlayfieldPattern(hdc uintptr, w, hgt int32) {
	r := arenaRect(w, hgt)
	minor := rgb(225, 237, 246)
	major := rgb(201, 222, 237)
	dot := rgb(174, 209, 231)
	dx := sx(42, w)
	dy := sy(42, hgt)
	if dx < 24 {
		dx = 24
	}
	if dy < 24 {
		dy = 24
	}

	for x := r.Left + dx; x < r.Right; x += dx {
		c := minor
		if ((x-r.Left)/dx)%4 == 0 {
			c = major
		}
		drawLineSimple(hdc, x, r.Top, x, r.Bottom, 1, c)
	}
	for y := r.Top + dy; y < r.Bottom; y += dy {
		c := minor
		if ((y-r.Top)/dy)%4 == 0 {
			c = major
		}
		drawLineSimple(hdc, r.Left, y, r.Right, y, 1, c)
	}
	for x := r.Left + dx*4; x < r.Right; x += dx * 4 {
		for y := r.Top + dy*4; y < r.Bottom; y += dy * 4 {
			fillSolidRect(hdc, RECT{x - 1, y - 1, x + 2, y + 2}, dot)
		}
	}

	// Soft inner-edge shading gives the arena depth without covering particles.
	fillSolidRect(hdc, RECT{r.Left, r.Top, r.Right, r.Top + 2}, rgb(210, 229, 241))
	fillSolidRect(hdc, RECT{r.Left, r.Bottom - 2, r.Right, r.Bottom}, rgb(225, 237, 246))
	fillSolidRect(hdc, RECT{r.Left, r.Top, r.Left + 2, r.Bottom}, rgb(210, 229, 241))
	fillSolidRect(hdc, RECT{r.Right - 2, r.Top, r.Right, r.Bottom}, rgb(225, 237, 246))
}

func drawPlayfieldBorder(hdc uintptr, w, hgt int32) {
	r := arenaRect(w, hgt)
	ink := rgb(3, 14, 34)
	cyan := rgb(43, 214, 255)
	orange := rgb(255, 149, 19)

	// Dark chassis border + thin cyan inner line.
	drawLineSimple(hdc, r.Left-2, r.Top-2, r.Right+2, r.Top-2, 5, ink)
	drawLineSimple(hdc, r.Left-2, r.Bottom+2, r.Right+2, r.Bottom+2, 5, ink)
	drawLineSimple(hdc, r.Left-2, r.Top, r.Left-2, r.Bottom, 5, ink)
	drawLineSimple(hdc, r.Right+2, r.Top, r.Right+2, r.Bottom, 5, ink)
	drawLineSimple(hdc, r.Left, r.Top, r.Right, r.Top, 1, cyan)
	drawLineSimple(hdc, r.Left, r.Bottom, r.Right, r.Bottom, 1, cyan)

	L := sx(42, w)
	if L < 22 {
		L = 22
	}
	// Cyan technology corners.
	drawLineSimple(hdc, r.Left, r.Top, r.Left+L, r.Top, 4, cyan)
	drawLineSimple(hdc, r.Left, r.Top, r.Left, r.Top+L, 4, cyan)
	drawLineSimple(hdc, r.Right-L, r.Top, r.Right, r.Top, 4, cyan)
	drawLineSimple(hdc, r.Right, r.Top, r.Right, r.Top+L, 4, cyan)
	drawLineSimple(hdc, r.Left, r.Bottom, r.Left+L, r.Bottom, 4, cyan)
	drawLineSimple(hdc, r.Left, r.Bottom-L, r.Left, r.Bottom, 4, cyan)
	drawLineSimple(hdc, r.Right-L, r.Bottom, r.Right, r.Bottom, 4, cyan)
	drawLineSimple(hdc, r.Right, r.Bottom-L, r.Right, r.Bottom, 4, cyan)

	// Small orange energy ticks tie the arena into the logo.
	tick := sx(18, w)
	drawLineSimple(hdc, r.Left+L+8, r.Top, r.Left+L+8+tick, r.Top, 3, orange)
	drawLineSimple(hdc, r.Right-L-8-tick, r.Bottom, r.Right-L-8, r.Bottom, 3, orange)
}

func crosshairColorValue() uintptr {
	colors := []uintptr{rgb(0, 0, 0), rgb(255, 255, 255), rgb(30, 214, 255), rgb(53, 224, 120), rgb(244, 62, 70), rgb(255, 218, 45), rgb(203, 82, 246)}
	i := gameMeta.CrosshairColor
	if i < 0 || i >= len(colors) {
		i = 0
	}
	return colors[i]
}

func drawEnduranceGDIShipCursor(hdc uintptr, w, hgt int32) {
	if !cursorInArena {
		return
	}
	id := gameMeta.SelectedShip
	hw := sx(20, w)
	hh := sy(15, hgt)
	x := int32(math.Round(cursorPos.X))
	y := int32(math.Round(cursorPos.Y))
	drawShipTextureFit(hdc, id, RECT{x - hw, y - hh, x + hw, y + hh})
	drawGDIShipHitbox(hdc, x, y)
}

func drawYellowCrosshair(hdc uintptr) {
	if !cursorInArena || enduranceActive() || survivalActive() {
		return
	}
	x, y := int32(math.Round(cursorPos.X)), int32(math.Round(cursorPos.Y))
	colors := []uintptr{rgb(0, 0, 0), rgb(255, 255, 255), rgb(40, 220, 255), rgb(60, 235, 120), rgb(255, 70, 70), rgb(255, 230, 50), rgb(200, 90, 255)}
	c := colors[gameMeta.CrosshairColor%len(colors)]
	scale := int32(1)
	if gameMeta.CrosshairSize == 0 {
		scale = 0
	} else if gameMeta.CrosshairSize == 2 {
		scale = 3
	}
	arm := int32(11) + scale*2
	gap := int32(2) + scale/2
	wid := 2
	switch gameMeta.CrosshairStyle {
	case 1:
		fillSolidRect(hdc, RECT{x - 2 - scale/2, y - 2 - scale/2, x + 3 + scale/2, y + 3 + scale/2}, c)
	case 2:
		drawLineSimple(hdc, x-arm, y, x-gap, y, wid, c)
		drawLineSimple(hdc, x+gap, y, x+arm, y, wid, c)
		drawLineSimple(hdc, x, y-arm, x, y-gap, wid, c)
		drawLineSimple(hdc, x, y+gap, x, y+arm, wid, c)
		drawCircleOutline(hdc, x, y, 7+scale, int32(wid), c)
	case 3:
		drawCircleOutline(hdc, x, y, 9+scale, int32(wid), c)
		fillSolidRect(hdc, RECT{x - 1, y - 1, x + 2, y + 2}, c)
	case 4:
		drawCircleOutline(hdc, x, y, 11+scale, int32(wid), c)
		drawLineSimple(hdc, x-arm-5, y, x-4, y, wid, c)
		drawLineSimple(hdc, x+4, y, x+arm+5, y, wid, c)
		drawLineSimple(hdc, x, y-arm-5, x, y-4, wid, c)
		drawLineSimple(hdc, x, y+4, x, y+arm+5, wid, c)
	default:
		drawLineSimple(hdc, x-arm, y, x+arm, y, wid, c)
		drawLineSimple(hdc, x, y-arm, x, y+arm, wid, c)
	}
}

func ensureHitFeedbackPen() uintptr {
	if hitFeedbackPen != 0 {
		return hitFeedbackPen
	}
	p, _, _ := createPen.Call(PS_SOLID, 2, rgb(24, 184, 92))
	hitFeedbackPen = p
	return hitFeedbackPen
}

func invalidatePrecisionHitRegion(h uintptr, p FPoint) {
	if h == 0 {
		return
	}
	// Covers the original target, expanding ring and TARGET LOCK label.
	const leftPad = int32(42)
	const rightPad = int32(170)
	const verticalPad = int32(56)
	x := int32(math.Round(p.X))
	y := int32(math.Round(p.Y))
	r := RECT{
		Left:   x - leftPad,
		Top:    y - verticalPad,
		Right:  x + rightPad,
		Bottom: y + verticalPad,
	}
	invalidateRect.Call(h, uintptr(unsafe.Pointer(&r)), 0)
}

func drawHitFeedback(hdc uintptr) {
	if !hitFXEnabled || lastHitAt.IsZero() {
		return
	}
	elapsed := time.Since(lastHitAt)
	if elapsed > 550*time.Millisecond {
		return
	}
	t := float64(elapsed) / float64(550*time.Millisecond)
	r := int32(10 + 18*t)
	x := int32(math.Round(lastHitPoint.X))
	y := int32(math.Round(lastHitPoint.Y))
	c := rgb(24, 184, 92)
	if pen := ensureHitFeedbackPen(); pen != 0 {
		old, _, _ := selectObject.Call(hdc, pen)
		ellipse.Call(hdc, uintptr(x-r), uintptr(y-r), uintptr(x+r), uintptr(y+r))
		selectObject.Call(hdc, old)
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, c)
		textOut(hdc, x+18, y-18, "TARGET LOCK")
		selectObject.Call(hdc, old)
	}
}

func drawMenuButton(hdc uintptr, w, hgt int32) {
	r := menuButtonRect(w, hgt)
	drawBevelPanel(hdc, r, rgb(241, 123, 11), rgb(255, 221, 53), rgb(137, 49, 0), 3)
	label := "MENU  ▶"
	if menuOpen {
		label = "MENU  ▼"
	}
	drawArcadeLabel(hdc, r, label, rgb(255, 255, 255))
}

func drawMenuSectionHeader(hdc uintptr, r RECT, y int32, title string) {
	setTextColor.Call(hdc, rgb(26, 205, 239))
	textOut(hdc, r.Left+20, y, title)
	drawLineSimple(hdc, r.Left+20, y+22, r.Right-20, y+22, 1, rgb(17, 70, 108))
}

func drawMenuRow(hdc uintptr, r RECT, y, rowH int32, label, value string, alternate bool) {
	bar := RECT{r.Left + 14, y, r.Right - 14, y + rowH}
	face := rgb(7, 53, 119)
	if alternate {
		face = rgb(6, 45, 105)
	}
	if pointInRect(cursorPos, bar) {
		face = blendColor(face, rgb(38, 176, 224), 0.22)
	}
	drawBevelPanel(hdc, bar, face, rgb(40, 207, 255), rgb(2, 26, 64), 2)

	// left label / right value with clear separation.
	divider := r.Left + sx(206, 1536)
	drawLineSimple(hdc, divider, bar.Top+6, divider, bar.Bottom-6, 1, rgb(42, 112, 170))
	ty := y + (rowH-16)/2
	setTextColor.Call(hdc, rgb(242, 248, 252))
	textOut(hdc, bar.Left+18, ty, label)
	setTextColor.Call(hdc, rgb(67, 225, 255))
	textOut(hdc, divider+18, ty, value)
}

func drawDropdownMenu(hdc uintptr, w, hgt int32) {
	if !menuOpen {
		return
	}

	r, rowH, rows, sections := menuLayout(w, hgt)
	drawStudioPanel(hdc, r, true)
	if hudSmallFont == 0 {
		return
	}
	old, _, _ := selectObject.Call(hdc, hudSmallFont)
	defer selectObject.Call(hdc, old)

	setTextColor.Call(hdc, rgb(245, 248, 252))
	centeredTextOut(hdc, r.Left, r.Right, r.Top+11, "TRAINING SETTINGS")

	mode := "STANDARD"
	if adaptiveMode {
		mode = "ADAPTIVE"
	}

	drawMenuSectionHeader(hdc, r, sections[0], "TRAINING")
	drawMenuRow(hdc, r, rows[0], rowH, "Mode", mode, false)
	courseValue := courseNames[pathMode]
	drawMenuRow(hdc, r, rows[1], rowH, "Course", courseValue, true)

	drawMenuSectionHeader(hdc, r, sections[1], "ONLINE")
	accountValue := "LOGIN"
	authMu.Lock()
	connected := discordConnected
	displayName := discordDisplayName
	authStatus := discordAuthStatus
	authMu.Unlock()
	if connected {
		accountValue = displayName + "  ✓"
		if displayName == "" {
			accountValue = "CONNECTED  ✓"
		}
	} else if authStatus != "" {
		accountValue = authStatus
	}
	drawMenuRow(hdc, r, rows[2], rowH, "Discord Account", accountValue, false)
	drawMenuRow(hdc, r, rows[3], rowH, "Global Leaderboard", "OPEN", true)

	drawMenuSectionHeader(hdc, r, sections[2], "RECORDS")
	drawMenuRow(hdc, r, rows[4], rowH, "Local Leaderboard", "OPEN", false)
	drawMenuRow(hdc, r, rows[5], rowH, "Close Menu", "CLOSE", true)
}

func drawAchievementToast(hdc uintptr, w, hgt int32) {
	if achievementAt.IsZero() || time.Since(achievementAt) > 3500*time.Millisecond {
		return
	}
	width := sx(430, w)
	height := sy(86, hgt)
	r := RECT{w - width - sx(32, w), sy(198, hgt), w - sx(32, w), sy(198, hgt) + height}
	drawBevelPanel(hdc, r, rgb(4, 31, 65), rgb(53, 211, 255), rgb(2, 15, 36), 2)
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, rgb(61, 219, 255))
		textOut(hdc, r.Left+16, r.Top+12, "ACHIEVEMENT UNLOCKED")
		setTextColor.Call(hdc, rgb(245, 248, 252))
		textOut(hdc, r.Left+16, r.Top+40, lastAchievement)
		if lastAchievementRewardEXP > 0 {
			reward := fmt.Sprintf("+%d EXP", lastAchievementRewardEXP)
			sz := textPixelSize(hdc, hudTinyFont, reward)
			setTextColor.Call(hdc, rgb(255, 210, 58))
			textOut(hdc, r.Right-16-sz.Cx, r.Top+40, reward)
		}
		selectObject.Call(hdc, old)
	}
}

func drawRewardToast(hdc uintptr, w, hgt int32) {
	msg := ""
	if !weeklyRewardAt.IsZero() && time.Since(weeklyRewardAt) < 3500*time.Millisecond {
		msg = "WEEKLY OPS COMPLETE  +200 EXP"
	}
	if !dailyRewardAt.IsZero() && time.Since(dailyRewardAt) < 3500*time.Millisecond {
		if lastDailyRewardEXP > 0 {
			msg = fmt.Sprintf("DAILY OPS REWARD  +%d EXP", lastDailyRewardEXP)
		} else {
			msg = "DAILY OPS REWARD"
		}
	}
	if msg == "" {
		return
	}
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, rgb(242, 192, 55))
		centeredTextOut(hdc, 0, w, sy(205, hgt), msg)
		selectObject.Call(hdc, old)
	}
}

func currencySafeColor(rating string) uintptr {
	switch strings.ToUpper(strings.TrimSpace(rating)) {
	case "S":
		return rgb(255, 202, 54)
	case "A":
		return rgb(79, 229, 143)
	case "B":
		return rgb(67, 199, 255)
	case "C":
		return rgb(255, 158, 43)
	default:
		return rgb(255, 83, 88)
	}
}

func drawResultOverlay(hdc uintptr, w, hgt int32) {
	if survivalBoss1SectionReport {
		return
	}
	if state != StateResult {
		return
	}

	ar := arenaRect(w, hgt)
	width := sx(820, w)
	if width < 650 {
		width = 650
	}
	height := sy(470, hgt)
	if height < 365 {
		height = 365
	}
	left := ar.Left + (ar.Right-ar.Left-width)/2
	baseTop := ar.Top + (ar.Bottom-ar.Top-height)/2
	// Mission Report geometry is fixed. It should never slide or shift when
	// appearing; the background is already frozen for report presentation.
	top := baseTop
	right := left + width
	bottom := top + height

	panel := RECT{left, top, right, bottom}
	drawBevelPanel(hdc, panel, rgb(4, 42, 99), rgb(48, 214, 255), rgb(2, 20, 53), 5)

	cyan := rgb(58, 222, 255)
	white := rgb(247, 251, 254)
	muted := rgb(166, 204, 229)
	gold := rgb(255, 186, 40)
	green := rgb(72, 227, 132)

	// Header.
	header := RECT{left + sx(18, w), top + sy(16, hgt), right - sx(18, w), top + sy(78, hgt)}
	fillSolidRect(hdc, header, rgb(3, 26, 65))
	drawLineSimple(hdc, header.Left, header.Bottom, header.Right, header.Bottom, 2, currencySafeColor(lastResult.Rating))
	if hudTitleFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTitleFont)
		setTextColor.Call(hdc, green)
		headerText := func() string {
			if lastResult.Course == "ENDURANCE" {
				return "ENDURANCE RUN OVER"
			}
			if lastResult.Course == "SURVIVAL" {
				return "SURVIVAL MISSION REPORT"
			}
			return "MISSION COMPLETE"
		}()
		textOut(hdc, header.Left+sx(20, w), verticallyCenteredTextY(hdc, hudTitleFont, header, headerText), headerText)
		selectObject.Call(hdc, old)
	}
	if lastResult.Course == "ENDURANCE" && polishEnduranceNewPB && hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(255, 214, 54))
		label := "NEW PERSONAL BEST"
		sz := textPixelSize(hdc, hudTinyFont, label)
		textOut(hdc, header.Right-sx(18, w)-sz.Cx, verticallyCenteredTextY(hdc, hudTinyFont, header, label), label)
		selectObject.Call(hdc, old)
	}

	// Large rating card.
	ratingR := RECT{left + sx(26, w), top + sy(98, hgt), left + sx(210, w), bottom - sy(80, hgt)}
	drawBevelPanel(hdc, ratingR, rgb(3, 28, 66), currencySafeColor(lastResult.Rating), rgb(1, 14, 37), 3)

	if lastResult.Course == "ENDURANCE" || lastResult.Course == "SURVIVAL" {
		// Endurance and Survival already show time in dedicated stat cards.
		// Keep the far-left card focused only on grade and accuracy.
		gradeText := "GRADE: " + lastResult.Rating
		accuracyValue := lastResult.CombinedAcc
		if lastResult.Course == "SURVIVAL" {
			accuracyValue = lastResult.TargetAcc
		}
		accuracyText := fmt.Sprintf("%.1f%%", accuracyValue)
		labelText := "ACCURACY"

		gradeH := int32(20)
		accuracyH := int32(16)
		labelH := int32(12)
		if hudTitleFont != 0 {
			gradeH = textPixelSize(hdc, hudTitleFont, gradeText).Cy
		}
		if hudSmallFont != 0 {
			accuracyH = textPixelSize(hdc, hudSmallFont, accuracyText).Cy
		}
		if hudTinyFont != 0 {
			labelH = textPixelSize(hdc, hudTinyFont, labelText).Cy
		}
		gap1 := sy(18, hgt)
		gap2 := sy(9, hgt)
		groupH := gradeH + gap1 + accuracyH + gap2 + labelH
		y := ratingR.Top + ((ratingR.Bottom-ratingR.Top)-groupH)/2

		if hudTitleFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTitleFont)
			setTextColor.Call(hdc, currencySafeColor(lastResult.Rating))
			centeredTextOut(hdc, ratingR.Left, ratingR.Right, y, gradeText)
			selectObject.Call(hdc, old)
		}
		y += gradeH + gap1
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setTextColor.Call(hdc, white)
			centeredTextOut(hdc, ratingR.Left, ratingR.Right, y, accuracyText)
			selectObject.Call(hdc, old)
		}
		y += accuracyH + gap2
		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, muted)
			centeredTextOut(hdc, ratingR.Left, ratingR.Right, y, labelText)
			selectObject.Call(hdc, old)
		}
	} else {
		if hudTitleFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTitleFont)
			setTextColor.Call(hdc, currencySafeColor(lastResult.Rating))
			centeredTextOut(hdc, ratingR.Left, ratingR.Right, ratingR.Top+sy(43, hgt), lastResult.Rating)
			selectObject.Call(hdc, old)
		}
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setTextColor.Call(hdc, white)
			centeredTextOut(hdc, ratingR.Left, ratingR.Right, ratingR.Top+sy(95, hgt), fmt.Sprintf("%.1f%% ACCURACY", lastResult.CombinedAcc))
			setTextColor.Call(hdc, muted)
			centeredTextOut(hdc, ratingR.Left, ratingR.Right, ratingR.Top+sy(127, hgt), fmt.Sprintf("%.2fs", lastResult.Time))
			centeredTextOut(hdc, ratingR.Left, ratingR.Right, ratingR.Top+sy(157, hgt), lastResult.Difficulty)
			selectObject.Call(hdc, old)
		}
	}

	// Six stat cards.
	type rs struct {
		label, value string
		accent       uintptr
	}
	stats := []rs{
		{"TRACKING", fmt.Sprintf("%.1f%%", lastResult.TrackingAcc), cyan},
		{"TARGET AIM", fmt.Sprintf("%.1f%%", lastResult.TargetAcc), rgb(255, 92, 95)},
		{"ROUND SCORE", fmt.Sprintf("+%d", lastResult.RoundPoints), gold},
		{"TOTAL SCORE", fmt.Sprintf("%d", lastResult.TotalScore), white},
		{"STREAK", fmt.Sprintf("%d", lastResult.Streak), rgb(255, 112, 76)},
		{"COMBO", fmt.Sprintf("x%.2g", lastResult.Combo), rgb(198, 102, 255)},
	}
	if lastResult.Course == "ENDURANCE" {
		stats = []rs{
			{"DISTANCE", fmt.Sprintf("%.1f m", lastResult.Distance), cyan},
			{"TARGETS HIT", fmt.Sprintf("%d", lastResult.TargetsHit), rgb(255, 92, 95)},
			{"SURVIVAL TIME", fmt.Sprintf("%.1fs", lastResult.Time), gold},
			{"TOTAL SCORE", fmt.Sprintf("%d", lastResult.TotalScore), white},
			{"TRACKING", fmt.Sprintf("%.1f%%", lastResult.TrackingAcc), rgb(255, 112, 76)},
			{"BEST DISTANCE", fmt.Sprintf("%.1f m", gameMeta.BestEnduranceDistance), rgb(198, 102, 255)},
		}
	}
	if lastResult.Course == "SURVIVAL" {
		stats = []rs{
			{"WAVE REACHED", fmt.Sprintf("%d", lastResult.Streak), cyan},
			{"ENEMIES DESTROYED", fmt.Sprintf("%d", lastResult.TargetsHit), rgb(255, 92, 95)},
			{"SURVIVAL TIME", fmt.Sprintf("%.1fs", lastResult.Time), gold},
			{"TOTAL SCORE", fmt.Sprintf("%d", lastResult.TotalScore), white},
			{"CENTRE HITS", fmt.Sprintf("%.1f%%", lastResult.TargetAcc), rgb(255, 112, 76)},
			{"AVG REACTION", fmt.Sprintf("%.0f ms", survivalAverageReactionMS()), rgb(198, 102, 255)},
		}
	}
	gridLeft := left + sx(232, w)
	gridRight := right - sx(26, w)
	gridTop := top + sy(98, hgt)
	gapX := sx(12, w)
	gapY := sy(12, hgt)
	cardW := (gridRight - gridLeft - gapX*2) / 3
	cardH := sy(92, hgt)
	for i, st := range stats {
		row := i / 3
		col := i % 3
		x := gridLeft + int32(col)*(cardW+gapX)
		y := gridTop + int32(row)*(cardH+gapY)
		cr := RECT{x, y, x + cardW, y + cardH}
		drawBevelPanel(hdc, cr, rgb(4, 31, 72), st.accent, rgb(1, 16, 40), 2)
		// Center the label/value pair vertically as a single group inside each card.
		labelH := int32(12)
		valueH := int32(15)
		if hudTinyFont != 0 {
			labelH = textPixelSize(hdc, hudTinyFont, st.label).Cy
		}
		if hudSmallFont != 0 {
			valueH = textPixelSize(hdc, hudSmallFont, st.value).Cy
		}
		groupGap := sy(8, hgt)
		groupH := labelH + groupGap + valueH
		groupTop := cr.Top + ((cr.Bottom-cr.Top)-groupH)/2

		if hudTinyFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudTinyFont)
			setTextColor.Call(hdc, muted)
			centeredTextOut(hdc, cr.Left, cr.Right, groupTop, st.label)
			selectObject.Call(hdc, old)
		}
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setTextColor.Call(hdc, st.accent)
			centeredTextOut(hdc, cr.Left, cr.Right, groupTop+labelH+groupGap, st.value)
			selectObject.Call(hdc, old)
		}
	}

	// EXP reward strip.
	expR := RECT{gridLeft, gridTop + cardH*2 + gapY + sy(9, hgt), gridRight, bottom - sy(80, hgt)}
	drawBevelPanel(hdc, expR, rgb(4, 36, 80), cyan, rgb(1, 18, 45), 2)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		rowText := "TOTAL EXP  " + fmt.Sprintf("%d", lastResult.TotalEXP)
		rowY := verticallyCenteredTextY(hdc, hudSmallFont, expR, rowText)

		if lastResult.Course == "ENDURANCE" {
			// Use the same Space Coin PNG language as Space Cache rewards.
			coinSize := sy(28, hgt)
			if coinSize < 20 {
				coinSize = 20
			}
			amount := fmt.Sprintf("+%d", lastResult.CoinsEarned)
			total := fmt.Sprintf("TOTAL  %d", gameMeta.SpaceCoins)
			amountSz := textPixelSize(hdc, hudSmallFont, amount)
			gap := sx(7, w)
			coinLeft := expR.Left + sx(18, w)
			coinTop := expR.Top + ((expR.Bottom-expR.Top)-coinSize)/2
			coinR := RECT{coinLeft, coinTop, coinLeft + coinSize, coinTop + coinSize}
			drawRawBGRAFit(hdc, spaceCoinBGRA, 200, 203, coinR)

			setTextColor.Call(hdc, gold)
			textOut(hdc, coinR.Right+gap, verticallyCenteredTextY(hdc, hudSmallFont, coinR, amount), amount)

			setTextColor.Call(hdc, white)
			totalX := coinR.Right + gap + amountSz.Cx + sx(28, w)
			textOut(hdc, totalX, verticallyCenteredTextY(hdc, hudSmallFont, expR, total), total)
		} else {
			setTextColor.Call(hdc, gold)
			leftText := fmt.Sprintf("+%d EXP", lastResult.EXPEarned)
			textOut(hdc, expR.Left+sx(18, w), verticallyCenteredTextY(hdc, hudSmallFont, expR, leftText), leftText)
			setTextColor.Call(hdc, white)
			textOut(hdc, expR.Left+sx(140, w), rowY, rowText)
		}

		setTextColor.Call(hdc, cyan)
		courseY := verticallyCenteredTextY(hdc, hudSmallFont, expR, lastResult.Course)
		courseSz := textPixelSize(hdc, hudSmallFont, lastResult.Course)
		textOut(hdc, expR.Right-sx(18, w)-courseSz.Cx, courseY, lastResult.Course)
		selectObject.Call(hdc, old)
	}

	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, cyan)
		centeredTextOut(hdc, left, right, bottom-sy(68, hgt), "CLICK TO CONTINUE")
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setTextColor.Call(hdc, muted)
		centeredTextOut(hdc, left, right, bottom-sy(42, hgt), "L = LOCAL LEADERBOARD     •     G = GLOBAL LEADERBOARD")
		selectObject.Call(hdc, old)
	}
}

func drawNameEntryOverlay(hdc uintptr, w, hgt int32) {
	if overlayMode != OverlayNameEntry {
		return
	}
	ar := arenaRect(w, hgt)
	left := ar.Left + (ar.Right-ar.Left)/2 - sx(260, w)
	right := ar.Left + (ar.Right-ar.Left)/2 + sx(260, w)
	top := ar.Top + sy(170, hgt)
	bottom := top + sy(220, hgt)
	drawStudioPanel(hdc, RECT{left, top, right, bottom}, false)
	cyan := rgb(26, 205, 239)
	white := rgb(245, 248, 252)
	drawLineSimple(hdc, left, top, right, top, 2, cyan)
	drawLineSimple(hdc, left, bottom, right, bottom, 2, cyan)
	drawLineSimple(hdc, left, top, left, bottom, 2, cyan)
	drawLineSimple(hdc, right, top, right, bottom, 2, cyan)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, cyan)
		centeredTextOut(hdc, left, right, top+24, "SAVE TO LOCAL LEADERBOARD")
		setTextColor.Call(hdc, white)
		textOut(hdc, left+40, top+72, "ENTER NAME:")
		fillSolidRect(hdc, RECT{left + 40, top + 100, right - 40, top + 137}, rgb(8, 30, 52))
		setTextColor.Call(hdc, rgb(255, 255, 255))
		textOut(hdc, left+52, top+108, nameInput+"_")
		setTextColor.Call(hdc, rgb(160, 190, 215))
		centeredTextOut(hdc, left, right, top+166, "ENTER = SAVE     ESC = CANCEL")
		selectObject.Call(hdc, old)
	}
}

func drawLevelUpConfetti(hdc uintptr, originX, originY int32) {
	if levelUpAt.IsZero() {
		return
	}

	elapsed := time.Since(levelUpAt)
	total := 1600 * time.Millisecond
	if elapsed < 0 || elapsed > total {
		return
	}

	t := float64(elapsed) / float64(total)

	// Deterministic particles so the animation is stable frame-to-frame.
	angles := []float64{
		-2.82, -2.50, -2.20, -1.92, -1.65, -1.35, -1.05, -0.78,
		-0.46, -0.18, 0.20, 0.50, 0.82, 1.12, 1.44, 1.77,
	}
	speeds := []float64{
		44, 56, 49, 63, 52, 68, 47, 58, 65, 51, 62, 45, 57, 69, 50, 61,
	}
	colors := []uintptr{
		rgb(26, 205, 239),
		rgb(255, 205, 48),
		rgb(235, 70, 72),
		rgb(72, 210, 112),
		rgb(190, 52, 246),
	}

	for i, a := range angles {
		speed := speeds[i%len(speeds)]
		// Ease outward, then drift down slightly.
		radius := speed * (1 - math.Pow(1-t, 2))
		x := float64(originX) + math.Cos(a)*radius
		y := float64(originY) + math.Sin(a)*radius + 26*t*t

		size := int32(4)
		if i%3 == 0 {
			size = 5
		}

		c := colors[i%len(colors)]
		fillSolidRect(
			hdc,
			RECT{int32(x) - size/2, int32(y) - size/2, int32(x) + size/2 + 1, int32(y) + size/2 + 1},
			c,
		)
	}
}

func drawAdvancedObjectivePanel(hdc uintptr, w, hgt int32) {
	if enduranceActive() && state == StateWaiting && (overlayMode == OverlayNone || overlayMode == OverlaySpaceCache) {
		drawEnduranceSpaceHUD(hdc, w, hgt)
		return
	}
	// v163: centre objective card is vertically centred in the lower HUD dock
	// and kept clear of the arena by the same top margin as the side panels.
	left := sx(478, w)
	top := sy(820, hgt)
	right := sx(1022, w)
	bottom := sy(982, hgt)
	r := RECT{left, top, right, bottom}
	drawBevelPanel(hdc, r, rgb(4, 42, 99), rgb(47, 213, 255), rgb(2, 21, 55), 4)

	// status rail
	drawLineSimple(hdc, left+sx(18, w), top+sy(18, hgt), right-sx(18, w), top+sy(18, hgt), 2, rgb(52, 220, 255))
	drawLineSimple(hdc, left+sx(18, w), bottom-sy(18, hgt), right-sx(18, w), bottom-sy(18, hgt), 2, rgb(255, 149, 21))

	title := ""
	instruction := ""
	info := ""
	switch state {
	case StateWaiting:
		if overlayMode == OverlaySpaceCache {
			title = "SPACE CACHE"
			instruction = strings.ToUpper(spaceCacheRewardText)
			info = ""
		} else if survivalActive() {
			title = "SURVIVAL"
			instruction = "CLICK THE SPACE STATION TO BEGIN"
			info = "RED = LEFT CLICK  •  BLUE = RIGHT CLICK"
		} else if enduranceActive() {
			title = "ENDURANCE"
			instruction = "START ON THE LEFT • SURVIVE RIGHTWARD"
			info = ""
		} else {
			title = "READY"
			instruction = "START  →  TARGETS  →  END"
			dn, _ := difficultyDisplay()
			mv := "STATIC"
			if movingTargetsActive() {
				mv = "MOVING"
			}
			info = fmt.Sprintf("%s  •  %s  •  %d TARGETS  •  %s", courseNames[currentCourse], dn, len(targets), mv)
		}
	case StatePlaying:
		if survivalActive() {
			if survivalBoss1Active() {
				title = "THE SENTINEL"
				if survivalBoss1CombatActive() {
					instruction = "TARGET ONLY THE ACTIVE WEAK POINT"
					info = "RED = LEFT  •  BLUE = RIGHT"
				} else {
					instruction = "BOSS ENCOUNTER // HOLD FIRE"
					info = "PLAYER VS BOSS // STATION SYSTEMS OFFLINE"
				}
			} else {
				title = fmt.Sprintf("WAVE %d", survivalWave)
				instruction = "DESTROY EVERY THREAT BEFORE IT REACHES THE STATION"
				info = "RED = LEFT  •  BLUE = RIGHT  •  LARGE = 2 HITS / 2 DAMAGE"
			}
		} else if enduranceActive() {
			t := currentRequiredTarget()
			if t >= 0 {
				title = "TARGET ACTIVE"
				instruction = "CLICK THE RED TARGET WHEN YOU REACH IT"
			} else {
				title = "KEEP MOVING"
				instruction = "STAY ON THE RAIL — DO NOT FIRE"
			}
			info = ""
		} else {
			t := currentRequiredTarget()
			if t >= 0 {
				title = fmt.Sprintf("TARGET %d / %d", t+1, len(targets))
				instruction = "CLICK THE RED TARGET"
			} else {
				title = "TARGETS CLEAR"
				instruction = "REACH END — DO NOT FIRE"
			}
			info = fmt.Sprintf("%.1fs  •  TRACKING %.0f%%  •  COMBO x%.2g", updateRealTimeClock(), trackingAccuracyPercent(), comboForStreak(streak+1))
		}
	case StateFailed:
		title = "FAILED"
		instruction = status
		if survivalActive() {
			info = fmt.Sprintf("WAVE %d  •  %d KILLS  •  %.0f ms AVG REACTION", survivalWave, survivalKills, survivalAverageReactionMS())
		} else if enduranceActive() {
			info = ""
		} else {
			info = "SCORE + STREAK RESET  •  NEW RUN IN 2 SECONDS"
		}
	case StateResult:
		if survivalActive() && lastResult.Course == "SURVIVAL" {
			title = "SURVIVAL RUN OVER"
			instruction = "MISSION REPORT READY — CLICK TO CONTINUE"
			info = fmt.Sprintf("WAVE %d  •  %d KILLS  •  %.1f%% ACCURACY", lastResult.Streak, lastResult.TargetsHit, lastResult.TargetAcc)
		} else {
			title = "MISSION COMPLETE"
			if !levelUpAt.IsZero() {
				instruction = fmt.Sprintf("LEVEL UP!  %s  •  +%d EXP", lastResult.Rank, lastResult.EXPEarned)
			} else {
				instruction = fmt.Sprintf("+%d EXP  •  %s", lastResult.EXPEarned, lastResult.Rank)
			}
			info = fmt.Sprintf("ACCURACY %.0f%%  •  SCORE %d  •  STREAK %d", lastResult.CombinedAcc, lastResult.TotalScore, lastResult.Streak)
		}
	}

	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		tc := rgb(65, 229, 255)
		if state == StateFailed {
			tc = rgb(255, 79, 73)
		} else if state == StateResult {
			tc = rgb(255, 175, 30)
		}
		setTextColor.Call(hdc, tc)
		centeredTextOut(hdc, left+sx(20, w), right-sx(20, w), top+sy(37, hgt), title)
		setTextColor.Call(hdc, rgb(255, 255, 255))
		instruction = fitTextEllipsis(hdc, hudSmallFont, instruction, (right-left)-sx(48, w))
		centeredTextOut(hdc, left+sx(24, w), right-sx(24, w), top+sy(76, hgt), instruction)
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 && info != "" {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(176, 219, 246))
		infoText := fitTextEllipsis(hdc, hudTinyFont, strings.ToUpper(info), (right-left)-sx(48, w))
		centeredTextOut(hdc, left+sx(24, w), right-sx(24, w), top+sy(116, hgt), infoText)
		selectObject.Call(hdc, old)
	}
}

func overlaySolidAlpha(hdc uintptr, w, hgt int32, color uintptr, alpha byte) {
	alphaSolidRect(hdc, RECT{0, 0, w, hgt}, color, alpha)
}

func overlayBlack(hdc uintptr, w, hgt int32, alpha byte) {
	overlaySolidAlpha(hdc, w, hgt, rgb(0, 0, 0), alpha)
}

func drawSelectedPenLine(hdc uintptr, x1, y1, x2, y2 int32) {
	moveToEx.Call(hdc, uintptr(x1), uintptr(y1), 0)
	lineTo.Call(hdc, uintptr(x2), uintptr(y2))
}

func drawSelectedPenCircle(hdc uintptr, cx, cy, r int32) {
	if r <= 0 {
		return
	}
	const segments = 96
	var firstX, firstY int32

	for i := 0; i <= segments; i++ {
		a := float64(i) * 2 * math.Pi / segments
		x := cx + int32(math.Round(math.Cos(a)*float64(r)))
		y := cy + int32(math.Round(math.Sin(a)*float64(r)))

		if i == 0 {
			firstX = x
			firstY = y
			moveToEx.Call(hdc, uintptr(x), uintptr(y), 0)
		} else {
			lineTo.Call(hdc, uintptr(x), uintptr(y))
		}
	}
	lineTo.Call(hdc, uintptr(firstX), uintptr(firstY))
}

func drawReticleGeometry(hdc uintptr, cx, cy, r int32) {
	if r < 18 {
		r = 18
	}

	drawSelectedPenCircle(hdc, cx, cy, r)
	drawSelectedPenCircle(hdc, cx, cy, int32(float64(r)*0.54))
	drawSelectedPenCircle(hdc, cx, cy, int32(float64(r)*0.17))

	gap := int32(float64(r) * 0.20)
	outer := int32(float64(r) * 1.18)

	drawSelectedPenLine(hdc, cx-outer, cy, cx-gap, cy)
	drawSelectedPenLine(hdc, cx+gap, cy, cx+outer, cy)
	drawSelectedPenLine(hdc, cx, cy-outer, cx, cy-gap)
	drawSelectedPenLine(hdc, cx, cy+gap, cx, cy+outer)

	// Sniper-style outer ticks.
	tickStart := int32(float64(r) * 0.78)
	tickEnd := int32(float64(r) * 0.96)
	for _, a := range []float64{
		math.Pi / 4, 3 * math.Pi / 4, 5 * math.Pi / 4, 7 * math.Pi / 4,
	} {
		x1 := cx + int32(math.Cos(a)*float64(tickStart))
		y1 := cy + int32(math.Sin(a)*float64(tickStart))
		x2 := cx + int32(math.Cos(a)*float64(tickEnd))
		y2 := cy + int32(math.Sin(a)*float64(tickEnd))
		drawSelectedPenLine(hdc, x1, y1, x2, y2)
	}
}

func drawBlueSniperCrosshair(hdc uintptr, cx, cy, r int32, intensity float64) {
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}

	// Outer glow.
	glow := rgb(
		byte(0),
		byte(70+80*intensity),
		byte(145+100*intensity),
	)
	penGlow, _, _ := createPen.Call(PS_SOLID, 5, glow)
	if penGlow != 0 {
		old, _, _ := selectObject.Call(hdc, penGlow)
		drawReticleGeometry(hdc, cx, cy, r)
		selectObject.Call(hdc, old)
		deleteObject.Call(penGlow)
	}

	bright := rgb(
		byte(20),
		byte(160+70*intensity),
		byte(230+25*intensity),
	)
	pen, _, _ := createPen.Call(PS_SOLID, 2, bright)
	if pen != 0 {
		old, _, _ := selectObject.Call(hdc, pen)
		drawReticleGeometry(hdc, cx, cy, r)
		selectObject.Call(hdc, old)
		deleteObject.Call(pen)
	}

	// Center dot.
	b, _, _ := createSolidBrush.Call(bright)
	if b != 0 {
		old, _, _ := selectObject.Call(hdc, b)
		ellipse.Call(hdc, uintptr(cx-4), uintptr(cy-4), uintptr(cx+5), uintptr(cy+5))
		selectObject.Call(hdc, old)
		deleteObject.Call(b)
	}
}

func introGray(v int) uintptr {
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return rgb(byte(v), byte(v), byte(v))
}

func drawIntroStackGlyph(hdc uintptr, cx, cy int32, spread float64, brightness int) {
	// Fallback for systems where MCI cannot play the authored MP4. Keep the
	// same publisher identity as the video: a clean vertical K / O / N / G
	// stack rather than the old abstract plate glyph.
	if introLogoFont == 0 {
		return
	}
	old, _, _ := selectObject.Call(hdc, introLogoFont)
	setTextColor.Call(hdc, introGray(brightness))
	step := int32(math.Round(92 * spread))
	if step < 70 {
		step = 70
	}
	letters := []string{"K", "O", "N", "G"}
	startY := cy - (step*3)/2 - sy(35, 900)
	for i, letter := range letters {
		y := startY + int32(i)*step
		// Small offset halo gives the fallback the same restrained luminous edge
		// as the video without introducing a solid logo card.
		halo := brightness / 4
		if halo < 24 {
			halo = 24
		}
		setTextColor.Call(hdc, introGray(halo))
		centeredTextOut(hdc, cx-90, cx+90, y+2, letter)
		setTextColor.Call(hdc, introGray(brightness))
		centeredTextOut(hdc, cx-90, cx+90, y, letter)
	}
	selectObject.Call(hdc, old)
}
func drawIntroBlueprint(hdc uintptr, w, hgt int32, t float64) {
	fillSolidRect(hdc, RECT{0, 0, w, hgt}, rgb(0, 0, 0))

	// Nearly-black technical geometry from the supplied reference: oversized
	// arcs, diagonal construction lines, hatch bands and sparse cross markers.
	fade := 1.0
	if t < 0.25 {
		fade = t / 0.25
	}
	if t > 1.75 {
		fade = math.Max(0, 1-(t-1.75)/0.35)
	}
	base := int(34 * fade)
	bright := int(116 * fade)
	if base <= 0 {
		return
	}

	cx, cy := w/2, hgt/2
	// Large construction circles.
	for _, q := range []struct{ x, y, r int32 }{
		{sx(205, w), sy(270, hgt), sy(182, hgt)},
		{sx(1260, w), sy(650, hgt), sy(250, hgt)},
		{sx(890, w), sy(430, hgt), sy(155, hgt)},
	} {
		pen, _, _ := createPen.Call(PS_SOLID, 1, introGray(base))
		if pen != 0 {
			old, _, _ := selectObject.Call(hdc, pen)
			drawSelectedPenCircle(hdc, q.x, q.y, q.r)
			selectObject.Call(hdc, old)
			deleteObject.Call(pen)
		}
	}

	// Long technical diagonals.
	for i := 0; i < 6; i++ {
		y := sy(float64(120+i*145), hgt)
		drawLineSimple(hdc, -sx(100, w), y, w+sx(160, w), y-sy(430, hgt), 1, introGray(base+6))
	}
	// Narrow hatched band like the reference's upper diagonal ribbon.
	for i := 0; i < 34; i++ {
		x := sx(float64(70+i*48), w)
		y := sy(float64(365-i*5), hgt)
		drawLineSimple(hdc, x, y, x+sx(90, w), y+sy(42, hgt), 1, introGray(base+12))
	}
	// Sparse calibration crosses.
	for i := 0; i < 18; i++ {
		x := sx(float64(90+(i*137)%1390), w)
		y := sy(float64(170+(i*83)%690), hgt)
		drawLineSimple(hdc, x-3, y, x+3, y, 1, introGray(base+15))
		drawLineSimple(hdc, x, y-3, x, y+3, 1, introGray(base+15))
	}

	// The vertical K/O/N/G publisher stack resolves as the sequence progresses.
	p := math.Max(0, math.Min(1, t/1.6))
	spread := 1.0 - 0.58*p
	gx := cx + int32(math.Sin(t*2.2)*float64(sx(95, w)))
	gy := cy - sy(55, hgt) + int32(math.Cos(t*1.7)*float64(sy(28, hgt)))
	drawIntroStackGlyph(hdc, gx, gy, spread, bright)

	// One bright diagonal sweep immediately before the wordmark resolves.
	if t > 1.35 && t < 1.9 {
		q := (t - 1.35) / (1.9 - 1.35)
		x := int32(float64(w) * q)
		drawLineSimple(hdc, x-sx(330, w), hgt, x+sx(160, w), 0, 1, introGray(int(180*(1-math.Abs(q-.5)*1.2))))
	}
}

func drawKongIntroLogo(hdc uintptr, w, hgt int32, reveal float64) {
	if introLogoFont == 0 {
		return
	}
	cx := w / 2
	cy := hgt/2 - sy(12, hgt)
	word := "KongGames ©"
	if reveal < 1 {
		n := int(math.Ceil(reveal * float64(len([]rune(word)))))
		if n < 1 {
			n = 1
		}
		r := []rune(word)
		if n > len(r) {
			n = len(r)
		}
		word = string(r[:n])
	}
	old, _, _ := selectObject.Call(hdc, introLogoFont)
	// Soft monochrome halo followed by a pure-white core, matching the supplied
	// intro's restrained publisher mark rather than the old blue/crown treatment.
	for _, off := range []POINT{{-3, 0}, {3, 0}, {0, -3}, {0, 3}, {-2, -2}, {2, 2}} {
		setTextColor.Call(hdc, introGray(46))
		centeredTextOut(hdc, cx-sx(420, w)+off.X, cx+sx(420, w)+off.X, cy-sy(50, hgt)+off.Y, word)
	}
	setTextColor.Call(hdc, rgb(246, 246, 246))
	centeredTextOut(hdc, cx-sx(420, w), cx+sx(420, w), cy-sy(50, hgt), word)
	selectObject.Call(hdc, old)
}

func drawGeneratedIntroLogo(hdc uintptr, w, hgt, size int32)      {}
func drawIntroImpact(hdc uintptr, w, hgt int32, strength float64) {}

func introElapsed() time.Duration {
	if introStart.IsZero() {
		return 0
	}
	return time.Since(introStart)
}

func drawIntroFrame(hdc uintptr, w, hgt int32) {
	if introVideoPlaying {
		return
	}
	ms := float64(introElapsed()) / float64(time.Millisecond)
	t := ms / 1000.0

	// Reference timing: technical construction phase -> partial publisher mark ->
	// clean white wordmark -> fast black exit. Total ~3.6 s.
	if ms < 1980 {
		drawIntroBlueprint(hdc, w, hgt, t)
		glitch := survivalBossIntroGlitchStrength(t)
		if glitch > .04 {
			survivalBossIntroDrawTears(hdc, w, hgt, t, glitch)
			if glitch > .78 {
				flash := byte(math.Min(80, (glitch-.78)*260))
				alphaSolidRect(hdc, RECT{0, 0, w, hgt}, rgb(235, 242, 246), flash)
			}
		}
		survivalBossIntroDrawScan(hdc, w, hgt, t)
		survivalBossIntroDrawVignette(hdc, w, hgt)
		return
	}

	fillSolidRect(hdc, RECT{0, 0, w, hgt}, rgb(0, 0, 0))
	if ms < 2380 {
		reveal := (ms - 1980) / (2380 - 1980)
		drawKongIntroLogo(hdc, w, hgt, reveal)
		glitch := survivalBossIntroGlitchStrength(t)
		if glitch > .04 {
			survivalBossIntroDrawTears(hdc, w, hgt, t, glitch)
		}
		survivalBossIntroDrawScan(hdc, w, hgt, t)
		survivalBossIntroDrawVignette(hdc, w, hgt)
		// Stutter the reveal with two very short blackout slices, exactly like the
		// reference's fragment-to-logo snap.
		if (ms > 2070 && ms < 2115) || (ms > 2220 && ms < 2250) {
			overlayBlack(hdc, w, hgt, 210)
		}
		return
	}
	if ms < 3120 {
		drawKongIntroLogo(hdc, w, hgt, 1)
		glitch := survivalBossIntroGlitchStrength(t)
		if glitch > .04 {
			survivalBossIntroDrawTears(hdc, w, hgt, t, glitch)
		}
		survivalBossIntroDrawScan(hdc, w, hgt, t)
		survivalBossIntroDrawVignette(hdc, w, hgt)
		return
	}
	if ms < 3600 {
		drawKongIntroLogo(hdc, w, hgt, 1)
		glitch := survivalBossIntroGlitchStrength(t)
		if glitch > .04 {
			survivalBossIntroDrawTears(hdc, w, hgt, t, glitch)
		}
		survivalBossIntroDrawScan(hdc, w, hgt, t)
		survivalBossIntroDrawVignette(hdc, w, hgt)
		f := (ms - 3120) / (3600 - 3120)
		overlayBlack(hdc, w, hgt, byte(math.Round(255*f)))
		return
	}

	fillSolidRect(hdc, RECT{0, 0, w, hgt}, rgb(0, 0, 0))
}

func minInt32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func startKongIntroVideo(h uintptr) bool {
	if h == 0 {
		return false
	}
	w, hgt := getClient(h)
	if w <= 0 || hgt <= 0 {
		return false
	}
	if introVideoHwnd == 0 {
		class := utf16ptr("STATIC")
		title := utf16ptr("")
		child, _, _ := createWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(class)),
			uintptr(unsafe.Pointer(title)),
			WS_CHILD|WS_DISABLED|WS_CLIPSIBLINGS,
			0, 0, uintptr(w), uintptr(hgt),
			h, 0, func() uintptr { v, _, _ := getModuleHandleW.Call(0); return v }(), 0,
		)
		if child == 0 {
			logRuntimeEvent("intro_overlay", "child_window_failed")
			return false
		}
		introVideoHwnd = child
	}

	// v455: the supplied MP4 is always attempted first, unchanged.  The WMV is a
	// Windows-native compatibility copy made from that exact MP4 and is only used
	// if the laptop's legacy MCI stack cannot open H.264 MP4 directly.
	candidates := []struct {
		path string
		kind string
	}{
		{filepath.Join(assetRoot, "video", "konggames_intro.mp4"), "exact_mp4"},
		{filepath.Join(assetRoot, "video", "konggames_intro_compat.wmv"), "compat_wmv"},
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate.path); err != nil {
			continue
		}
		mci("stop kongintro")
		mci("close kongintro")
		showWindow.Call(introVideoHwnd, SW_HIDE)

		// Let Windows choose the correct installed playback filter instead of forcing
		// a hidden/pre-window mpegvideo device before the game exists.
		if !mci(fmt.Sprintf(`open "%s" alias kongintro`, candidate.path)) {
			continue
		}
		if !mci(fmt.Sprintf("window kongintro handle %d", introVideoHwnd)) {
			mci("close kongintro")
			continue
		}
		mci(fmt.Sprintf("put kongintro destination at 0 0 %d %d", w, hgt))
		// Audio is deliberately supplied by the game's existing waveOut SFX bus so
		// the intro sound does not depend on the selected video decoder.
		mci("setaudio kongintro volume to 0")
		showWindow.Call(introVideoHwnd, SW_SHOW)
		if !mci("play kongintro from 0") {
			mci("close kongintro")
			showWindow.Call(introVideoHwnd, SW_HIDE)
			continue
		}
		introVideoPlaying = true
		logRuntimeEvent("intro_overlay", "playing_"+candidate.kind)
		return true
	}

	showWindow.Call(introVideoHwnd, SW_HIDE)
	logRuntimeEvent("intro_overlay", "all_video_decoders_failed")
	return false
}

func stopKongIntroVideo() {
	mci("stop kongintro")
	mci("close kongintro")
	introVideoPlaying = false
	if introVideoHwnd != 0 {
		showWindow.Call(introVideoHwnd, SW_HIDE)
	}
}

func beginIntro(h uintptr) {
	state = StateIntro
	introRechamberPlayed = false
	introVideoPlaying = false

	// v457: video belongs to the dedicated splash window, never the real game
	// window. The game is already initialised but remains genuinely hidden.
	started := false
	if introSplashHwnd != 0 {
		started = startKongIntroVideo(introSplashHwnd)
	}
	introStart = time.Now()
	playKongIntroSound()
	introRechamberPlayed = true
	if !started {
		// Decoder failure leaves the dedicated splash solid black. It cannot expose
		// or block startup of the hidden game window.
		introVideoPlaying = false
	}
	setCursor.Call(0)
	setTimer.Call(h, TIMER_INTRO, 16, 0)
	if introSplashHwnd != 0 {
		invalidateRect.Call(introSplashHwnd, 0, 0)
	}
}

func finishIntro(h uintptr) {
	killTimer.Call(h, TIMER_INTRO)
	stopKongIntroVideo()
	introStart = time.Time{}
	introRechamberPlayed = false
	state = StateWaiting

	if particleEpoch.IsZero() {
		particleEpoch = time.Now()
	}
	setTimer.Call(h, TIMER_PARTICLES, particleTimerInterval(), 0)
	setTimer.Call(h, TIMER_LIVE, 16, 0)
	setTimer.Call(h, TIMER_LIVE_SYSTEMS, 250, 0)
	liveNextAmbientAt = time.Now().Add(8 * time.Second)
	go fetchGlobalLeaderboard()
	go fetchLiveFeed()

	startBossMusic()
	applyAudioVolumes()

	// The destination interface was prepared before the overlay began, so there is
	// no menu-construction hitch here; this simply leaves that interface active.
	invalidateRect.Call(h, 0, 0)
}

func ensureParticleResources(hdc uintptr) bool {
	const sprite = 9
	colors := [3][3]byte{
		{20, 72, 158},
		{28, 139, 231},
		{132, 219, 255},
	}

	for i := 0; i < 3; i++ {
		if particleDCs[i] != 0 && particleBmps[i] != 0 {
			continue
		}
		dc, _, _ := createCompatibleDC.Call(hdc)
		if dc == 0 {
			return false
		}

		var bits uintptr
		bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{
			BiSize:  uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
			BiWidth: sprite, BiHeight: -sprite,
			BiPlanes: 1, BiBitCount: 32, BiCompression: BI_RGB,
		}}
		bmp, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
		if bmp == 0 || bits == 0 {
			deleteDC.Call(dc)
			return false
		}
		old, _, _ := selectObject.Call(dc, bmp)

		pix := unsafe.Slice((*byte)(unsafe.Pointer(bits)), sprite*sprite*4)
		center := float64(sprite-1) / 2
		radius := center + 0.2
		r, g, b := colors[i][0], colors[i][1], colors[i][2]
		for y := 0; y < sprite; y++ {
			for x := 0; x < sprite; x++ {
				dx := float64(x) - center
				dy := float64(y) - center
				d := math.Sqrt(dx*dx+dy*dy) / radius
				a := 0.0
				if d < 1 {
					a = math.Pow(1-d, 1.65)
				}
				alpha := byte(math.Round(255 * a))
				o := (y*sprite + x) * 4
				pix[o+0] = byte(uint16(b) * uint16(alpha) / 255)
				pix[o+1] = byte(uint16(g) * uint16(alpha) / 255)
				pix[o+2] = byte(uint16(r) * uint16(alpha) / 255)
				pix[o+3] = alpha
			}
		}

		particleDCs[i] = dc
		particleBmps[i] = bmp
		particleOlds[i] = old
	}
	return true
}

func releaseParticleResources() {
	for i := 0; i < 3; i++ {
		if particleDCs[i] != 0 {
			if particleOlds[i] != 0 {
				selectObject.Call(particleDCs[i], particleOlds[i])
			}
			if particleBmps[i] != 0 {
				deleteObject.Call(particleBmps[i])
			}
			deleteDC.Call(particleDCs[i])
		}
		particleDCs[i] = 0
		particleBmps[i] = 0
		particleOlds[i] = 0
	}
}

func ensureEnduranceBackgroundResources(hdc uintptr) bool {
	const srcW = 1942
	const srcH = 809
	if enduranceBgDC != 0 && enduranceBgBmp != 0 {
		return true
	}
	if textureRoot == "" {
		return false
	}
	bgData, err := os.ReadFile(filepath.Join(textureRoot, "endurance_background.bgra"))
	if err != nil || len(bgData) < srcW*srcH*4 {
		return false
	}

	dc, _, _ := createCompatibleDC.Call(hdc)
	if dc == 0 {
		return false
	}
	var bits uintptr
	bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{
		BiSize:  uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth: srcW, BiHeight: -srcH,
		BiPlanes: 1, BiBitCount: 32, BiCompression: BI_RGB,
	}}
	bmp, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		deleteDC.Call(dc)
		return false
	}
	old, _, _ := selectObject.Call(dc, bmp)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(bits)), srcW*srcH*4), bgData)
	enduranceBgDC = dc
	enduranceBgBmp = bmp
	enduranceBgOld = old
	return true
}

func releaseEnduranceBackgroundResources() {
	if enduranceBgDC != 0 {
		if enduranceBgOld != 0 {
			selectObject.Call(enduranceBgDC, enduranceBgOld)
		}
		if enduranceBgBmp != 0 {
			deleteObject.Call(enduranceBgBmp)
		}
		deleteDC.Call(enduranceBgDC)
	}
	enduranceBgDC = 0
	enduranceBgBmp = 0
	enduranceBgOld = 0
}

func ensureArenaBackgroundResources(hdc uintptr) bool {
	const srcW = 1200
	const srcH = 400
	if arenaBgDC != 0 && arenaBgBmp != 0 {
		return true
	}

	bgData := readExternalBytes("backgrounds", "arena_scrolling_bg.bgra")
	if len(bgData) < srcW*srcH*4 {
		return false
	}

	dc, _, _ := createCompatibleDC.Call(hdc)
	if dc == 0 {
		return false
	}
	var bits uintptr
	bmi := BITMAPINFO{BmiHeader: BITMAPINFOHEADER{
		BiSize:  uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth: srcW, BiHeight: -srcH,
		BiPlanes: 1, BiBitCount: 32, BiCompression: BI_RGB,
	}}
	bmp, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		deleteDC.Call(dc)
		return false
	}
	old, _, _ := selectObject.Call(dc, bmp)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(bits)), srcW*srcH*4), bgData)

	arenaBgDC = dc
	arenaBgBmp = bmp
	arenaBgOld = old
	return true
}

func releaseArenaBackgroundResources() {
	if arenaBgDC != 0 {
		if arenaBgOld != 0 {
			selectObject.Call(arenaBgDC, arenaBgOld)
		}
		if arenaBgBmp != 0 {
			deleteObject.Call(arenaBgBmp)
		}
		deleteDC.Call(arenaBgDC)
	}
	arenaBgDC = 0
	arenaBgBmp = 0
	arenaBgOld = 0
}

var visualElapsedFrozenAt float64
var visualElapsedFreezeStarted time.Time

func enduranceVisualElapsedSeconds() float64 {
	if particleEpoch.IsZero() {
		particleEpoch = time.Now()
	}
	now := time.Now()
	if state == StateFailed || state == StateResult {
		if visualElapsedFreezeStarted.IsZero() {
			visualElapsedFrozenAt = now.Sub(particleEpoch).Seconds()
			visualElapsedFreezeStarted = now
		}
		return visualElapsedFrozenAt
	}
	if !visualElapsedFreezeStarted.IsZero() {
		particleEpoch = particleEpoch.Add(now.Sub(visualElapsedFreezeStarted))
		visualElapsedFreezeStarted = time.Time{}
	}
	return now.Sub(particleEpoch).Seconds()
}

func backgroundMotionEnabled() bool {
	// MOVING BACKGROUND is the single authoritative switch for scrolling
	// arena/world backdrops. Reduced Motion controls other ambient/UI animation
	// (for example the Starbase Singularity rotation) but must not silently
	// override a player who explicitly turned background scrolling back ON.
	// Artwork is always drawn; OFF freezes only the scrolling phase.
	return gameMeta.MovingBackground
}

func drawScrollingArenaBackground(hdc uintptr, w, hgt int32) {
	ar := arenaRect(w, hgt)
	if ar.Right <= ar.Left || ar.Bottom <= ar.Top {
		return
	}

	saved, _, _ := saveDC.Call(hdc)
	intersectClipRect.Call(hdc, uintptr(ar.Left), uintptr(ar.Top), uintptr(ar.Right), uintptr(ar.Bottom))
	defer func() {
		if saved != 0 {
			restoreDC.Call(hdc, saved)
		}
	}()

	if enduranceActive() {
		const srcW = 1942
		const srcH = 809
		if !ensureEnduranceBackgroundResources(hdc) {
			return
		}
		if particleEpoch.IsZero() {
			particleEpoch = time.Now()
		}
		dstW := ar.Right - ar.Left
		dstH := ar.Bottom - ar.Top
		bgSpeed := 18.0 + 12.0*enduranceWorldDepth(enduranceProgressDistance())
		phase := 0.0
		if backgroundMotionEnabled() {
			phase = math.Mod(enduranceAmbientClockNow()*bgSpeed, float64(dstW))
		}
		// Keep normal Endurance background brightness identical before and after
		// START. Transparency is reserved for the intentional Warp pulse.
		blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)
		for k := 0; k < 2; k++ {
			x := ar.Left - int32(math.Round(phase)) + int32(k)*dstW
			alphaBlend.Call(
				hdc,
				uintptr(x), uintptr(ar.Top),
				uintptr(dstW), uintptr(dstH),
				enduranceBgDC, 0, 0, srcW, srcH, blend,
			)
		}
		return
	}

	const srcW = 1200
	const srcH = 400
	if !ensureArenaBackgroundResources(hdc) {
		return
	}
	if particleEpoch.IsZero() {
		particleEpoch = time.Now()
	}

	dstH := ar.Bottom - ar.Top
	dstW := int32(float64(dstH) * float64(srcW) / float64(srcH))
	if dstW < ar.Right-ar.Left {
		dstW = ar.Right - ar.Left
	}

	startOnRight := false
	if len(path) > 0 {
		startOnRight = path[0].X > float64(ar.Left+ar.Right)/2
	}
	speed := 13.5
	phase := 0.0
	if backgroundMotionEnabled() {
		elapsed := enduranceVisualElapsedSeconds()
		phase = math.Mod(elapsed*speed, float64(dstW))
		if phase < 0 {
			phase += float64(dstW)
		}
	}
	// Precision background is intentionally rendered at full opacity for maximum
	// path readability and visual consistency.
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(255)<<16 | uint32(AC_SRC_ALPHA)<<24)

	var base float64
	if startOnRight {
		base = float64(ar.Left) - float64(dstW) + phase
	} else {
		base = float64(ar.Left) - phase
	}
	for k := -1; k <= 3; k++ {
		x := int32(math.Round(base + float64(k)*float64(dstW)))
		if x >= ar.Right || x+dstW <= ar.Left {
			continue
		}
		alphaBlend.Call(hdc, uintptr(x), uintptr(ar.Top), uintptr(dstW), uintptr(dstH), arenaBgDC, 0, 0, srcW, srcH, blend)
	}
}

func drawArenaParticles(hdc uintptr, w, hgt int32) {
	const sprite = 9
	ar := arenaRect(w, hgt)
	if ar.Right <= ar.Left || ar.Bottom <= ar.Top || gameMeta.ParticleQuality == 0 {
		return
	}
	if !ensureParticleResources(hdc) {
		return
	}
	if particleEpoch.IsZero() {
		particleEpoch = time.Now()
	}
	arenaW := float64(ar.Right - ar.Left)
	arenaH := float64(ar.Bottom - ar.Top)

	if enduranceActive() {
		t := enduranceParticleClockNow()
		count := 190
		if gameMeta.ParticleQuality == 1 {
			count = 110
		}
		depth := enduranceWorldDepth(enduranceProgressDistance())
		if gameMeta.ParticleQuality == 1 {
			count += int(28 * depth)
		} else {
			count += int(52 * depth)
		}
		warpVisual := enduranceParticleSpeedMultiplierNow()
		if warpVisual > 1.05 {
			if gameMeta.ParticleQuality == 1 {
				count = 190
			} else {
				count = 340
			}
		}
		for i := 0; i < count; i++ {
			seedX := math.Mod(math.Abs(math.Sin(float64(i)*12.731+0.4)*43758.5453), 1)
			seedY := math.Mod(math.Abs(math.Sin(float64(i)*41.117+3.2)*24634.6345), 1)
			layer := i % 3
			speed := 38.0 + float64((i*19)%74)
			travel := math.Mod(seedX*arenaW+t*speed, arenaW)
			x := float64(ar.Right) - travel
			y := float64(ar.Top) + seedY*arenaH
			length := float64(7+(i*5)%18) * (1.0 + 0.30*depth)
			if warpVisual > 1.0 {
				length *= 1.0 + 0.78*(warpVisual-1.0)
			}
			thickness := 2.0
			if layer == 1 {
				length += 5
				thickness = 2.2
			} else if layer == 2 {
				length += 9
				thickness = 2.8
			}
			alpha := byte(72)
			if layer == 1 {
				alpha = 107
			}
			if layer == 2 {
				alpha = 148
			}
			blend := uintptr(uint32(AC_SRC_OVER) | uint32(alpha)<<16 | uint32(AC_SRC_ALPHA)<<24)
			alphaBlend.Call(
				hdc,
				uintptr(int32(math.Round(x-length))),
				uintptr(int32(math.Round(y-thickness/2))),
				uintptr(int32(math.Max(1, math.Round(length)))),
				uintptr(int32(math.Max(1, math.Round(thickness)))),
				particleDCs[layer], 0, 0, sprite, sprite, blend,
			)
		}
		return
	}

	t := enduranceVisualElapsedSeconds()
	particleCount := 512
	if gameMeta.ParticleQuality == 1 {
		particleCount = 216
	}
	startOnRight := false
	if len(path) > 0 {
		startOnRight = path[0].X > float64(ar.Left+ar.Right)/2
	}

	for i := 0; i < particleCount; i++ {
		seedX := math.Mod(math.Abs(math.Sin(float64(i)*12.731+0.4)*43758.5453), 1)
		seedY := math.Mod(math.Abs(math.Sin(float64(i)*41.117+3.2)*24634.6345), 1)
		layer := i % 3
		speed := 24.0 + float64((i*17)%52)
		if layer == 0 {
			speed *= 0.62
		} else if layer == 2 {
			speed *= 1.20
		}
		travel := math.Mod(seedX*arenaW+t*speed, arenaW)
		x := float64(ar.Left) + travel
		if startOnRight {
			x = float64(ar.Right) - travel
		}
		y := float64(ar.Top) + seedY*arenaH
		if x < float64(ar.Left)+10 || x > float64(ar.Right)-10 || y < float64(ar.Top)+5 || y > float64(ar.Bottom)-5 {
			continue
		}

		var alpha byte
		switch layer {
		case 0:
			alpha = byte(51 + (i*17)%82)
		case 1:
			alpha = byte(77 + (i*23)%103)
		default:
			alpha = byte(102 + (i*29)%103)
		}
		length := int32(8 + (i*7)%15)
		thickness := int32(2)
		if layer == 1 {
			length += 5
			thickness = 3
		} else if layer == 2 {
			length += 10
			thickness = 3
		}
		if i%17 == 0 {
			length += 8
		}
		ix := int32(math.Round(x))
		iy := int32(math.Round(y))
		dstX := ix
		if startOnRight {
			dstX = ix - length
		}
		blend := uintptr(uint32(AC_SRC_OVER) | uint32(alpha)<<16 | uint32(AC_SRC_ALPHA)<<24)
		alphaBlend.Call(hdc, uintptr(dstX), uintptr(iy-thickness/2), uintptr(length), uintptr(thickness), particleDCs[layer], 0, 0, sprite, sprite, blend)
	}
}

func fillPolygon(hdc uintptr, pts []POINT, color uintptr) {
	if len(pts) < 3 {
		return
	}
	brush, _, _ := createSolidBrush.Call(color)
	if brush == 0 {
		return
	}
	old, _, _ := selectObject.Call(hdc, brush)
	polygon.Call(
		hdc,
		uintptr(unsafe.Pointer(&pts[0])),
		uintptr(len(pts)),
	)
	selectObject.Call(hdc, old)
	deleteObject.Call(brush)
}

func drawDifficultyMeter(hdc uintptr, w, hgt int32) {
	left := sx(1335, w)
	right := sx(1513, w)
	top := sy(98, hgt)
	bottom := sy(116, hgt)

	fillSolidRect(hdc, RECT{left + 14, top, right - 14, bottom}, rgb(4, 12, 30))
	tier := activeDifficultyIndex()
	if enduranceActive() {
		if enduranceProgressDistance() < 140 {
			tier = 1
		} else if enduranceProgressDistance() < 340 {
			tier = 2
		} else {
			tier = 3
		}
	}
	fillCount := []int{2, 4, 6, 7}[tier]
	active := diffs[tier].color
	if adaptiveMode {
		active = rgb(39, 210, 255)
	}

	il := left + sx(22, w)
	ir := right - sx(22, w)
	gap := sx(4, w)
	sw := (ir - il - gap*6) / 7
	for i := 0; i < 7; i++ {
		x := il + int32(i)*(sw+gap)
		c := rgb(45, 57, 76)
		if i < fillCount {
			c = active
		}
		pts := []POINT{
			{x + sx(3, w), top + sy(3, hgt)},
			{x + sw, top + sy(3, hgt)},
			{x + sw - sx(3, w), bottom - sy(3, hgt)},
			{x, bottom - sy(3, hgt)},
		}
		fillPolygon(hdc, pts, c)
	}
}

func drawUpdatedControlsHint(hdc uintptr, w, hgt int32) {
	// v62: lower-right space is now the Quick Access dock.
}

func drawKongGameWatermark(hdc uintptr, w, hgt int32) {
	if hudTinyFont == 0 {
		return
	}
	old, _, _ := selectObject.Call(hdc, hudTinyFont)
	setBkMode.Call(hdc, TRANSPARENT)
	// Low-contrast watermark: visually subdued against the navy lower deck.
	setTextColor.Call(hdc, rgb(55, 78, 105))
	txt := "Kong Game ©"
	sz := textPixelSize(hdc, hudTinyFont, txt)
	textOut(hdc, w-sx(14, w)-sz.Cx, hgt-sy(20, hgt), txt)
	selectObject.Call(hdc, old)
}

func drawGameFrame(hdc uintptr, w, hgt int32) {
	if survivalActive() {
		drawSurvivalGameFrame(hdc, w, hgt)
		return
	}
	setBkMode.Call(hdc, TRANSPARENT)
	if !failureOverlaySpritesPrewarmed {
		prewarmFailureOverlaySprites(hdc)
	}
	drawUIBase(hdc, w, hgt)
	setBkMode.Call(hdc, TRANSPARENT)
	drawUpdatedControlsHint(hdc, w, hgt)

	if state == StateFailed {
		r := arenaRect(w, hgt)
		fillSolidRect(hdc, r, rgb(255, 230, 233))
		drawPlayfieldPattern(hdc, w, hgt)
	}

	d2dPlaying := d2dReady && d2dChildVisible && enduranceActive() && state == StatePlaying
	if !d2dPlaying {
		drawScrollingArenaBackground(hdc, w, hgt)
		drawArenaParticles(hdc, w, hgt)
		drawPathInsideArena(hdc, w, hgt)
		drawEnduranceBlocks(hdc, w, hgt)
		drawHitFeedback(hdc)
	}
	drawPlayfieldBorder(hdc, w, hgt)
	drawGlobalAnnouncementBar(hdc, w, hgt)

	dark := rgb(4, 27, 72)
	cyan := rgb(58, 224, 255)
	white := rgb(250, 252, 255)

	// Value wells use the EXACT same horizontal inset as the white title labels above.
	hudCards := []RECT{
		{sx(585, w), sy(7, hgt), sx(755, w), sy(124, hgt)},
		{sx(770, w), sy(7, hgt), sx(940, w), sy(124, hgt)},
		{sx(955, w), sy(7, hgt), sx(1125, w), sy(124, hgt)},
		{sx(1140, w), sy(7, hgt), sx(1320, w), sy(124, hgt)},
		{sx(1335, w), sy(7, hgt), sx(1513, w), sy(124, hgt)},
	}
	for _, card := range hudCards {
		well := RECT{
			card.Left + 14,
			sy(53, hgt),
			card.Right - 14,
			sy(119, hgt),
		}
		fillSolidRect(hdc, well, dark)
	}

	if hudStatFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudStatFont)
		setTextColor.Call(hdc, cyan)
		if enduranceActive() {
			textOut(hdc, sx(621, w), sy(70, hgt), fmt.Sprintf("%.1fs", updateRealTimeClock()))
		} else {
			textOut(hdc, sx(621, w), sy(70, hgt), fmt.Sprintf("%.1fs", updateRealTimeClock()))
		}
		textOut(hdc, sx(811, w), sy(70, hgt), fmt.Sprint(score))
		setTextColor.Call(hdc, white)
		if enduranceActive() {
			textOut(hdc, sx(996, w), sy(70, hgt), fmt.Sprintf("%.0fm", enduranceDistance))
		} else {
			textOut(hdc, sx(996, w), sy(70, hgt), fmt.Sprint(streak))
		}
		selectObject.Call(hdc, old)
	}

	// BEST shows BOTH highest score and highest streak.
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setTextColor.Call(hdc, cyan)
		if enduranceActive() {
			textOut(hdc, sx(1167, w), sy(75, hgt), fmt.Sprintf("%.0fm", gameMeta.BestEnduranceDistance))
		} else {
			textOut(hdc, sx(1167, w), sy(75, hgt), fmt.Sprintf("%d / x%d", bestScore, bestStreak))
		}
		selectObject.Call(hdc, old)
	}

	// Difficulty has a dedicated semantic color.
	if enduranceActive() {
		if hudSmallFont != 0 {
			old, _, _ := selectObject.Call(hdc, hudSmallFont)
			setTextColor.Call(hdc, rgb(63, 225, 255))
			centeredTextOut(hdc, sx(1349, w), sx(1499, w), sy(69, hgt), "ENDURANCE")
			selectObject.Call(hdc, old)
		}
	} else if hudStatFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudStatFont)
		diffLabel, diffColor := difficultyDisplay()
		setTextColor.Call(hdc, diffColor)
		centeredTextOut(hdc, sx(1349, w), sx(1499, w), sy(62, hgt), diffLabel)
		selectObject.Call(hdc, old)
	}

	drawDifficultyMeter(hdc, w, hgt)

	// Advanced bottom HUD replaces the original simplified mission/status copy.

	drawAdvancedMissionPanel(hdc, w, hgt)
	drawAdvancedObjectivePanel(hdc, w, hgt)
	drawQuickAccessDock(hdc, w, hgt)
	drawQuickBugReport(hdc, w, hgt)
	drawQuickSupportDev(hdc, w, hgt)
	drawQuickDiscordLogin(hdc, w, hgt)
	drawModeSelectorDropdown(hdc, w, hgt)
	drawHUDLayoutEditor(hdc, w, hgt)
	drawKongGameWatermark(hdc, w, hgt)

	drawResultOverlay(hdc, w, hgt)
	drawLeaderboardOverlay(hdc, w, hgt)
	drawGlobalLeaderboardOverlay(hdc, w, hgt)
	drawRemoteProfileOverlay(hdc, w, hgt)
	drawProfileSkinGallery(hdc, w, hgt)
	drawDifficultyLockedOverlay(hdc, w, hgt)
	if survivalActive() {
		drawSurvivalSectionLockedOverlay(hdc, w, hgt)
		drawSurvivalMonsterGuide(hdc, w, hgt)
	}
	drawGarageOverlay(hdc, w, hgt)
	drawSpaceCacheOverlay(hdc, w, hgt)
	drawAFKSingularityOverlay(hdc, w, hgt)
	drawDeveloperConsoleOverlay(hdc, w, hgt)
	drawNameEntryOverlay(hdc, w, hgt)
	drawMainMenuOverlay(hdc, w, hgt)
	drawProfileOverlay(hdc, w, hgt)
	drawSettingsOverlay(hdc, w, hgt)
	drawTutorialOverlay(hdc, w, hgt)
	drawAchievementsOverlay(hdc, w, hgt)
	drawReleaseNotesOverlay(hdc, w, hgt)
	drawAchievementToast(hdc, w, hgt)
	drawRewardToast(hdc, w, hgt)
	drawOverlayTransitionAccent(hdc, w, hgt)

	// Cursor treatment is mode-specific: Precision uses the player's configured crosshair;
	// Endurance keeps its spaceship cursor; Survival owns its fixed white ring crosshair.
	if !d2dPlaying && overlayMode == OverlayNone && !pointInMenuUI(cursorPos, w, hgt) {
		if enduranceActive() {
			drawEnduranceGDIShipCursor(hdc, w, hgt)
		} else {
			drawYellowCrosshair(hdc)
		}
	}

	drawFailedOverlay(hdc, w, hgt)
}

func drawFrame(hdc uintptr, w, hgt int32) {
	drawGameFrame(hdc, w, hgt)
	if state == StateIntro {
		// While the native video child is visible it sits over this already-rendered
		// game frame. After the movie ends, reveal the game through a short black
		// fade instead of pausing on the last video frame.
		ms := float64(introElapsed()) / float64(time.Millisecond)
		if !introVideoPlaying {
			if ms < 3600 {
				overlayBlack(hdc, w, hgt, 255)
			} else if ms < 3740 {
				p := (ms - 3600) / 140.0
				if p < 0 {
					p = 0
				}
				if p > 1 {
					p = 1
				}
				overlayBlack(hdc, w, hgt, byte(math.Round(255*(1-p))))
			}
		}
		return
	}
}

func releaseBackBuffer() {
	if backBufferDC != 0 {
		if backBufferOld != 0 {
			selectObject.Call(backBufferDC, backBufferOld)
		}
		if backBufferBmp != 0 {
			deleteObject.Call(backBufferBmp)
		}
		deleteDC.Call(backBufferDC)
	}
	backBufferDC = 0
	backBufferBmp = 0
	backBufferOld = 0
	backBufferW = 0
	backBufferH = 0
}

func ensureBackBuffer(hdc uintptr, w, h int32) bool {
	if backBufferDC != 0 && backBufferBmp != 0 && backBufferW == w && backBufferH == h {
		return true
	}
	releaseBackBuffer()

	dc, _, _ := createCompatibleDC.Call(hdc)
	if dc == 0 {
		return false
	}
	bmp, _, _ := createCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	if bmp == 0 {
		deleteDC.Call(dc)
		return false
	}
	old, _, _ := selectObject.Call(dc, bmp)

	backBufferDC = dc
	backBufferBmp = bmp
	backBufferOld = old
	backBufferW = w
	backBufferH = h
	return true
}

func paint(h uintptr) {
	var ps PAINTSTRUCT
	hdc, _, _ := beginPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer endPaint.Call(h, uintptr(unsafe.Pointer(&ps)))

	w, hgt := getClient(h)
	if w <= 0 || hgt <= 0 {
		return
	}

	if !ensureBackBuffer(hdc, w, hgt) {
		drawFrame(hdc, w, hgt)
		return
	}
	hudPerfStart := time.Now()
	drawFrame(backBufferDC, w, hgt)
	drawDeveloperPerfOverlay(backBufferDC, w, hgt)
	perfMeasureHUD(hudPerfStart)

	// Respect the invalidated paint region. During Endurance, HUD refreshes no
	// longer copy the full 1536x1024 backbuffer to the screen every time.
	rc := ps.RcPaint
	if rc.Right <= rc.Left || rc.Bottom <= rc.Top {
		rc = RECT{0, 0, w, hgt}
	}
	bitBlt.Call(
		hdc,
		uintptr(rc.Left), uintptr(rc.Top),
		uintptr(rc.Right-rc.Left), uintptr(rc.Bottom-rc.Top),
		backBufferDC,
		uintptr(rc.Left), uintptr(rc.Top),
		SRCCOPY,
	)
}

func overlayDismissPanelRect(w, hgt int32) (RECT, bool) {
	switch overlayMode {
	case OverlayLeaderboard:
		ar := arenaRect(w, hgt)
		return RECT{ar.Left + sx(55, w), ar.Top + sy(32, hgt), ar.Right - sx(55, w), ar.Bottom - sy(26, hgt)}, true
	case OverlayGlobalLeaderboard:
		left, right, top, bottom, _, _, _, _ := globalLeaderboardGeometry(w, hgt)
		return RECT{left, top, right, bottom}, true
	case OverlayMainMenu:
		return centeredPanel(w, hgt, 980, 800), true
	case OverlayProfile:
		return centeredPanel(w, hgt, 1180, 840), true
	case OverlaySettings:
		return settingsPanelRect(w, hgt), true
	case OverlayTutorial:
		return tutorialPanelRect(w, hgt), true
	case OverlayAchievements:
		return achievementPanelRect(w, hgt), true
	case OverlayReleaseNotes:
		return centeredPanel(w, hgt, 940, 690), true
	case OverlayRemoteProfile:
		return centeredPanel(w, hgt, 1180, 840), true
	case OverlayProfileSkins:
		return profileSkinPanelRect(w, hgt), true
	case OverlaySurvivalMonsterGuide:
		return survivalGuidePanelRect(w, hgt), true
	case OverlayGarage:
		return garagePanelRect(w, hgt), true
	}
	return RECT{}, false
}

func dismissOverlayFromOutsideClick() {
	switch overlayMode {
	case OverlaySettings:
		if starbaseSettingsReturn {
			starbaseSettingsReturn = false
			setOverlay(OverlayAFKSingularity)
			return
		}
		closeOverlay()
	case OverlayRemoteProfile:
		setOverlay(OverlayGlobalLeaderboard)
	case OverlayProfileSkins:
		setOverlay(OverlayRemoteProfile)
	default:
		closeOverlay()
	}
}

// introSplashWndProc owns the dedicated startup window. It deliberately does
// almost nothing: paint solid black, hide the cursor, and consume all input. The
// actual game window is created hidden behind it, so no game frame can reach the
// desktop before the cinematic begins.
func introSplashWndProc(h uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_SETCURSOR:
		setCursor.Call(0)
		return 1
	case WM_NCHITTEST:
		// Treat the whole splash as inert client space. This disables caption,
		// close/minimise hit targets while the non-interactive intro is active.
		return 1 // HTCLIENT
	case WM_LBUTTONDOWN, WM_LBUTTONUP, WM_RBUTTONDOWN, WM_RBUTTONUP, WM_MOUSEWHEEL, WM_MOUSEMOVE, WM_KEYDOWN, WM_CHAR:
		return 0
	case WM_CLOSE:
		return 0
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := beginPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			w, hgt := getClient(h)
			fillSolidRect(hdc, RECT{0, 0, w, hgt}, rgb(0, 0, 0))
		}
		endPaint.Call(h, uintptr(unsafe.Pointer(&ps)))
		return 0
	}
	r, _, _ := defWindowProcW.Call(h, uintptr(msg), wParam, lParam)
	return r
}

func shouldShowWindowsCursor(h uintptr, p FPoint, w, hgt int32) bool {
	// v438: native pointer is for interfaces/HUDs, not active game-space control.
	// Every overlay is an interface. Result/failure screens and HUD layout editing
	// are interfaces too. Otherwise only the non-arena HUD region shows Windows.
	if overlayMode != OverlayNone || hudLayoutEditorActive || state == StateFailed || state == StateResult {
		return true
	}
	return !pointInArena(p, w, hgt)
}

func wndProc(h uintptr, msg uint32, wParam, lParam uintptr) (ret uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			recordRecoveredPanic(fmt.Sprintf("wndProc_msg_%d", msg), recovered)
			ret = 0
		}
	}()

	// The opening cinematic is a modal visual overlay. No mouse button, wheel,
	// keyboard or text input is allowed to reach the game beneath it.
	if state == StateIntro {
		switch msg {
		case WM_LBUTTONDOWN, WM_LBUTTONUP, WM_RBUTTONDOWN, WM_RBUTTONUP, WM_MOUSEWHEEL, WM_KEYDOWN, WM_CHAR:
			return 0
		}
	}

	switch msg {
	case WM_SURVIVAL_RESPAWN:
		survivalRespawnProcessMainThread(h)
		return 0
	case WM_MAIN_THREAD_TASK:
		processMainThreadTasks()
		return 0
	case WM_DESTROY:
		runtimeLifecycleSnapshot("wm_destroy")
		afkBankFlushOnExit()
		stopTransientGameplayAudio()
		killTimer.Call(h, TIMER_GAME)
		killTimer.Call(h, TIMER_FAIL_RESET)
		killTimer.Call(h, TIMER_RESULT_RESET)
		killTimer.Call(h, TIMER_LEVELUP)
		killTimer.Call(h, TIMER_INTRO)
		killTimer.Call(h, TIMER_PARTICLES)
		killTimer.Call(h, TIMER_UI)
		killTimer.Call(h, TIMER_LIVE)
		killTimer.Call(h, TIMER_LIVE_SYSTEMS)
		killTimer.Call(h, TIMER_STARBASE)
		// If Precision closes during an active run, keep the persisted marker intact.
		// The next launch consumes it and reports the streak break reliably; a goroutine
		// launched during WM_DESTROY can be terminated before the HTTP request completes.
		saveSessionPlaytime()
		authMu.Lock()
		if authServer != nil {
			_ = authServer.Close()
			authServer = nil
		}
		authMu.Unlock()
		releaseCapture.Call()
		releaseParticleResources()
		releaseHUDIconResources()
		releaseHUDTextureResources()
		releaseAFKSingularityRotationSurface()
		releaseRuntimeSpriteCache()
		releaseSurvivalGDIResources()
		releaseSolidPixelCache()
		releaseCachedBGRASprite(&powerupShieldSprite)
		releaseCachedBGRASprite(&powerupTimeSprite)
		releaseCachedBGRASprite(&hazardBlueSprite)
		releaseCachedBGRASprite(&hazardOrangeSprite)
		releaseArenaBackgroundResources()
		releaseEnduranceBackgroundResources()
		releaseEnduranceRailCache()
		releaseBackBuffer()
		releaseD2DResources()
		if hitFeedbackPen != 0 {
			deleteObject.Call(hitFeedbackPen)
			hitFeedbackPen = 0
		}
		shutdownAudio()
		postQuitMessage.Call(0)
		return 0

	case WM_ERASEBKGND:
		return 1

	case WM_SETCURSOR:
		// v438: show the Windows arrow anywhere the user is interacting with UI,
		// while leaving active gameplay arena control to Cursor Control's cursor.
		w, hgt := getClient(h)
		var cp POINT
		p := cursorPos
		if ok, _, _ := getCursorPos.Call(uintptr(unsafe.Pointer(&cp))); ok != 0 {
			screenToClient.Call(h, uintptr(unsafe.Pointer(&cp)))
			p = FPoint{X: float64(cp.X), Y: float64(cp.Y)}
		}
		if shouldShowWindowsCursor(h, p, w, hgt) {
			setCursor.Call(arrowCursor)
		} else {
			setCursor.Call(0)
		}
		return 1

	case WM_PAINT:
		paint(h)
		return 0

	case WM_LBUTTONUP:
		if afkTalentPanDragging {
			afkTalentPanDragging = false
			releaseCapture.Call()
			return 0
		}
		if survivalCheckpointScrollDragging {
			survivalCheckpointScrollDragging = false
			releaseCapture.Call()
			return 0
		}
		if settingsScrollbarDragging {
			settingsScrollbarDragging = false
			releaseCapture.Call()
			return 0
		}
		if afkTechScrollDragging {
			afkTechScrollDragging = false
			releaseCapture.Call()
			return 0
		}
		if afkExpeditionScrollDragging {
			afkExpeditionScrollDragging = false
			releaseCapture.Call()
			return 0
		}
		if settingsVolumeDrag != 0 {
			settingsVolumeDrag = 0
			releaseCapture.Call()
			saveGameMeta()
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if hudLayoutEditorActive && hudLayoutDragging {
			hudLayoutDragging = false
			releaseCapture.Call()
			saveHUDLayoutConfig()
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if achievementDragging {
			achievementDragging = false
			releaseCapture.Call()
			return 0
		}
		if survivalBoss3Active() && state == StatePlaying && overlayMode == OverlayNone {
			p := FPoint{float64(loword(lParam)), float64(hiword(lParam))}
			if survivalBoss3HandleRelease(h, p, false) {
				return 0
			}
		}
		return 0

	case WM_MOUSEWHEEL:
		if overlayMode == OverlayAFKSingularity && afkTechPanelOpen {
			w, hgt := getClient(h)
			if handleAFKTechLabWheel(hiword(wParam), w, hgt) {
				invalidateRect.Call(h, 0, 0)
				return 0
			}
		}
		if overlayMode == OverlayAFKSingularity && afkExpeditionPanelOpen {
			w, hgt := getClient(h)
			if handleAFKExpeditionWheel(hiword(wParam), w, hgt) {
				invalidateRect.Call(h, 0, 0)
				return 0
			}
		}
		if overlayMode == OverlayLeaderboard {
			w, hgt := getClient(h)
			scrollLocalLeaderboard(hiword(wParam), w, hgt)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if overlayMode == OverlayGlobalLeaderboard {
			w, hgt := getClient(h)
			scrollGlobalLeaderboard(hiword(wParam), w, hgt)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if overlayMode == OverlaySettings {
			w, hgt := getClient(h)
			delta := hiword(wParam)
			if delta < 0 {
				settingsScroll++
			} else if delta > 0 {
				settingsScroll--
			}
			clampSettingsScroll(w, hgt)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if overlayMode == OverlaySurvivalMonsterGuide {
			w, hgt := getClient(h)
			delta := hiword(wParam)
			step := sy(118, hgt)
			if delta < 0 {
				survivalGuideScroll += step
			} else if delta > 0 {
				survivalGuideScroll -= step
			}
			clampSurvivalGuideScroll(w, hgt)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if overlayMode == OverlayAchievements {
			w, hgt := getClient(h)
			delta := hiword(wParam)
			scrollAchievements(delta, w, hgt)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if survivalActive() && state == StateWaiting && overlayMode == OverlayNone {
			w, hgt := getClient(h)
			delta := hiword(wParam)
			step := sx(82, w)
			if delta < 0 {
				survivalCheckpointScroll += step
			} else if delta > 0 {
				survivalCheckpointScroll -= step
			}
			clampSurvivalCheckpointScroll(w, hgt)
			invalidateSurvivalHUD(h)
			return 0
		}
		if overlayMode == OverlayProfileSkins {
			w, hgt := getClient(h)
			delta := hiword(wParam)
			if profileCustomizeTab == 1 {
				if delta < 0 {
					profileSkinScroll++
				} else if delta > 0 {
					profileSkinScroll--
				}
				if profileSkinScroll < 0 {
					profileSkinScroll = 0
				}
				maxScroll := profileBannerMaxScroll(w, hgt)
				if profileSkinScroll > maxScroll {
					profileSkinScroll = maxScroll
				}
			}
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		return 0

	case WM_RBUTTONUP:
		if state == StateIntro {
			return 0
		}
		if survivalBoss3Active() && state == StatePlaying && overlayMode == OverlayNone {
			p := FPoint{float64(loword(lParam)), float64(hiword(lParam))}
			if survivalBoss3HandleRelease(h, p, true) {
				return 0
			}
		}
		return 0

	case WM_RBUTTONDOWN:
		if state == StateIntro {
			return 0
		}
		p := FPoint{float64(loword(lParam)), float64(hiword(lParam))}
		if survivalActive() && overlayMode == OverlayNone {
			survivalHandleClick(h, p, true)
			return 0
		}
		return 0

	case WM_LBUTTONDOWN:
		if state == StateIntro {
			return 0
		}
		p := FPoint{float64(loword(lParam)), float64(hiword(lParam))}
		recordPolishClick(p)
		w, hgt := getClient(h)

		if survivalRespawnHandleClick(h, p, w, hgt) {
			playUIButtonClickSound()
			return 0
		}

		// Survival Monster Guide owns its dedicated HUD button before generic HUD
		// controls. This prevents a customised/moved mode selector or quick-access
		// rectangle from consuming the same click and merely collapsing other buttons.
		if survivalActive() && state == StateWaiting && overlayMode == OverlayNone && pointInRect(p, survivalGuideButtonRect(w, hgt)) {
			survivalGuideScroll = 0
			modeSelectorOpen = false
			menuOpen = false
			playUIButtonClickSound()
			setOverlay(OverlaySurvivalMonsterGuide)
			return 0
		}

		// v356 QoL: SELECT MODE remains clickable while any front-end tab/overlay
		// is open. Clicking it closes the current screen and immediately opens the
		// three-mode chooser for fast re-entry into gameplay.
		if (state == StateWaiting || state == StateResult || (state == StatePlaying && survivalActive())) && pointInRect(p, enduranceModeButtonRect(w, hgt)) {
			// Survival can hand off directly to the mode selector. Abort only the
			// transient Survival run; do not create a result or submit a score.
			if state == StatePlaying && survivalActive() {
				resetToWaiting(h)
			}
			if state == StateResult {
				resetToWaiting(h)
			}
			playUIButtonClickSound()
			wasOverlayOpen := overlayMode != OverlayNone
			if wasOverlayOpen {
				closeOverlay()
			}
			if wasOverlayOpen {
				modeSelectorOpen = true
			} else {
				modeSelectorOpen = !modeSelectorOpen
			}
			menuOpen = false
			invalidateRect.Call(h, 0, 0)
			return 0
		}

		// v446: every modal/interface can be dismissed by clicking the backdrop.
		// Nested screens return to their parent; Starbase itself retains its own
		// management-panel dismissal rules inside handleAFKSingularityClick.
		if panel, ok := overlayDismissPanelRect(w, hgt); ok && !pointInRect(p, panel) {
			playUIButtonClickSound()
			dismissOverlayFromOutsideClick()
			invalidateRect.Call(h, 0, 0)
			return 0
		}

		if hudLayoutEditorActive && overlayMode == OverlayNone && state == StateWaiting {
			hit := hudLayoutHitTest(p, w, hgt)
			if hit >= 0 {
				hudLayoutSelected = hit
				r := hudLayoutRectByIndex(hit)
				px := screenToDesignX(int32(p.X), w)
				py := screenToDesignY(int32(p.Y), hgt)
				hudLayoutDragOffsetX = px - r.Left
				hudLayoutDragOffsetY = py - r.Top
				hudLayoutDragging = true
				setCapture.Call(h)
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			hudLayoutSelected = -1
			invalidateRect.Call(h, 0, 0)
			return 0
		}

		if overlayMode == OverlayDifficultyLocked {
			closeOverlay()
			return 0
		}
		if overlayMode == OverlaySurvivalSectionLocked {
			playUIButtonClickSound()
			closeOverlay()
			return 0
		}
		if overlayMode == OverlaySurvivalMonsterGuide {
			panel := survivalGuidePanelRect(w, hgt)
			if !pointInRect(p, panel) {
				playUIButtonClickSound()
				closeOverlay()
			}
			return 0
		}
		if overlayMode == OverlaySpaceCache {
			if spaceCacheOpened {
				closeOverlay()
			}
			return 0
		}
		if overlayMode == OverlayAFKSingularity {
			handleAFKSingularityClick(h, p, w, hgt)
			return 0
		}
		if overlayMode == OverlayGarage {
			panel := garagePanelRect(w, hgt)
			if !pointInRect(p, panel) {
				closeOverlay()
				return 0
			}
			for i, r := range garageTabRects(w, hgt) {
				if pointInRect(p, r) {
					garageTab = i
					garageNoticeText = ""
					garageNoticeUntil = time.Time{}
					playUIButtonClickSound()
					invalidateRect.Call(h, 0, 0)
					return 0
				}
			}
			if garageTab == 0 {
				cards := garageShipCardRects(w, hgt)
				for i, r := range cards {
					if pointInRect(p, r) {
						playUIButtonClickSound()
						id := garageShipOrder[i]
						if shipUnlocked(id) {
							gameMeta.SelectedShip = id
							saveGameMeta()
							requestPlayerProfileSync()
							status = fmt.Sprintf("Equipped %s", spaceShipDefs[id].Name)
							invalidateRect.Call(h, 0, 0)
						}
						return 0
					}
				}
			} else if garageTab == 1 {
				cards := garageFireCardRects(w, hgt)
				for slot, r := range cards {
					id := garageFireColorOrder[slot]
					if pointInRect(p, r) {
						playUIButtonClickSound()
						if fireColorUnlocked(id) {
							gameMeta.SelectedFireColor = id
							markGarageFireColorSelectionDirty()
							saveGameMeta()
							requestPlayerProfileSync()
							garageNoticeText = ""
							garageNoticeUntil = time.Time{}
							status = fmt.Sprintf("Equipped %s thruster fire", fireColorDefs[id].Name)
						} else if gameMeta.SpaceCoins >= fireColorDefs[id].Cost {
							gameMeta.SpaceCoins -= fireColorDefs[id].Cost
							unlockFireColor(id)
							gameMeta.SelectedFireColor = id
							markGarageFireColorSelectionDirty()
							saveGameMeta()
							requestPlayerProfileSync()
							garageNoticeText = ""
							garageNoticeUntil = time.Time{}
							status = fmt.Sprintf("Unlocked %s thruster fire", fireColorDefs[id].Name)
							playGarageBuySound()
						} else {
							garageNoticeText = "You do not have enough coins to buy this."
							garageNoticeUntil = time.Now().Add(2 * time.Second)
						}
						invalidateRect.Call(h, 0, 0)
						return 0
					}
				}
			} else {
				cards := garageFireSizeCardRects(w, hgt)
				for id, r := range cards {
					if pointInRect(p, r) {
						playUIButtonClickSound()
						if fireSizeUnlocked(id) {
							gameMeta.SelectedFireSize = id
							markGarageFireSizeSelectionDirty()
							saveGameMeta()
							requestPlayerProfileSync()
							garageNoticeText = ""
							garageNoticeUntil = time.Time{}
							status = fmt.Sprintf("Equipped %s thruster fire size", fireSizeDefs[id].Name)
						} else if gameMeta.SpaceCoins >= fireSizeDefs[id].Cost {
							gameMeta.SpaceCoins -= fireSizeDefs[id].Cost
							unlockFireSize(id)
							gameMeta.SelectedFireSize = id
							markGarageFireSizeSelectionDirty()
							saveGameMeta()
							requestPlayerProfileSync()
							garageNoticeText = ""
							garageNoticeUntil = time.Time{}
							status = fmt.Sprintf("Unlocked %s thruster fire size", fireSizeDefs[id].Name)
							playGarageBuySound()
						} else {
							garageNoticeText = "You do not have enough coins to buy this."
							garageNoticeUntil = time.Now().Add(2 * time.Second)
						}
						invalidateRect.Call(h, 0, 0)
						return 0
					}
				}
			}
			return 0
		}

		// v40 front-end screens.
		if overlayMode == OverlayDeveloperConsole {
			if wParam == VK_ESCAPE {
				closeOverlay()
				developerConsoleInput = ""
				developerConsoleStatus = ""
			}
			return 0
		}

		if overlayMode == OverlayMainMenu {
			if musicCreditsPopupOpen {
				if pointInRect(p, musicCreditsPopupRect(w, hgt)) {
					return 0
				}
				musicCreditsPopupOpen = false
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			if pointInRect(p, musicCreditsButtonRect(w, hgt)) {
				musicCreditsPopupOpen = true
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			if pointInRect(p, afkMainMenuRect(w, hgt)) {
				playUIButtonClickSound()
				setOverlay(OverlayAFKSingularity)
				return 0
			}
			rects := mainMenuRects(w, hgt)
			for i, r := range rects {
				if pointInRect(p, r) {
					playUIButtonClickSound()
					switch i {
					case 0:
						closeOverlay()
					case 1:
						beginDiscordLogin()
					case 2:
						achievementScroll = 0
						achievementShowcaseTarget = 0
						achievementShowcaseConfirmID = ""
						achievementShowcaseConfirmUntil = time.Time{}
						setOverlay(OverlayAchievements)
					case 3:
						setOverlay(OverlaySettings)
					case 4:
						tutorialPage = 0
						tutorialMode = -1
						setOverlay(OverlayTutorial)
					case 5:
						postQuitMessage.Call(0)
					}
					return 0
				}
			}
			if pointInRect(p, releaseNotesRect(w, hgt)) {
				playUIButtonClickSound()
				setOverlay(OverlayReleaseNotes)
			}
			return 0
		}
		if overlayMode == OverlayProfile {
			pr := centeredPanel(w, hgt, 1180, 840)
			if !pointInRect(p, pr) {
				setOverlay(OverlayMainMenu)
				return 0
			}
			if pointInRect(p, localToGlobalProfileButtonRect(w, hgt)) {
				playUIButtonClickSound()
				openOwnGlobalProfile()
				return 0
			}
			// Local Profile is intentionally view-only. Profile customization and
			// achievement showcase selection live in Global Profile/Achievements.
			return 0
		}
		if overlayMode == OverlayReleaseNotes {
			setOverlay(OverlayMainMenu)
			return 0
		}
		if overlayMode == OverlayAchievements {
			for i, fr := range achievementFilterRects(achievementPanelRect(w, hgt), w, hgt) {
				if pointInRect(p, fr) {
					playUIButtonClickSound()
					achievementFilter = i
					achievementScroll = 0
					clampAchievementScroll(w, hgt)
					invalidateRect.Call(h, 0, 0)
					return 0
				}
			}
			if a, ok := achievementAtPoint(p, w, hgt); ok && achievementUnlocked(a.ID) {
				playUIButtonClickSound()
				slot := achievementShowcaseTarget
				setAchievementShowcaseSlot(slot, a.ID)
				achievementShowcaseConfirmID = a.ID
				achievementShowcaseConfirmSlot = slot
				achievementShowcaseConfirmUntil = time.Now().Add(1800 * time.Millisecond)
				achievementShowcaseTarget = (slot + 1) % 3
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			track, thumb := achievementScrollbarRects(w, hgt)
			px, py := int32(p.X), int32(p.Y)
			if px >= thumb.Left && px <= thumb.Right && py >= thumb.Top && py <= thumb.Bottom {
				achievementDragging = true
				achievementDragOffset = py - thumb.Top
				setCapture.Call(h)
				return 0
			}
			if px >= track.Left && px <= track.Right && py >= track.Top && py <= track.Bottom {
				achievementScrollbarClick(py, w, hgt)
				invalidateRect.Call(h, 0, 0)
			}
			return 0
		}
		if overlayMode == OverlaySettings {
			track, thumb := settingsScrollbarRects(w, hgt)
			px, py := int32(p.X), int32(p.Y)
			if px >= thumb.Left && px <= thumb.Right && py >= thumb.Top && py <= thumb.Bottom && settingsMaxScroll(w, hgt) > 0 {
				settingsScrollbarDragging = true
				settingsScrollbarDragOffset = py - thumb.Top
				setCapture.Call(h)
				return 0
			}
			if px >= track.Left && px <= track.Right && py >= track.Top && py <= track.Bottom && settingsMaxScroll(w, hgt) > 0 {
				settingsScrollbarDragTo(py-(thumb.Bottom-thumb.Top)/2, w, hgt)
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			action, _ := settingsActionAtPoint(p, w, hgt)
			if action != settingsActionNone {
				playUIButtonClickSound()
				switch action {
				case settingsActionParticle:
					gameMeta.ParticleQuality = (gameMeta.ParticleQuality + 1) % 3
					killTimer.Call(h, TIMER_PARTICLES)
					setTimer.Call(h, TIMER_PARTICLES, particleTimerInterval(), 0)
				case settingsActionBackground:
					gameMeta.MovingBackground = !gameMeta.MovingBackground
				case settingsActionFPS:
					gameMeta.FPSMode = (gameMeta.FPSMode + 1) % 3
				case settingsActionReducedMotion:
					gameMeta.ReducedMotion = !gameMeta.ReducedMotion
				case settingsActionShake:
					gameMeta.ScreenShakeStrength = (gameMeta.ScreenShakeStrength + 1) % 4
				case settingsActionMusic:
					settingsVolumeDrag = 1
					setCapture.Call(h)
					updateSettingsVolumeFromPoint(1, p, w, hgt)
					invalidateRect.Call(h, 0, 0)
					return 0
				case settingsActionEffects:
					settingsVolumeDrag = 2
					setCapture.Call(h)
					updateSettingsVolumeFromPoint(2, p, w, hgt)
					invalidateRect.Call(h, 0, 0)
					return 0
				case settingsActionFailSound:
					gameMeta.FailureSound = (gameMeta.FailureSound + 1) % 5
					playFailureSoundPreview()
				case settingsActionFont:
					gameMeta.FontOverride = (gameMeta.FontOverride + 1) % len(uiFontFaces)
					rebuildUIFontHandles()
				case settingsActionHUDCorners:
					gameMeta.HUDCornerStyle = (gameMeta.HUDCornerStyle + 1) % 4
				case settingsActionHUDBackgroundTheme:
					gameMeta.HUDBackgroundTheme = (gameMeta.HUDBackgroundTheme + 1) % 6
				case settingsActionEXPAnim:
					gameMeta.EXPBarAnimation = (gameMeta.EXPBarAnimation + 1) % 3
				case settingsActionBossHPTheme:
					gameMeta.BossHPBarTheme = (gameMeta.BossHPBarTheme + 1) % 4
				case settingsActionHover:
					gameMeta.ButtonHoverEffect = (gameMeta.ButtonHoverEffect + 1) % 4
				case settingsActionAnnouncementTheme:
					gameMeta.AnnouncementTheme = (gameMeta.AnnouncementTheme + 1) % 6
				case settingsActionCrosshairStyle:
					gameMeta.CrosshairStyle = (gameMeta.CrosshairStyle + 1) % 5
				case settingsActionCrosshairSize:
					gameMeta.CrosshairSize = (gameMeta.CrosshairSize + 1) % 3
				case settingsActionCrosshairColour:
					gameMeta.CrosshairColor = (gameMeta.CrosshairColor + 1) % 7
				case settingsActionHitFeedback:
					hitFXEnabled = !hitFXEnabled
				case settingsActionShipHitbox:
					gameMeta.ShowShipHitbox = !gameMeta.ShowShipHitbox
				}
				saveGameMeta()
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			if !pointInRect(p, centeredPanel(w, hgt, 900, 840)) {
				if starbaseSettingsReturn {
					setOverlay(OverlayAFKSingularity)
					starbaseSettingsReturn = false
				} else {
					setOverlay(OverlayMainMenu)
				}
			}
			return 0
		}
		if overlayMode == OverlayDifficultyLocked {
			if wParam == VK_ESCAPE || wParam == VK_RETURN || wParam == VK_SPACE {
				closeOverlay()
			}
			return 0
		}
		if overlayMode == OverlaySurvivalSectionLocked {
			if wParam == VK_ESCAPE || wParam == VK_RETURN || wParam == VK_SPACE {
				closeOverlay()
			}
			return 0
		}
		if overlayMode == OverlaySpaceCache {
			if spaceCacheOpened && (wParam == VK_ESCAPE || wParam == VK_RETURN || wParam == VK_SPACE) {
				closeOverlay()
			}
			return 0
		}
		if overlayMode == OverlayTutorial {
			if tutorialMode < 0 {
				for i, tr := range tutorialModeCardRects(w, hgt) {
					if pointInRect(p, tr) {
						playUIButtonClickSound()
						tutorialMode = i
						tutorialPage = 0
						invalidateRect.Call(h, 0, 0)
						return 0
					}
				}
				return 0
			}
			if pointInRect(p, tutorialBackRect(w, hgt)) {
				playUIButtonClickSound()
				tutorialMode = -1
				tutorialPage = 0
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			if pointInRect(p, tutorialNextRect(w, hgt)) {
				playUIButtonClickSound()
				tutorialPage++
				if tutorialPage >= tutorialPageCount(tutorialMode) {
					gameMeta.FirstLaunchDone = true
					analyticsEvent("tutorial_completed", map[string]any{"mode": tutorialMode})
					saveGameMeta()
					tutorialMode = -1
					tutorialPage = 0
				}
				invalidateRect.Call(h, 0, 0)
			}
			return 0
		}

		// Leaderboard filter tabs.
		if overlayMode == OverlayLeaderboard {
			if handleLeaderboardScrollbarClick(p, w, hgt, false) {
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			if f := leaderboardFilterAt(p, w, hgt); f >= 0 {
				playUIButtonClickSound()
				localLeaderboardFilter = f
				localLeaderboardScroll = 0
				invalidateRect.Call(h, 0, 0)
			}
			return 0
		}
		if overlayMode == OverlayGlobalLeaderboard {
			left, right, top, bottom, _, _, _, _ := globalLeaderboardGeometry(w, hgt)
			if globalLeaderboardScope == leaderboardScopeCompetition && competitionGuideOpen {
				guide := competitionGuidePanelRect(left, right, top, bottom, w, hgt)
				if !pointInRect(p, guide) {
					competitionGuideOpen = false
					playUIButtonClickSound()
					invalidateRect.Call(h, 0, 0)
					return 0
				}
				return 0
			}
			for scope, sr := range globalLeaderboardScopeRects(left+sx(18, w), right-sx(18, w), top+sy(20, hgt), w, hgt) {
				if pointInRect(p, sr) {
					playUIButtonClickSound()
					selectGlobalLeaderboardScope(scope)
					globalLeaderboardScroll = 0
					invalidateRect.Call(h, 0, 0)
					return 0
				}
			}
			if globalLeaderboardScope == leaderboardScopeCompetition && pointInRect(p, competitionGuideButtonRect(left+sx(18, w), right-sx(18, w), top+sy(20, hgt), w, hgt)) {
				playUIButtonClickSound()
				competitionGuideOpen = !competitionGuideOpen
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			if globalLeaderboardScope == leaderboardScopeCompetition {
				if v := competitionViewAt(p, left+sx(18, w), right-sx(18, w), top+sy(51, hgt), w, hgt); v >= 0 {
					playUIButtonClickSound()
					selectCompetitionView(v)
					invalidateRect.Call(h, 0, 0)
					return 0
				}
			}
			if handleLeaderboardScrollbarClick(p, w, hgt, true) {
				playUIButtonClickSound()
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			if globalLeaderboardScope != leaderboardScopeCompetition {
				if f := globalLeaderboardFilterAt(p, w, hgt); f >= 0 {
					playUIButtonClickSound()
					globalLeaderboardFilter = f
					globalLeaderboardScroll = 0
					selectedGlobalPlayer = -1
					if globalLeaderboardScope != leaderboardScopeTop20 {
						go fetchCompetitiveHub()
					}
					invalidateRect.Call(h, 0, 0)
					return 0
				}
			}
			if idx := globalLeaderboardRowAt(p, w, hgt); idx >= 0 {
				playUIButtonClickSound()
				selectedGlobalPlayer = idx
				selectedGlobalOverrideOn = false
				setOverlay(OverlayRemoteProfile)
				prepareRemoteProfile()
				return 0
			}
			return 0
		}
		if overlayMode == OverlayRemoteProfile {
			if remoteProfileIsSelf() && pointInRect(p, profileSkinEditButtonRect(w, hgt)) {
				playUIButtonClickSound()
				profileSkinScroll = 0
				profileCustomizeTab = 0
				refreshProfileSkinUnlocks()
				setOverlay(OverlayProfileSkins)
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			playUIButtonClickSound()
			setOverlay(OverlayGlobalLeaderboard)
			return 0
		}
		if overlayMode == OverlayProfileSkins {
			handleProfileCustomizerClick(p, w, hgt)
			return 0
		}
		if overlayMode == OverlayNameEntry {
			return 0
		}

		// Result information remains until the player deliberately clicks.
		if state == StateResult {
			if survivalBoss1SectionReport {
				continueSurvivalAfterSectionReport(h)
				return 0
			}
			resetToWaiting(h)
			return 0
		}

		// v356: SELECT MODE button clicks are handled before overlay routing above.
		if (state == StateWaiting || state == StateResult || (state == StatePlaying && survivalActive())) && overlayMode == OverlayNone && modeSelectorOpen {
			for i, mr := range modeSelectorCardRects(w, hgt) {
				if pointInRect(p, mr) {
					playUIButtonClickSound()
					selectGameMode(h, i)
					return 0
				}
			}
			// Clicking anywhere else closes the selector, then normal HUD handling continues.
			modeSelectorOpen = false
			invalidateRect.Call(h, 0, 0)
		}

		// Endurance Space Cache / Garage controls in the bottom-middle HUD.
		if state == StateWaiting && overlayMode == OverlayNone && enduranceActive() {
			_, cacheR, garageR := enduranceSpaceUIRects(w, hgt)
			if pointInRect(p, freeCacheClaimRect(w, hgt)) {
				playUIButtonClickSound()
				beginFreeSpaceCacheClaim(h)
				return 0
			}
			if pointInRect(p, cacheR) {
				playUIButtonClickSound()
				// If the server says the daily free Space Cache is available,
				// the main OPEN SPACE CACHE button claims/opens that free cache
				// first. This makes the large cache button behave intuitively
				// instead of showing the 100-coin warning while a free cache is
				// waiting underneath it. The dedicated CLAIM FREE SPACE CACHE
				// text remains clickable as an alternate entry point.
				liveMu.Lock()
				freeReady := discordConnected && freeCacheStatusKnown && freeCacheAvailable
				liveMu.Unlock()
				if freeReady {
					beginFreeSpaceCacheClaim(h)
				} else {
					beginSpaceCacheOpen(h)
				}
				return 0
			}
			if pointInRect(p, garageR) {
				playUIButtonClickSound()
				setOverlay(OverlayGarage)
				return 0
			}
		}

		// Optional player support page. Opens the public PayPal.Me portal.
		if state == StateWaiting && overlayMode == OverlayNone && pointInRect(p, quickSupportDevRect(w, hgt)) {
			playUIButtonClickSound()
			_ = openExternalURL("https://paypal.me/KongDistorted")
			return 0
		}

		// Player-facing bug report form. Opens in the default browser.
		if state == StateWaiting && overlayMode == OverlayNone && pointInRect(p, quickBugReportRect(w, hgt)) {
			playUIButtonClickSound()
			_ = openExternalURL("https://tally.so/r/yPA1od")
			return 0
		}

		// Alternate Discord account button directly beneath LOCAL // TOP 10.
		// Logged out: start Discord OAuth. Logged in: force an immediate cloud/profile sync.
		if state == StateWaiting && overlayMode == OverlayNone && pointInRect(p, quickDiscordLoginRect(w, hgt)) {
			playUIButtonClickSound()
			if discordConnected {
				requestPlayerProfileSync()
			} else {
				beginDiscordLogin()
			}
			invalidateRect.Call(h, 0, 0)
			return 0
		}

		// v360: LOCAL/GLOBAL EXP progression toggle above the left EXP panel.
		if state == StateWaiting && overlayMode == OverlayNone && !survivalActive() && pointInRect(p, expRankToggleRect(w, hgt)) {
			playUIButtonClickSound()
			expRankUseGlobal = !expRankUseGlobal
			if expRankUseGlobal {
				requestGlobalAccountEXP()
			}
			invalidateRect.Call(h, 0, 0)
			return 0
		}

		// Bottom-right quick access dock.
		if state == StateWaiting && overlayMode == OverlayNone {
			for i, qr := range quickAccessRects(w, hgt) {
				if pointInRect(p, qr) {
					playUIButtonClickSound()
					switch i {
					case 0:
						setOverlay(OverlayProfile)
					case 1:
						openLeaderboard()
					case 2:
						openGlobalLeaderboard()
					}
					return 0
				}
			}
		}

		// v274: the legacy orange training dropdown is retired. Its useful account
		// action now lives in the ESC menu; Precision course archetypes randomize.

		// Survival replay checkpoint carousel: six buttons live inside a clipped
		// viewport with a draggable horizontal scrollbar directly underneath.
		if survivalActive() && state == StateWaiting && overlayMode == OverlayNone {
			view := survivalCheckpointViewportRect(w, hgt)
			track, thumb := survivalCheckpointScrollbarRects(w, hgt)
			if pointInRect(p, thumb) && survivalCheckpointMaxScroll(w, hgt) > 0 {
				survivalCheckpointScrollDragging = true
				survivalCheckpointScrollDragOffset = int32(p.X) - thumb.Left
				setCapture.Call(h)
				return 0
			}
			if pointInRect(p, track) && survivalCheckpointMaxScroll(w, hgt) > 0 {
				survivalCheckpointScrollbarDragTo(int32(p.X)-(thumb.Right-thumb.Left)/2, w, hgt)
				invalidateSurvivalHUD(h)
				return 0
			}
			if pointInRect(p, view) {
				for i, sr := range survivalCheckpointButtonRects(w, hgt) {
					// A partially clipped button is clickable only inside the viewport.
					if pointInRect(p, sr) {
						selectSurvivalReplayCheckpoint(h, i)
						return 0
					}
				}
			}
		}

		// Survival gameplay receives the click only after all waiting-state HUD controls.
		if survivalActive() && overlayMode == OverlayNone {
			if state == StateWaiting || state == StatePlaying {
				survivalHandleClick(h, p, false)
				return 0
			}
		}

		if state == StateWaiting && len(path) >= 2 {
			if dist(p, path[0]) <= 25 {
				startGame(h)
			}
			return 0
		}

		if state == StatePlaying {
			// Endurance power-ups are explicit click pickups: consume and activate immediately,
			// without treating the click as a target shot.
			if enduranceActive() && handleEndurancePowerupClick(p) {
				lastMouse = p
				hasLastMouse = true
				if d2dReady {
					renderEnduranceD2D()
				} else {
					invalidateRect.Call(h, 0, 0)
				}
				return 0
			}
			// Precision has a 10-second course clock. Endurance has no course timeout.
			// Never run the Precision clock gate for an Endurance click.
			if !enduranceActive() && updateRealTimeClock() <= 0 {
				loseGame(h, "Time up")
				return 0
			}

			// Every shot matters. Endurance validates against the currently
			// visible scrolling rail; Standard keeps its existing index logic.
			if enduranceActive() {
				if !developerBoundaryGraceActive() && !validateEnduranceMovement(lastMouse, p) {
					loseGame(h, "Cursor crossed the side boundary")
					return 0
				}
				recordAccuracySegment(lastMouse, p)
			} else {
				ok, newProgress := validateMovement(lastMouse, p)
				if !ok {
					loseGame(h, "Cursor left the line")
					return 0
				}
				recordAccuracySegment(lastMouse, p)
				progressIndex = newProgress
			}
			lastMouse = p
			hasLastMouse = true

			t := currentRequiredTarget()
			if t < 0 {
				loseGame(h, "Unnecessary shot")
				return 0
			}

			tp := targetCurrentPoint(t)
			currentIdx := targetCurrentIndex(t)
			hitRadius := 14.0
			// Endurance is intended to be approachable for newer players. Its target
			// hitbox matches the full visible glow, while Standard modes retain the
			// tighter 14 px precision hitbox.
			if enduranceActive() {
				// v175: larger Endurance target hitbox for fast warp sections.
				hitRadius = 28.0
			}
			clickDist := dist(p, tp)

			// Endurance uses the target's visible position because the rail
			// itself scrolls. Standard keeps its path-index timing.
			if enduranceActive() {
				if tp.X > p.X+30 {
					loseGame(h, "Shot too early")
					return 0
				}
				if tp.X < p.X-30 {
					loseGame(h, "Shot too late")
					return 0
				}
			} else {
				if progressIndex+14 < currentIdx {
					loseGame(h, "Shot too early")
					return 0
				}
				if progressIndex > currentIdx+14 {
					loseGame(h, "Shot too late")
					return 0
				}
			}

			// Correct timing but inaccurate aim is still a failed shot.
			if clickDist > hitRadius {
				loseGame(h, "Missed shot")
				return 0
			}

			precision := 1.0 - clickDist/hitRadius
			if precision < 0 {
				precision = 0
			}
			if precision > 1 {
				precision = 1
			}
			targetPrecisionSum += precision
			targetPrecisionHits++

			// Freeze a successful moving target exactly where it was hit.
			targets[t].Point = tp
			targets[t].Index = currentIdx
			targets[t].MinIndex = currentIdx
			targets[t].MaxIndex = currentIdx
			targets[t].MoveRange = 0
			targets[t].Clicked = true

			lastHitAt = time.Now()
			lastHitPoint = tp

			if enduranceActive() {
				playEnduranceExplodeSound()
				// Explosion origin is the target centre captured above, never the cursor.
				addEnduranceTargetExplosion(tp)
				if mainHwnd != 0 {
					w, hgt := getClient(mainHwnd)
					ar := arenaRect(w, hgt)
					addPolishVFX(polishVFXHit, float32(tp.X-float64(ar.Left)), float32(tp.Y-float64(ar.Top)), 0.28)
				}
				enduranceTargetsHit++
				if enduranceWarpActive && enduranceWarpTargetsAllHit() {
					startEnduranceAmbientReturn()
				}
				bonus := 400 + int(math.Round(enduranceDistance*4))
				enduranceBonusScore += bonus
				score = int(math.Round(enduranceDistance*10)) + enduranceBonusScore
				if enduranceWarpActive {
					enduranceNextTargetAt = math.MaxFloat64
				} else {
					enduranceNextTargetAt = enduranceProgressDistance() + enduranceTargetGap()
				}
				status = fmt.Sprintf("TARGET HIT  +%d", bonus)
			} else if activeDifficultyIndex() >= 2 {
				playHitSound()
				timeBonus += 2.5
				lastTime = 10 + timeBonus - time.Since(startTime).Seconds()
				status = "Target hit  +2.5s"
			} else {
				playHitSound()
				status = "Target hit"
			}
			if !(d2dReady && enduranceActive()) {
				if enduranceActive() {
					invalidateRect.Call(h, 0, 0)
				} else {
					// Precision already has a gameplay timer repainting the scene.
					// On the click frame only redraw the affected target/feedback
					// area instead of forcing an immediate full-window paint.
					invalidatePrecisionHitRegion(h, tp)
				}
			}
		}
		return 0
	case WM_MOUSEMOVE:
		if afkTalentPanDragging && overlayMode == OverlayAFKSingularity && afkTalentPanelOpen {
			w, hgt := getClient(h)
			x := int32(loword(lParam))
			y := int32(hiword(lParam))
			afkTalentPanX += x - afkTalentPanLastX
			afkTalentPanY += y - afkTalentPanLastY
			afkTalentPanLastX = x
			afkTalentPanLastY = y
			clampAFKTalentPan(w, hgt)
			setCursor.Call(arrowCursor)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if afkTechScrollDragging && overlayMode == OverlayAFKSingularity && afkTechPanelOpen {
			w, hgt := getClient(h)
			afkTechScrollbarDragTo(int32(hiword(lParam))-afkTechScrollDragOffset, w, hgt)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if afkExpeditionScrollDragging && overlayMode == OverlayAFKSingularity && afkExpeditionPanelOpen {
			w, hgt := getClient(h)
			afkExpeditionScrollbarDragTo(int32(hiword(lParam))-afkExpeditionScrollDragOffset, w, hgt)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if survivalCheckpointScrollDragging && survivalActive() && state == StateWaiting && overlayMode == OverlayNone {
			w, hgt := getClient(h)
			survivalCheckpointScrollbarDragTo(int32(loword(lParam))-survivalCheckpointScrollDragOffset, w, hgt)
			invalidateSurvivalHUD(h)
			return 0
		}
		if settingsScrollbarDragging && overlayMode == OverlaySettings {
			w, hgt := getClient(h)
			settingsScrollbarDragTo(int32(hiword(lParam))-settingsScrollbarDragOffset, w, hgt)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if settingsVolumeDrag != 0 && overlayMode == OverlaySettings {
			w, hgt := getClient(h)
			p := FPoint{float64(loword(lParam)), float64(hiword(lParam))}
			updateSettingsVolumeFromPoint(settingsVolumeDrag, p, w, hgt)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if hudLayoutEditorActive && hudLayoutDragging {
			w, hgt := getClient(h)
			px := screenToDesignX(loword(lParam), w)
			py := screenToDesignY(hiword(lParam), hgt)
			r := hudLayoutRectByIndex(hudLayoutSelected)
			width := r.Right - r.Left
			height := r.Bottom - r.Top
			r.Left = px - hudLayoutDragOffsetX
			r.Top = py - hudLayoutDragOffsetY
			r.Right = r.Left + width
			r.Bottom = r.Top + height
			setHUDLayoutRectByIndex(hudLayoutSelected, r)
			setCursor.Call(arrowCursor)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if achievementDragging && overlayMode == OverlayAchievements {
			w, hgt := getClient(h)
			updateAchievementDrag(hiword(lParam), w, hgt)
			setCursor.Call(arrowCursor)
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if state == StateIntro {
			setCursor.Call(0)
			return 0
		}
		p := FPoint{float64(loword(lParam)), float64(hiword(lParam))}
		cursorPos = p
		w, hgt := getClient(h)
		cursorInArena = pointInArena(p, w, hgt)
		// v438: Windows pointer on HUD/interface surfaces; game cursor only while
		// the pointer is inside the gameplay arena with no interface overlay.
		if shouldShowWindowsCursor(h, p, w, hgt) {
			setCursor.Call(arrowCursor)
		} else {
			setCursor.Call(0)
		}

		if state != StatePlaying {
			// Survival revive is a frozen decision state. Repaint only the opaque
			// revive panel so its button hover remains responsive without redrawing
			// or advancing anything in the gameplay arena behind it.
			if state == StateFailed && survivalActive() {
				r := survivalRespawnPanelRect(w, hgt)
				invalidateRect.Call(h, uintptr(unsafe.Pointer(&r)), 0)
			} else {
				invalidateRect.Call(h, 0, 0)
			}
			return 0
		}
		if overlayMode == OverlayDeveloperConsole {
			// The console is a true gameplay pause. Mouse movement while typing
			// must never trigger rail, target or meteor collision checks.
			invalidateRect.Call(h, 0, 0)
			return 0
		}

		if survivalActive() {
			// Survival does not depend on the Precision/Endurance path buffer.
			// Boundary failure is evaluated directly from the arena while combat is live.
			if survivalBoundaryFailureActive() && !pointInArena(p, w, hgt) {
				survivalFail(h, "Cursor left game area")
				return 0
			}
			if survivalBoss2CombatActive() && survivalBoss2CursorCollides(p, w, hgt) {
				survivalFail(h, "Cursor touched the Void Serpent")
				return 0
			}
			// The 60 Hz Survival timer already repaints the arena. Do not schedule an
			// additional full-window paint for every raw mouse event; high-polling-rate
			// mice otherwise multiply render work and make moving enemies appear jumpy.
			return 0
		}

		if len(path) < 2 {
			invalidateRect.Call(h, 0, 0)
			return 0
		}

		if !enduranceActive() && updateRealTimeClock() <= 0 {
			loseGame(h, "Time up")
			return 0
		}

		// Leaving the protected white game box itself is also an immediate failure.
		if !pointInArena(p, w, hgt) {
			loseGame(h, "Cursor left game area")
			return 0
		}

		if enduranceActive() {
			if cursorTouchesEnduranceBlock(p) {
				loseGame(h, "Hit a falling meteorite")
				return 0
			}
			if !developerBoundaryGraceActive() && !validateEnduranceMovement(lastMouse, p) {
				loseGame(h, "Cursor crossed the side boundary")
				return 0
			}
			recordAccuracySegment(lastMouse, p)
		} else {
			ok, newProgress := validateMovement(lastMouse, p)
			if !ok {
				loseGame(h, "Cursor left the line")
				return 0
			}
			recordAccuracySegment(lastMouse, p)
			progressIndex = newProgress
		}

		// The next required target cannot be passed without clicking.
		t := currentRequiredTarget()
		if t >= 0 {
			if enduranceActive() {
				tp := targetCurrentPoint(t)
				if tp.X < p.X-26 {
					loseGame(h, "Target missed")
					return 0
				}
			} else {
				passIndex := targets[t].Index + 8
				if movingTargetsActive() {
					passIndex = targets[t].MaxIndex + 8
				}
				if progressIndex > passIndex {
					loseGame(h, "Target missed")
					return 0
				}
			}
		}

		if !enduranceActive() && dist(p, path[len(path)-1]) <= 22 {
			if currentRequiredTarget() >= 0 {
				loseGame(h, "Target missed")
				return 0
			}
			if progressIndex < len(path)-40 {
				loseGame(h, "Invalid route")
				return 0
			}
			winGame(h)
			return 0
		}

		lastMouse = p
		hasLastMouse = true
		if !(d2dReady && enduranceActive()) {
			invalidateRect.Call(h, 0, 0)
		}
		return 0

	case WM_TIMER:
		switch wParam {
		case TIMER_GAME:
			if overlayMode == OverlayDeveloperConsole {
				return 0
			}
			if state == StatePlaying {
				if survivalActive() {
					// v365: enforce the Survival arena boundary from the game timer as well as
					// WM_MOUSEMOVE. Without this, Windows stops delivering client mouse-move
					// messages once the cursor leaves the window, allowing players to park the
					// cursor outside the game and wait out boss attacks. Poll the real screen
					// cursor position every Survival tick and convert it back to client space.
					if survivalBoundaryFailureActive() {
						var cp POINT
						if ok, _, _ := getCursorPos.Call(uintptr(unsafe.Pointer(&cp))); ok != 0 {
							screenToClient.Call(h, uintptr(unsafe.Pointer(&cp)))
							actual := FPoint{X: float64(cp.X), Y: float64(cp.Y)}
							cw, ch := getClient(h)
							if !pointInArena(actual, cw, ch) {
								survivalFail(h, "Cursor left game area")
								return 0
							}
						}
					}
					updateSurvival(h)
				} else if enduranceActive() {
					if !d2dReady {
						updateEndurance(h)
						if state == StatePlaying {
							invalidateRect.Call(h, 0, 0)
						}
					}
				} else if updateRealTimeClock() <= 0 {
					lastTime = 0
					loseGame(h, "Time up")
				} else {
					invalidateRect.Call(h, 0, 0)
				}
			}
		case TIMER_FAIL_ANIM:
			if state != StateFailed || polishFailureStarted.IsZero() || time.Since(polishFailureStarted) >= 360*time.Millisecond {
				killTimer.Call(h, TIMER_FAIL_ANIM)
			}
			invalidateRect.Call(h, 0, 0)
			return 0
		case TIMER_FAIL_RESET:
			killTimer.Call(h, TIMER_FAIL_ANIM)
			killTimer.Call(h, TIMER_FAIL_RESET)
			if state == StateFailed {
				if survivalActive() {
					finishSurvivalFailure(h)
				} else if enduranceActive() {
					finishEnduranceFailure(h)
				} else {
					resetToWaiting(h)
				}
			}
		case TIMER_RESULT_RESET:
			// v36 result screen is player-dismissed, never time-dismissed.
			killTimer.Call(h, TIMER_RESULT_RESET)

		case TIMER_LEVELUP:
			if levelUpAt.IsZero() || time.Since(levelUpAt) > 1600*time.Millisecond {
				killTimer.Call(h, TIMER_LEVELUP)
				levelUpAt = time.Time{}
			}
			invalidateRect.Call(h, 0, 0)

		case TIMER_INTRO:
			elapsed := introElapsed()

			// At the exact end of the 3.600s movie, prepare the hidden game window
			// as a fully black frame FIRST. Only then remove the splash. From that
			// black frame the existing 140ms GDI fade reveals the already-loaded UI.
			if elapsed >= 3600*time.Millisecond && introSplashHwnd != 0 {
				stopKongIntroVideo()
				invalidateRect.Call(h, 0, 0)
				showWindow.Call(h, SW_SHOW)
				updateWindow.Call(h)
				destroyWindow.Call(introSplashHwnd)
				introSplashHwnd = 0
				introVideoHwnd = 0
				if !bossStarted {
					startBossMusic()
				}
			}
			if elapsed >= 3740*time.Millisecond {
				finishIntro(h)
			} else if introSplashHwnd == 0 {
				invalidateRect.Call(h, 0, 0)
			}

		case TIMER_PARTICLES:
			if overlayMode == OverlayAFKSingularity {
				// Third independent repaint heartbeat for Starbase. TIMER_UI and
				// TIMER_STARBASE are the primary clocks, but the global particle timer
				// guarantees world motion even if another UI path accidentally replaces
				// one of those timer IDs later.
				invalidateRect.Call(h, 0, 0)
			} else if state == StateWaiting && overlayMode == OverlayNone {
				r := arenaRectForInvalidate(h)
				invalidateRect.Call(h, uintptr(unsafe.Pointer(&r)), 0)
			}
		case TIMER_STARBASE:
			// Dedicated Starbase animation heartbeat. It remains alive beneath the
			// ESC/settings overlay so Starbase never falls back to the prior game mode.
			if overlayMode == OverlayAFKSingularity || (overlayMode == OverlaySettings && starbaseSettingsReturn) {
				invalidateRect.Call(h, 0, 0)
			} else {
				killTimer.Call(h, TIMER_STARBASE)
			}
			return 0

		case TIMER_UI:
			if overlayMode == OverlayGlobalLeaderboard {
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			if overlayMode == OverlayGarage || overlayMode == OverlayRemoteProfile || overlayMode == OverlayProfileSkins || overlayMode == OverlayAFKSingularity {
				// Cosmetic UI animation heartbeat: Garage effects and Master-rank Global
				// Profile animations must continue even when the mouse is completely still.
				invalidateRect.Call(h, 0, 0)
				return 0
			}
			if !spaceCacheWarningUntil.IsZero() {
				if time.Now().After(spaceCacheWarningUntil) {
					spaceCacheWarningUntil = time.Time{}
					spaceCacheWarningText = ""
					if overlayMode != OverlaySpaceCache {
						killTimer.Call(h, TIMER_UI)
					}
				}
				invalidateRect.Call(h, 0, 0)
			}
			if overlayMode == OverlaySpaceCache {
				if !spaceCacheOpened && !spaceCacheOpenStarted.IsZero() && time.Since(spaceCacheOpenStarted) >= 2*time.Second {
					resolveSpaceCacheReward()
				}
				invalidateRect.Call(h, 0, 0)
				if spaceCacheOpened && time.Since(spaceCacheOpenStarted) > 10*time.Second {
					killTimer.Call(h, TIMER_UI)
				}
			} else {
				if gameMeta.ReducedMotion || uiTransitionStart.IsZero() || time.Since(uiTransitionStart) > 240*time.Millisecond {
					killTimer.Call(h, TIMER_UI)
				}
				invalidateRect.Call(h, 0, 0)
			}
		case TIMER_LIVE:
			tickLiveAnimation(h)
		case TIMER_LIVE_SYSTEMS:
			tickLiveSystems(h)
		}
		return 0

	case WM_CHAR:
		if overlayMode == OverlayDeveloperConsole {
			ch := rune(wParam)
			switch ch {
			case 8:
				r := []rune(developerConsoleInput)
				if len(r) > 0 {
					developerConsoleInput = string(r[:len(r)-1])
				}
			case 9:
				// TAB is handled in WM_KEYDOWN.
			case 13:
				developerConsoleStatus = executeDeveloperCommand(developerConsoleInput)
				developerConsoleInput = ""
			default:
				if ch >= 32 && ch <= 126 && len([]rune(developerConsoleInput)) < 96 {
					developerConsoleInput += string(ch)
				}
			}
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		if overlayMode == OverlayNameEntry {
			ch := rune(wParam)
			switch ch {
			case 8:
				runes := []rune(nameInput)
				if len(runes) > 0 {
					nameInput = string(runes[:len(runes)-1])
				}
			case 13:
				saveLeaderboardEntry(nameInput)
				overlayMode = OverlayNone
				nameInput = ""
				resetToWaiting(h)
				status = "Score saved locally"
			default:
				if ch >= 32 && ch <= 126 && len([]rune(nameInput)) < 16 {
					nameInput += string(ch)
				}
			}
			invalidateRect.Call(h, 0, 0)
			return 0
		}
		return 0

	case WM_KEYDOWN:
		if state == StateIntro {
			return 0
		}

		// F2: owner-only live HUD position editor.
		if wParam == VK_F2 && isDeveloperOwner() {
			if overlayMode != OverlayNone && overlayMode != OverlayDeveloperConsole {
				closeOverlay()
			}
			if overlayMode == OverlayDeveloperConsole {
				closeOverlay()
				developerConsoleInput = ""
				developerConsoleStatus = ""
			}
			hudLayoutEditorActive = !hudLayoutEditorActive
			hudLayoutDragging = false
			if !hudLayoutEditorActive {
				hudLayoutSelected = -1
				releaseCapture.Call()
				saveHUDLayoutConfig()
				status = "HUD layout saved"
			} else {
				status = "HUD POSITION EDITOR: drag buttons or use arrow keys"
			}
			invalidateRect.Call(h, 0, 0)
			return 0
		}

		if hudLayoutEditorActive {
			step := int32(1)
			if s, _, _ := getKeyState.Call(VK_SHIFT); int16(s&0xFFFF) < 0 {
				step = 10
			}
			switch wParam {
			case VK_LEFT:
				moveHUDLayoutSelection(-step, 0)
			case VK_RIGHT:
				moveHUDLayoutSelection(step, 0)
			case VK_UP:
				moveHUDLayoutSelection(0, -step)
			case VK_DOWN:
				moveHUDLayoutSelection(0, step)
			case VK_ESCAPE:
				hudLayoutEditorActive = false
				hudLayoutSelected = -1
				hudLayoutDragging = false
				releaseCapture.Call()
				saveHUDLayoutConfig()
				status = "HUD layout saved"
			default:
				return 0
			}
			invalidateRect.Call(h, 0, 0)
			return 0
		}

		// Developer console: TAB toggles it for the authenticated owner account only.
		if wParam == VK_TAB {
			if overlayMode == OverlayDeveloperConsole {
				closeOverlay()
				developerConsoleInput = ""
				developerConsoleStatus = ""
			} else if isDeveloperOwner() {
				setOverlay(OverlayDeveloperConsole)
				developerConsoleInput = ""
				developerConsoleStatus = ""
			}
			invalidateRect.Call(h, 0, 0)
			return 0
		}

		// While the developer console is open, consume every gameplay/menu hotkey.
		// Text entry continues through WM_CHAR, so letters, numbers, spaces,
		// backspace and Enter are still available to the console input itself.
		if overlayMode == OverlayDeveloperConsole {
			if wParam == VK_ESCAPE {
				closeOverlay()
				developerConsoleInput = ""
				developerConsoleStatus = ""
				invalidateRect.Call(h, 0, 0)
			}
			return 0
		}

		if overlayMode == OverlayMainMenu {
			if wParam == VK_ESCAPE {
				closeOverlay()
			}
			if wParam == VK_RETURN || wParam == VK_SPACE {
				closeOverlay()
			}
			return 0
		}
		if overlayMode == OverlayAFKSingularity {
			if wParam == VK_ESCAPE {
				if afkAnyManagementPanelOpen() {
					afkCloseManagementPanels()
					invalidateRect.Call(h, 0, 0)
				} else {
					starbaseSettingsReturn = true
					setOverlay(OverlaySettings)
				}
			}
			return 0
		}
		if overlayMode == OverlayProfile || overlayMode == OverlaySettings ||
			overlayMode == OverlayAchievements || overlayMode == OverlayReleaseNotes || overlayMode == OverlayGarage {

			if wParam == VK_ESCAPE {
				if overlayMode == OverlaySettings && starbaseSettingsReturn {
					setOverlay(OverlayAFKSingularity)
					starbaseSettingsReturn = false
				} else {
					closeOverlay()
				}
			}
			return 0
		}
		if overlayMode == OverlayTutorial {
			if wParam == VK_ESCAPE {
				if tutorialMode >= 0 {
					tutorialMode = -1
					tutorialPage = 0
					invalidateRect.Call(h, 0, 0)
				} else {
					gameMeta.FirstLaunchDone = true
					analyticsEvent("tutorial_completed", map[string]any{"mode": tutorialMode})
					saveGameMeta()
					closeOverlay()
				}
				return 0
			}
			if tutorialMode >= 0 && (wParam == VK_RETURN || wParam == VK_SPACE) {
				tutorialPage++
				if tutorialPage >= tutorialPageCount(tutorialMode) {
					gameMeta.FirstLaunchDone = true
					analyticsEvent("tutorial_completed", map[string]any{"mode": tutorialMode})
					saveGameMeta()
					tutorialMode = -1
					tutorialPage = 0
				}
				invalidateRect.Call(h, 0, 0)
			}
			return 0
		}
		if overlayMode == OverlayNameEntry {
			if wParam == VK_ESCAPE {
				overlayMode = OverlayNone
				nameInput = ""
				resetToWaiting(h)
			}
			return 0
		}
		if overlayMode == OverlayLeaderboard {
			if wParam == VK_ESCAPE || wParam == 'L' {
				closeOverlay()
			}
			return 0
		}
		if overlayMode == OverlayRemoteProfile {
			if wParam == VK_ESCAPE {
				closeOverlay()
			} else if wParam == VK_RETURN || wParam == VK_SPACE {
				setOverlay(OverlayGlobalLeaderboard)
			}
			return 0
		}
		if overlayMode == OverlayProfileSkins {
			if wParam == VK_ESCAPE {
				setOverlay(OverlayRemoteProfile)
			}
			return 0
		}
		if overlayMode == OverlayGlobalLeaderboard {
			if wParam == VK_ESCAPE || wParam == 'G' {
				closeOverlay()
			}
			return 0
		}

		if wParam == VK_ESCAPE {
			if state == StateWaiting {
				setOverlay(OverlayMainMenu)
			}
			// During an active run, ESC is intentionally ignored so it can never
			// accidentally close the game.
			return 0
		}

		if state == StateResult {
			switch wParam {
			case 'L':
				openLeaderboard()
				invalidateRect.Call(h, 0, 0)
			case 'G':
				openGlobalLeaderboard()
				invalidateRect.Call(h, 0, 0)
			case VK_RETURN, VK_SPACE:
				resetToWaiting(h)
			}
			return 0
		}

		if state == StateFailed {
			return 0
		}

		// Endurance inventory hotkeys. They are processed only during an active
		// run and never while a menu/developer overlay is consuming keyboard input.
		if enduranceActive() && state == StatePlaying {
			switch wParam {
			case 'Q':
				if activateStoredEnduranceShield() {
					invalidateRect.Call(h, 0, 0)
				} else if enduranceStoredShields <= 0 {
					status = "NO SHIELD STORED"
				}
				return 0
			case 'W':
				if activateStoredEnduranceTime() {
					invalidateRect.Call(h, 0, 0)
				} else if enduranceStoredTime <= 0 {
					status = "NO TIME POWER-UP STORED"
				}
				return 0
			}
		}

		switch wParam {
		case 'M':
			if state == StateWaiting {
				menuOpen = !menuOpen
				invalidateRect.Call(h, 0, 0)
			}
		case 'L':
			if state == StateWaiting {
				openLeaderboard()
				invalidateRect.Call(h, 0, 0)
			}
		case 'G':
			if state == StateWaiting {
				openGlobalLeaderboard()
				invalidateRect.Call(h, 0, 0)
			}
		case '1', '2', '3', '4':
			changeDifficulty(h, int(wParam-'1'))
		}
		return 0
	}

	r, _, _ := defWindowProcW.Call(h, uintptr(msg), wParam, lParam)
	return r
}

var uiFontFaces = []string{
	"Lucida Console",
	"Segoe UI",
	"Bahnschrift",
	"Georgia",
	"Trebuchet MS",
	"Comic Sans MS",
	"Consolas",
	"Franklin Gothic Medium",
}

var uiFontLabels = []string{
	"RETRO // LUCIDA",
	"MODERN // SEGOE",
	"TECH // BAHNSCHRIFT",
	"CLASSIC // GEORGIA",
	"HUMANIST // TREBUCHET",
	"PLAYFUL // COMIC SANS",
	"MONO // CONSOLAS",
	"ARCADE // FRANKLIN",
}

func selectedUIFontFace() string {
	idx := gameMeta.FontOverride
	if idx < 0 || idx >= len(uiFontFaces) {
		idx = 0
	}
	return uiFontFaces[idx]
}

func selectedUIFontLabel() string {
	idx := gameMeta.FontOverride
	if idx < 0 || idx >= len(uiFontLabels) {
		idx = 0
	}
	return uiFontLabels[idx]
}

func makeFontForFace(faceName string, height uint32, weight uintptr) uintptr {
	face := utf16ptr(faceName)
	h, _, _ := createFontW.Call(uintptr(^height+1), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, CLEARTYPE_QUALITY, 0, uintptr(unsafe.Pointer(face)))
	return h
}

func deleteUIFontHandle(p *uintptr) {
	if p != nil && *p != 0 {
		deleteObject.Call(*p)
		*p = 0
	}
}

func releaseUIFontHandles() {
	releaseAdvancedProfileFonts()
	deleteUIFontHandle(&failedFont)
	deleteUIFontHandle(&failedReasonFont)
	deleteUIFontHandle(&hudTitleFont)
	deleteUIFontHandle(&hudStatFont)
	deleteUIFontHandle(&hudSmallFont)
	deleteUIFontHandle(&hudTinyFont)
	deleteUIFontHandle(&profileNameFont)
	deleteUIFontHandle(&introLogoFont)
	deleteUIFontHandle(&introTextFont)
}

func rebuildUIFontHandles() {
	// All choices use the exact same requested pixel heights as the original UI.
	// This keeps alternate faces from silently enlarging Settings, leaderboards,
	// achievement cards, profile rows, or HUD labels. Windows performs normal
	// face fallback if a font is unavailable on a particular machine.
	releaseUIFontHandles()
	face := utf16ptr(selectedUIFontFace())
	makeFont := func(height uint32, weight uintptr) uintptr {
		h, _, _ := createFontW.Call(
			uintptr(^height+1), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, CLEARTYPE_QUALITY, 0, uintptr(unsafe.Pointer(face)),
		)
		return h
	}
	failedFont = makeFont(78, 800)
	failedReasonFont = makeFont(19, 900)
	hudTitleFont = makeFont(23, 700)
	hudStatFont = makeFont(25, 700)
	hudSmallFont = makeFont(15, 600)
	hudTinyFont = makeFont(12, 600)
	profileNameFont = makeFont(30, 800)
	introLogoFont = makeFontForFace("Bahnschrift SemiBold", 92, 800)
	introTextFont = makeFontForFace("Bahnschrift", 24, 600)
}

func runGame() {
	// Use physical client pixels for mouse/input/render coordinates. This avoids
	// Windows DPI virtualization shifting cursor-to-rail geometry at 125–200% scaling.
	if err := setProcessDPIAware.Find(); err == nil {
		setProcessDPIAware.Call()
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	timeBeginPeriod.Call(1)
	defer timeEndPeriod.Call(1)
	initGDIPlus()
	defer shutdownGDIPlus()

	rand.Seed(time.Now().UnixNano())
	initGameFolders()
	if err := validateRuntimeAssets(); err != nil {
		writeStartupAssetError(err)
	}
	loadHUDLayoutConfig()
	initUI()
	loadLeaderboard()
	loadPlayerProgress()
	loadGameMeta()
	if stale := strings.TrimSpace(gameMeta.PrecisionCompetitionActiveDifficulty); stale != "" {
		gameMeta.PrecisionCompetitionActiveDifficulty = ""
		saveGameMeta()
		go reportPrecisionCompetitionFailure(stale)
	}
	initGameplayAnalytics()
	loadSyncedEndurancePB()
	if !gameMeta.MovingBackgroundV79Migrated {
		gameMeta.MovingBackground = true
		gameMeta.MovingBackgroundV79Migrated = true
		saveGameMeta()
	}
	applyV59RankResetMigration()
	applyAchievementEXPTripleV105Migration()
	grantMissingAchievementEXPRewards()
	difficulty = 0
	adaptiveTier = 0
	adaptiveMode = false
	loadAuthSession()
	if discordConnected {
		requestPlayerProfileSync()
	}
	evaluateAchievements(0, 0, 0, 0, "", 0)
	evaluateEnduranceAchievements()

	hInstance, _, _ := getModuleHandleW.Call(0)
	className := utf16ptr("CursorTrainerV108Class")
	arrowCursor, _, _ = loadCursorW.Call(0, IDC_ARROW)
	appIcon, _, _ := loadIconW.Call(hInstance, 1)
	callbackPtr = syscall.NewCallback(wndProc)

	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   callbackPtr,
		HInstance:     hInstance,
		HIcon:         appIcon,
		HCursor:       arrowCursor,
		LpszClassName: className,
		HIconSm:       appIcon,
	}
	atom, _, _ := registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return
	}

	d2dChildCallback = syscall.NewCallback(d2dChildWndProc)
	d2dChildClass := utf16ptr("CursorTrainerV108PlayfieldClass")
	d2dwc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		Style:         0,
		LpfnWndProc:   d2dChildCallback,
		HInstance:     hInstance,
		HCursor:       0,
		LpszClassName: d2dChildClass,
	}
	registerClassExW.Call(uintptr(unsafe.Pointer(&d2dwc)))

	// Dedicated startup splash. It is intentionally plain Win32/GDI and reuses
	// the already-proven MCI intro path; no Media Foundation/COM subsystem.
	introSplashCallback = syscall.NewCallback(introSplashWndProc)
	introSplashClass := utf16ptr("CursorTrainerV108IntroSplashClass")
	introSplashWC := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   introSplashCallback,
		HInstance:     hInstance,
		HIcon:         appIcon,
		HCursor:       0,
		LpszClassName: introSplashClass,
		HIconSm:       appIcon,
	}
	if atom, _, _ := registerClassExW.Call(uintptr(unsafe.Pointer(&introSplashWC))); atom == 0 {
		return
	}

	rebuildUIFontHandles()
	defer releaseUIFontHandles()

	title := utf16ptr("Cursor Control Trainer 1.0")
	// v350: lock every device to the same 3:2 virtual UI geometry.  The actual
	// client size may scale down to fit a smaller laptop display, but X and Y use
	// the same uniform scale because the client aspect ratio remains 1536:1024.
	targetClientW, targetClientH := int32(1200), int32(800)
	screenW, _, _ := getSystemMetrics.Call(0)
	screenH, _, _ := getSystemMetrics.Call(1)
	maxClientW := int32(screenW) - 80
	maxClientH := int32(screenH) - 110
	if maxClientW < 720 {
		maxClientW = 720
	}
	if maxClientH < 480 {
		maxClientH = 480
	}
	scale := math.Min(float64(maxClientW)/1200.0, float64(maxClientH)/800.0)
	if scale > 1.0 {
		scale = 1.0
	}
	if scale < 0.60 {
		scale = 0.60
	}
	targetClientW = int32(math.Round(1200.0 * scale))
	targetClientH = targetClientW * 2 / 3
	wr := RECT{0, 0, targetClientW, targetClientH}
	adjustWindowRectEx.Call(uintptr(unsafe.Pointer(&wr)), WINDOW_STYLE, 0, 0)
	windowW := wr.Right - wr.Left
	windowH := wr.Bottom - wr.Top
	h, _, _ := createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		WINDOW_STYLE&^WS_VISIBLE, // genuinely hidden until the intro has finished
		CW_USEDEFAULT, CW_USEDEFAULT, uintptr(windowW), uintptr(windowH),
		0, 0, hInstance, 0,
	)
	if h == 0 {
		return
	}
	mainHwnd = h

	cw, ch := getClient(h)
	ar := arenaRect(cw, ch)
	childTitle := utf16ptr("")
	child, _, _ := createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(d2dChildClass)),
		uintptr(unsafe.Pointer(childTitle)),
		WS_CHILD|WS_CLIPSIBLINGS,
		uintptr(ar.Left), uintptr(ar.Top),
		uintptr(ar.Right-ar.Left), uintptr(ar.Bottom-ar.Top),
		h, 0, hInstance, 0,
	)
	if child != 0 {
		d2dChildHwnd = child
		initD2D(child)
	}

	initAudio()
	go retryPendingEnduranceSubmission()

	// Prepare the destination interface completely while the real game window is
	// still hidden. This preserves the working v454/v456 game initialisation.
	resetToWaiting(h)
	if !gameMeta.FirstLaunchDone {
		tutorialPage = 0
		tutorialMode = -1
		setOverlay(OverlayTutorial)
	} else {
		setOverlay(OverlayMainMenu)
	}
	state = StateIntro
	introStart = time.Time{}
	introVideoPlaying = false

	// The splash is the FIRST Cursor Control window Windows is allowed to display.
	// Match the hidden game's exact outer rectangle so the handoff has no jump.
	var gameRect RECT
	if ok, _, _ := getWindowRect.Call(h, uintptr(unsafe.Pointer(&gameRect))); ok == 0 {
		gameRect = RECT{Left: 100, Top: 100, Right: 100 + windowW, Bottom: 100 + windowH}
	}
	splash, _, _ := createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(introSplashClass)),
		uintptr(unsafe.Pointer(title)),
		WINDOW_STYLE&^WS_VISIBLE,
		uintptr(gameRect.Left), uintptr(gameRect.Top),
		uintptr(gameRect.Right-gameRect.Left), uintptr(gameRect.Bottom-gameRect.Top),
		0, 0, hInstance, 0,
	)
	if splash == 0 {
		// Conservative failure path: never strand the user with an invisible app.
		state = StateWaiting
		showWindow.Call(h, SW_SHOW)
		updateWindow.Call(h)
		setTimer.Call(h, TIMER_PARTICLES, particleTimerInterval(), 0)
		setTimer.Call(h, TIMER_LIVE, 16, 0)
		setTimer.Call(h, TIMER_LIVE_SYSTEMS, 250, 0)
		startBossMusic()
		applyAudioVolumes()
		runMainLoop(h)
		return
	}
	introSplashHwnd = splash
	showWindow.Call(splash, SW_SHOW)
	updateWindow.Call(splash) // synchronous solid-black first paint
	beginIntro(h)

	runMainLoop(h)
}
