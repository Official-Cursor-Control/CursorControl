from pathlib import Path
s=Path('afk_singularity.go').read_text()
m=Path('main.go').read_text()
p=Path('progression_achievements.go').read_text()
checks={
 'starbase dedicated timer':'case TIMER_STARBASE:' in m and 'setTimer.Call(mainHwnd, TIMER_STARBASE, 33, 0)' in p,
 'ui timer repaint fallback':'OverlayAFKSingularity' in m and 'case TIMER_UI:' in m,
 'global particle timer repaint fallback':'case TIMER_PARTICLES:' in m and 'if overlayMode == OverlayAFKSingularity' in m,
 'animation clock is absolute and non-resetting':'return float64(time.Now().UnixMilli()) / 1000.0' in s,
 'core motion ignores reduced-motion freeze':'func afkStarbaseVisualElapsedSeconds' in s and 'if gameMeta.ReducedMotion {\n\t\treturn 0' not in s,
 'software pixel rotation':'func afkRotateSingularitySource' in s and 'Inverse map output -> source for clockwise output rotation' in s,
 'no starbase GDI world-transform rotation':'drawBoss2RotatedBGRAAlpha(hdc, starbaseSingularityBGRA' not in s,
 'mutable rotated dib refresh':'afkSingularitySurfaceDegree != degree' in s and 'afkSingularityRotationBits' in s,
 'clockwise 24 second cycle':'afkStarbaseVisualElapsedSeconds()*15.0' in s,
 '80 percent singularity alpha':'starbaseSingularityOpacity byte = 204' in s,
 'background persistent sprite':'ensureRuntimeSprite(hdc, starbaseBackgroundBGRA, srcW, srcH)' in s,
 'background two tile alpha blend':'for k := 0; k < 2; k++' in s and 'x0 := r.Left - int32(math.Round(phase))' in s,
 'background fractional phase accumulates before rounding':'math.Mod(afkStarbaseVisualElapsedSeconds()*34.0, float64(dstW))' in s,
 'draw order background before singularity':'drawAFKScrollingArtBackground(hdc, field, w, hgt)' in s and s.index('drawAFKScrollingArtBackground(hdc, field, w, hgt)') < s.index('drawAFKRotatingSingularityBackdrop(hdc, w, hgt)'),
 'single starbase background renderer':s.count('drawAFKScrollingArtBackground(hdc, field, w, hgt)') == 1,
 'single starbase singularity renderer':s.count('drawAFKRotatingSingularityBackdrop(hdc, w, hgt)') == 1,
 'particle independent velocities':'VX, VY float64' in s,
 'particle movement not frozen by reduced motion':'Starbits are part of the core Starbase world simulation' in s,
 'particle pair collisions':'for j := i + 1; j < len(afkSingularityFreeParticles); j++' in s,
 'particle overlap separation':'overlap := minD - d' in s,
 'particle elastic response':'equal masses exchange normal velocity' in s,
 'particle circular wall':'Circular wall collision' in s and 'p.VX -= 2 * dot * nx' in s,
 'rotation resources released':'releaseAFKSingularityRotationSurface()' in m,
}
failed=[]
for name,ok in checks.items():
 print(('PASS' if ok else 'FAIL'),'-',name)
 if not ok: failed.append(name)
print(f'\n{len(checks)-len(failed)}/{len(checks)} Starbase animation/physics assertions passed')
raise SystemExit(1 if failed else 0)
