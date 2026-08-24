//go:build windows

package main

type afkIconSprite struct {
	data []byte
	w, h int32
}

var afkTechLabIcons = map[string]*afkIconSprite{
	"void_cursor":             {w: 371, h: 344},
	"blackstone_routing":      {w: 372, h: 336},
	"snare_algorithm":         {w: 362, h: 336},
	"singularity_pointer":     {w: 356, h: 339},
	"stealth_pathing":         {w: 363, h: 335},
	"pixel_compression_field": {w: 360, h: 328},
	"cursor_prism":            {w: 361, h: 325},
	"overdrive_regulators":    {w: 360, h: 347},
	"quantum_conduit":         {w: 370, h: 330},
	"convergence":             {w: 364, h: 331},
}

var afkPilotNetworkBackground = &afkIconSprite{w: 2400, h: 1600}

var afkSkillTreeIcons = map[string]*afkIconSprite{
	"sharper_cursors_active":      {w: 150, h: 147},
	"sharper_cursors_locked":      {w: 147, h: 148},
	"orbital_cursor_math_active":  {w: 150, h: 147},
	"orbital_cursor_math_locked":  {w: 146, h: 148},
	"tap_precision_active":        {w: 150, h: 150},
	"tap_precision_locked":        {w: 150, h: 150},
	"deep_space_navigator_active": {w: 152, h: 150},
	"deep_space_navigator_locked": {w: 146, h: 149},
	"cache_hunter_active":         {w: 161, h: 148},
	"cache_hunter_locked":         {w: 149, h: 149},
	"cursor_supremacy_active":     {w: 150, h: 146},
	"cursor_supremacy_locked":     {w: 148, h: 149},
	"overdrive_charge_active":     {w: 154, h: 150},
	"overdrive_charge_locked":     {w: 144, h: 150},
	"overdrive_extension_active":  {w: 154, h: 150},
	"overdrive_extension_locked":  {w: 147, h: 150},
	"nav_data_analysis_active":    {w: 156, h: 149},
	"nav_data_analysis_locked":    {w: 150, h: 148},
	"signal_intelligence_active":  {w: 155, h: 147},
	"signal_intelligence_locked":  {w: 145, h: 148},
	"rapid_dispatch_active":       {w: 145, h: 148},
	"rapid_dispatch_locked":       {w: 143, h: 147},
	"magnetic_attractor_active":   {w: 156, h: 147},
	"magnetic_attractor_locked":   {w: 148, h: 145},
	"orbital_drones_active":       {w: 149, h: 154},
	"orbital_drones_locked":       {w: 149, h: 154},
	"stealth_protocol_active":     {w: 153, h: 147},
	"stealth_protocol_locked":     {w: 148, h: 147},
	"question_locked":             {w: 146, h: 148},
}

func loadAFKResearchAssets() {
	afkPilotNetworkBackground.data = readExternalBytes("ui", "pilot_skills", "pilot_network_background.bgra")
	for name, s := range afkTechLabIcons {
		if s != nil {
			s.data = readExternalBytes("ui", "tech_lab_icons", name+".bgra")
		}
	}
	for name, s := range afkSkillTreeIcons {
		if s != nil {
			s.data = readExternalBytes("ui", "skill_tree_icons", name+".bgra")
		}
	}
}

func drawAFKIconSprite(hdc uintptr, s *afkIconSprite, r RECT) bool {
	if s == nil || len(s.data) < int(s.w*s.h*4) || r.Right <= r.Left || r.Bottom <= r.Top {
		return false
	}
	drawRawBGRATrimmedFit(hdc, s.data, s.w, s.h, r)
	return true
}

func afkTechIconForResearch(i int) *afkIconSprite {
	names := []string{
		"void_cursor",
		"blackstone_routing",
		"snare_algorithm",
		"singularity_pointer",
		"stealth_pathing",
		"pixel_compression_field",
		"cursor_prism",
		"overdrive_regulators",
		"quantum_conduit",
		"convergence",
	}
	if i < 0 || i >= len(names) {
		return nil
	}
	return afkTechLabIcons[names[i]]
}

func afkSkillIconName(i int, state string) string {
	base := []string{
		"sharper_cursors", "orbital_cursor_math", "tap_precision", "deep_space_navigator",
		"cache_hunter", "cursor_supremacy", "overdrive_charge", "overdrive_extension",
		"nav_data_analysis", "signal_intelligence", "rapid_dispatch", "magnetic_attractor",
		"orbital_drones", "stealth_protocol",
	}
	if state == "hidden" {
		return "question_locked"
	}
	if i < 0 || i >= len(base) {
		return ""
	}
	if state == "active" {
		return base[i] + "_active"
	}
	return base[i] + "_locked"
}
