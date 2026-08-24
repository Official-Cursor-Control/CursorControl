//go:build windows

package main

type afkModuleSprite struct {
	file string
	data []byte
	w, h int32
}

// v463: sprite order is the authored 9x8 source grid: row = set, column = piece.
// Do not reorder this table without changing afkModuleSetDefs in the same order.
var afkModuleSprites = [...]afkModuleSprite{
	{file: "00_holo_platform.bgra", w: 147, h: 113},      // set 1, item 1: HOLO PLATFORM
	{file: "01_operator_id.bgra", w: 146, h: 113},        // set 1, item 2: OPERATOR ID
	{file: "02_stasis_pod.bgra", w: 84, h: 125},          // set 1, item 3: STASIS POD
	{file: "03_crew_crate.bgra", w: 122, h: 125},         // set 1, item 4: CREW CRATE
	{file: "04_comms_headset.bgra", w: 139, h: 113},      // set 1, item 5: COMMS HEADSET
	{file: "05_operator_seat.bgra", w: 122, h: 130},      // set 1, item 6: OPERATOR SEAT
	{file: "06_support_drone.bgra", w: 162, h: 112},      // set 1, item 7: SUPPORT DRONE
	{file: "07_holo_card.bgra", w: 129, h: 117},          // set 1, item 8: HOLO CARD
	{file: "08_collector_ring.bgra", w: 179, h: 145},     // set 2, item 1: COLLECTOR RING
	{file: "09_generator_coil.bgra", w: 133, h: 119},     // set 2, item 2: GENERATOR COIL
	{file: "10_star_crystal.bgra", w: 135, h: 120},       // set 2, item 3: STAR CRYSTAL
	{file: "11_energy_canister.bgra", w: 87, h: 120},     // set 2, item 4: ENERGY CANISTER
	{file: "12_reactor_core.bgra", w: 131, h: 145},       // set 2, item 5: REACTOR CORE
	{file: "13_starbit_bars.bgra", w: 125, h: 114},       // set 2, item 6: STARBIT BARS
	{file: "14_starbit_orb.bgra", w: 143, h: 131},        // set 2, item 7: STARBIT ORB
	{file: "15_collector_unit.bgra", w: 147, h: 128},     // set 2, item 8: COLLECTOR UNIT
	{file: "16_lab_platform.bgra", w: 171, h: 140},       // set 3, item 1: LAB PLATFORM
	{file: "17_research_chip.bgra", w: 147, h: 127},      // set 3, item 2: RESEARCH CHIP
	{file: "18_sample_vial.bgra", w: 109, h: 127},        // set 3, item 3: SAMPLE VIAL
	{file: "19_robotic_arm.bgra", w: 127, h: 130},        // set 3, item 4: ROBOTIC ARM
	{file: "20_research_terminal.bgra", w: 124, h: 140},  // set 3, item 5: RESEARCH TERMINAL
	{file: "21_scan_dish.bgra", w: 106, h: 111},          // set 3, item 6: SCAN DISH
	{file: "22_quantum_canister.bgra", w: 131, h: 143},   // set 3, item 7: QUANTUM CANISTER
	{file: "23_tech_crate.bgra", w: 129, h: 121},         // set 3, item 8: TECH CRATE
	{file: "24_hydro_hull.bgra", w: 177, h: 136},         // set 4, item 1: HYDRO HULL
	{file: "25_mini_sub.bgra", w: 134, h: 108},           // set 4, item 2: MINI SUB
	{file: "26_turbine_fan.bgra", w: 123, h: 119},        // set 4, item 3: TURBINE FAN
	{file: "27_oxygen_canister.bgra", w: 85, h: 114},     // set 4, item 4: OXYGEN CANISTER
	{file: "28_hydro_drill.bgra", w: 129, h: 133},        // set 4, item 5: HYDRO DRILL
	{file: "29_solar_array.bgra", w: 157, h: 126},        // set 4, item 6: SOLAR ARRAY
	{file: "30_scanner_module.bgra", w: 147, h: 110},     // set 4, item 7: SCANNER MODULE
	{file: "31_sonar_dome.bgra", w: 101, h: 120},         // set 4, item 8: SONAR DOME
	{file: "32_lounge_habitat.bgra", w: 182, h: 117},     // set 5, item 1: LOUNGE HABITAT
	{file: "33_lounge_chair.bgra", w: 129, h: 121},       // set 5, item 2: LOUNGE CHAIR
	{file: "34_sleep_pod.bgra", w: 157, h: 129},          // set 5, item 3: SLEEP POD
	{file: "35_plant_terrarium.bgra", w: 102, h: 120},    // set 5, item 4: PLANT TERRARIUM
	{file: "36_refreshment_unit.bgra", w: 111, h: 127},   // set 5, item 5: REFRESHMENT UNIT
	{file: "37_fitness_module.bgra", w: 175, h: 108},     // set 5, item 6: FITNESS MODULE
	{file: "38_game_console.bgra", w: 130, h: 113},       // set 5, item 7: GAME CONSOLE
	{file: "39_jukebox.bgra", w: 110, h: 113},            // set 5, item 8: JUKEBOX
	{file: "40_arcade_cabinet.bgra", w: 130, h: 133},     // set 6, item 1: ARCADE CABINET
	{file: "41_racing_seat.bgra", w: 122, h: 116},        // set 6, item 2: RACING SEAT
	{file: "42_vr_visor.bgra", w: 125, h: 99},            // set 6, item 3: VR VISOR
	{file: "43_game_cartridge.bgra", w: 117, h: 126},     // set 6, item 4: GAME CARTRIDGE
	{file: "44_joystick.bgra", w: 116, h: 124},           // set 6, item 5: JOYSTICK
	{file: "45_claw_machine.bgra", w: 116, h: 133},       // set 6, item 6: CLAW MACHINE
	{file: "46_token_chest.bgra", w: 121, h: 117},        // set 6, item 7: TOKEN CHEST
	{file: "47_dance_pad.bgra", w: 149, h: 129},          // set 6, item 8: DANCE PAD
	{file: "48_extractor_base.bgra", w: 175, h: 137},     // set 7, item 1: EXTRACTOR BASE
	{file: "49_drill_bit.bgra", w: 129, h: 130},          // set 7, item 2: DRILL BIT
	{file: "50_cargo_crate.bgra", w: 127, h: 121},        // set 7, item 3: CARGO CRATE
	{file: "51_ore_cart.bgra", w: 142, h: 121},           // set 7, item 4: ORE CART
	{file: "52_mining_mech.bgra", w: 131, h: 120},        // set 7, item 5: MINING MECH
	{file: "53_robotic_claw.bgra", w: 132, h: 125},       // set 7, item 6: ROBOTIC CLAW
	{file: "54_ore_conveyor.bgra", w: 135, h: 117},       // set 7, item 7: ORE CONVEYOR
	{file: "55_processing_canister.bgra", w: 83, h: 110}, // set 7, item 8: PROCESSING CANISTER
	{file: "56_void_portal.bgra", w: 177, h: 126},        // set 8, item 1: VOID PORTAL
	{file: "57_void_crystal.bgra", w: 108, h: 124},       // set 8, item 2: VOID CRYSTAL
	{file: "58_vault_cube.bgra", w: 107, h: 120},         // set 8, item 3: VAULT CUBE
	{file: "59_void_spiral.bgra", w: 112, h: 119},        // set 8, item 4: VOID SPIRAL
	{file: "60_rift_drive.bgra", w: 106, h: 113},         // set 8, item 5: RIFT DRIVE
	{file: "61_containment_tube.bgra", w: 114, h: 122},   // set 8, item 6: CONTAINMENT TUBE
	{file: "62_phase_ring.bgra", w: 112, h: 132},         // set 8, item 7: PHASE RING
	{file: "63_void_gem.bgra", w: 101, h: 113},           // set 8, item 8: VOID GEM
	{file: "64_defence_platform.bgra", w: 145, h: 134},   // set 9, item 1: DEFENCE PLATFORM
	{file: "65_pulse_cannon.bgra", w: 131, h: 129},       // set 9, item 2: PULSE CANNON
	{file: "66_orbital_missile.bgra", w: 99, h: 128},     // set 9, item 3: ORBITAL MISSILE
	{file: "67_satellite_turret.bgra", w: 133, h: 132},   // set 9, item 4: SATELLITE TURRET
	{file: "68_targeting_terminal.bgra", w: 115, h: 115}, // set 9, item 5: TARGETING TERMINAL
	{file: "69_missile_rack.bgra", w: 129, h: 127},       // set 9, item 6: MISSILE RACK
	{file: "70_orbital_mine.bgra", w: 104, h: 122},       // set 9, item 7: ORBITAL MINE
	{file: "71_missile_battery.bgra", w: 126, h: 133},    // set 9, item 8: MISSILE BATTERY
}

func loadAFKModuleAssets() {
	for i := range afkModuleSprites {
		s := &afkModuleSprites[i]
		s.data = readExternalBytes("ui", "ship_modules", s.file)
	}
}

func drawAFKModuleSprite(hdc uintptr, r RECT, set, item int, crafted bool) bool {
	idx := afkModuleIndex(set, item)
	if idx < 0 || idx >= len(afkModuleSprites) || r.Right <= r.Left || r.Bottom <= r.Top {
		return false
	}
	s := &afkModuleSprites[idx]
	if len(s.data) < int(s.w*s.h*4) {
		return false
	}
	sprite := ensureRuntimeSprite(hdc, s.data, s.w, s.h)
	if sprite == nil || sprite.dc == 0 {
		return false
	}
	crop := alphaBoundsBGRA(s.data, s.w, s.h, 5)
	cropW, cropH := crop.Right-crop.Left, crop.Bottom-crop.Top
	if cropW <= 0 || cropH <= 0 {
		return false
	}
	boxW, boxH := r.Right-r.Left, r.Bottom-r.Top
	// v465: keep module art noticeably smaller and close to native pixel size.
	// The old fit-to-box path enlarged several sprites and softened the pixel art.
	// At most 76% of the available art region is used, and artwork is never
	// enlarged beyond its authored crop size.
	scale := float64(boxW) * 0.76 / float64(cropW)
	if sh := float64(boxH) * 0.76 / float64(cropH); sh < scale {
		scale = sh
	}
	if scale > 1.0 {
		scale = 1.0
	}
	if scale <= 0 {
		return false
	}
	dstW := int32(float64(cropW)*scale + 0.5)
	dstH := int32(float64(cropH)*scale + 0.5)
	dstX := r.Left + (boxW-dstW)/2
	dstY := r.Top + (boxH-dstH)/2
	alpha := byte(255)
	if !crafted {
		// Keep every supplied sprite visible before crafting, but clearly locked.
		alpha = 92
	}
	blend := uintptr(uint32(AC_SRC_OVER) | uint32(alpha)<<16 | uint32(AC_SRC_ALPHA)<<24)
	alphaBlend.Call(hdc, uintptr(dstX), uintptr(dstY), uintptr(dstW), uintptr(dstH),
		sprite.dc, uintptr(crop.Left), uintptr(crop.Top), uintptr(cropW), uintptr(cropH), blend)
	return true
}
