//go:build windows

package main

import "time"

// Section 12 — final presentation layer. Milestones are queued once and drawn
// as short pixel banners; no visual effect itself mutates economy state.
type afkMilestoneBanner struct {
	Title, Detail string
	Started       time.Time
}

var afkMilestones []afkMilestoneBanner

func afkQueueMilestone(title, detail string) {
	if title == "" {
		return
	}
	now := time.Now()
	// Do not stack identical banners from repeated UI input.
	if len(afkMilestones) > 0 && afkMilestones[len(afkMilestones)-1].Title == title {
		return
	}
	afkMilestones = append(afkMilestones, afkMilestoneBanner{title, detail, now})
	if len(afkMilestones) > 4 {
		afkMilestones = afkMilestones[len(afkMilestones)-4:]
	}
}

func drawAFKMilestoneBanner(hdc uintptr, w, hgt int32) {
	if len(afkMilestones) == 0 {
		return
	}
	now := time.Now()
	b := afkMilestones[0]
	age := now.Sub(b.Started)
	if age > 2600*time.Millisecond {
		afkMilestones = afkMilestones[1:]
		return
	}
	r := RECT{sx(360, w), sy(205, hgt), w - sx(360, w), sy(286, hgt)}
	drawBevelPanel(hdc, r, rgb(20, 18, 54), rgb(242, 192, 55), rgb(2, 5, 18), 3)
	if hudSmallFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudSmallFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(250, 221, 103))
		centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(17, hgt), b.Title)
		selectObject.Call(hdc, old)
	}
	if hudTinyFont != 0 {
		old, _, _ := selectObject.Call(hdc, hudTinyFont)
		setBkMode.Call(hdc, TRANSPARENT)
		setTextColor.Call(hdc, rgb(207, 231, 245))
		centeredTextOut(hdc, r.Left, r.Right, r.Top+sy(49, hgt), b.Detail)
		selectObject.Call(hdc, old)
	}
}

func afkEnsureSection12State() {
	if gameMeta.AFKSection11Complete {
		gameMeta.AFKSection12Complete = true
	}
}
