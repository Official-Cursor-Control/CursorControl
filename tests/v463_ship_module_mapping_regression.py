from pathlib import Path
import re
R=Path(__file__).resolve().parents[1]
A=(R/'afk_module_assets.go').read_text()
M=(R/'afk_modules.go').read_text()
B=(R/'build_identity.go').read_text()
files=re.findall(r'\{file: "([^"]+\.bgra)"',A)
expected_files=['00_holo_platform.bgra', '01_operator_id.bgra', '02_stasis_pod.bgra', '03_crew_crate.bgra', '04_comms_headset.bgra', '05_operator_seat.bgra', '06_support_drone.bgra', '07_holo_card.bgra', '08_collector_ring.bgra', '09_generator_coil.bgra', '10_star_crystal.bgra', '11_energy_canister.bgra', '12_reactor_core.bgra', '13_starbit_bars.bgra', '14_starbit_orb.bgra', '15_collector_unit.bgra', '16_lab_platform.bgra', '17_research_chip.bgra', '18_sample_vial.bgra', '19_robotic_arm.bgra', '20_research_terminal.bgra', '21_scan_dish.bgra', '22_quantum_canister.bgra', '23_tech_crate.bgra', '24_hydro_hull.bgra', '25_mini_sub.bgra', '26_turbine_fan.bgra', '27_oxygen_canister.bgra', '28_hydro_drill.bgra', '29_solar_array.bgra', '30_scanner_module.bgra', '31_sonar_dome.bgra', '32_lounge_habitat.bgra', '33_lounge_chair.bgra', '34_sleep_pod.bgra', '35_plant_terrarium.bgra', '36_refreshment_unit.bgra', '37_fitness_module.bgra', '38_game_console.bgra', '39_jukebox.bgra', '40_arcade_cabinet.bgra', '41_racing_seat.bgra', '42_vr_visor.bgra', '43_game_cartridge.bgra', '44_joystick.bgra', '45_claw_machine.bgra', '46_token_chest.bgra', '47_dance_pad.bgra', '48_extractor_base.bgra', '49_drill_bit.bgra', '50_cargo_crate.bgra', '51_ore_cart.bgra', '52_mining_mech.bgra', '53_robotic_claw.bgra', '54_ore_conveyor.bgra', '55_processing_canister.bgra', '56_void_portal.bgra', '57_void_crystal.bgra', '58_vault_cube.bgra', '59_void_spiral.bgra', '60_rift_drive.bgra', '61_containment_tube.bgra', '62_phase_ring.bgra', '63_void_gem.bgra', '64_defence_platform.bgra', '65_pulse_cannon.bgra', '66_orbital_missile.bgra', '67_satellite_turret.bgra', '68_targeting_terminal.bgra', '69_missile_rack.bgra', '70_orbital_mine.bgra', '71_missile_battery.bgra']
expected_labels=['HOLO PLATFORM', 'OPERATOR ID', 'STASIS POD', 'CREW CRATE', 'COMMS HEADSET', 'OPERATOR SEAT', 'SUPPORT DRONE', 'HOLO CARD', 'COLLECTOR RING', 'GENERATOR COIL', 'STAR CRYSTAL', 'ENERGY CANISTER', 'REACTOR CORE', 'STARBIT BARS', 'STARBIT ORB', 'COLLECTOR UNIT', 'LAB PLATFORM', 'RESEARCH CHIP', 'SAMPLE VIAL', 'ROBOTIC ARM', 'RESEARCH TERMINAL', 'SCAN DISH', 'QUANTUM CANISTER', 'TECH CRATE', 'HYDRO HULL', 'MINI SUB', 'TURBINE FAN', 'OXYGEN CANISTER', 'HYDRO DRILL', 'SOLAR ARRAY', 'SCANNER MODULE', 'SONAR DOME', 'LOUNGE HABITAT', 'LOUNGE CHAIR', 'SLEEP POD', 'PLANT TERRARIUM', 'REFRESHMENT UNIT', 'FITNESS MODULE', 'GAME CONSOLE', 'JUKEBOX', 'ARCADE CABINET', 'RACING SEAT', 'VR VISOR', 'GAME CARTRIDGE', 'JOYSTICK', 'CLAW MACHINE', 'TOKEN CHEST', 'DANCE PAD', 'EXTRACTOR BASE', 'DRILL BIT', 'CARGO CRATE', 'ORE CART', 'MINING MECH', 'ROBOTIC CLAW', 'ORE CONVEYOR', 'PROCESSING CANISTER', 'VOID PORTAL', 'VOID CRYSTAL', 'VAULT CUBE', 'VOID SPIRAL', 'RIFT DRIVE', 'CONTAINMENT TUBE', 'PHASE RING', 'VOID GEM', 'DEFENCE PLATFORM', 'PULSE CANNON', 'ORBITAL MISSILE', 'SATELLITE TURRET', 'TARGETING TERMINAL', 'MISSILE RACK', 'ORBITAL MINE', 'MISSILE BATTERY']
labels=[]
for line in M.splitlines():
    if '[8]string{' in line and any(s in line for s in ['OPERATOR LOCKER','STARBIT GENERATOR','TECH LAB','HYDRO BAY','STAR LOUNGE','QUANTUM ARCADE','EXTRACTION DEPOT','VOID VAULT','ORBITAL ARSENAL']):
        tail=line.split('[8]string{',1)[1].rsplit('}',2)[0]
        labels.extend(re.findall(r'"([^"]+)"',tail))
checks={
    'current build identity present': 'clientBuildVersion = "v465"' in B and 'clientBuildNumber  = 405' in B,
    '72 sprite files remain': len(files)==72,
    'sprite files follow exact source-grid order': files==expected_files,
    '72 display labels remain': len(labels)==72,
    'labels follow exact source-grid order': labels==expected_labels,
    'save/recipe index formula unchanged': 'return set*afkModuleItemsPerSet + item' in M,
}
for n,ok in checks.items(): print(('PASS' if ok else 'FAIL')+' - '+n)
raise SystemExit(0 if all(checks.values()) else 1)
