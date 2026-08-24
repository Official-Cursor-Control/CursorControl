//go:build windows

package main

import (
	"sync"
	"sync/atomic"
)

// Profile writes are coalesced through one background worker. Rapid UI actions
// (equipping cosmetics, purchasing fire options, achievement unlocks) used to be
// able to launch overlapping Supabase requests. Serialising them keeps networking
// completely off the game thread and prevents stale responses/races while still
// guaranteeing that a change made during an active sync triggers one final sync.
var (
	profileSyncOnce                   sync.Once
	profileSyncReq                    chan struct{}
	garageFireColorSelectionDirty     atomic.Uint64
	garageFireSizeSelectionDirty      atomic.Uint64
	achievementShowcaseSelectionDirty atomic.Uint64
	profileTitleSelectionDirty        atomic.Uint64
	profileNameColourSelectionDirty   atomic.Uint64
	profileFrameSelectionDirty        atomic.Uint64
)

func markGarageFireColorSelectionDirty()     { garageFireColorSelectionDirty.Add(1) }
func markGarageFireSizeSelectionDirty()      { garageFireSizeSelectionDirty.Add(1) }
func markAchievementShowcaseSelectionDirty() { achievementShowcaseSelectionDirty.Add(1) }
func markProfileTitleSelectionDirty()        { profileTitleSelectionDirty.Add(1) }
func markProfileNameColourSelectionDirty()   { profileNameColourSelectionDirty.Add(1) }
func markProfileFrameSelectionDirty()        { profileFrameSelectionDirty.Add(1) }

func startPlayerProfileSyncWorker() {
	profileSyncOnce.Do(func() {
		profileSyncReq = make(chan struct{}, 1)
		go func() {
			for range profileSyncReq {
				syncPlayerProfile()
			}
		}()
	})
}

func requestPlayerProfileSync() {
	startPlayerProfileSyncWorker()
	select {
	case profileSyncReq <- struct{}{}:
	default:
		// A sync is already pending/running. The current local state is read by the
		// worker, so duplicate requests need not create another network goroutine.
	}
}
