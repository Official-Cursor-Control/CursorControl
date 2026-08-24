//go:build windows

package main

import (
	"bytes"
	"image"
	"image/draw"
	"image/gif"
	"net/url"
	stdpath "path"
	"strings"
	"time"
)

type AvatarAnimation struct {
	Frames  [][]byte
	Delays  []time.Duration
	Width   int32
	Height  int32
	Started time.Time
	Total   time.Duration
}

var discordAvatarAnim AvatarAnimation
var remoteAvatarAnim AvatarAnimation

func discordAnimatedAvatarURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return raw
	}
	base := stdpath.Base(u.Path)
	if strings.HasPrefix(base, "a_") || strings.Contains(u.Path, "/a_") {
		ext := stdpath.Ext(u.Path)
		if ext != "" {
			u.Path = strings.TrimSuffix(u.Path, ext) + ".gif"
		}
		q := u.Query()
		if q.Get("size") == "" {
			q.Set("size", "256")
		}
		u.RawQuery = q.Encode()
		return u.String()
	}
	return raw
}

func imageToPremultipliedBGRA(img image.Image) ([]byte, int32, int32) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, 0, 0
	}
	out := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			a8 := uint32(a >> 8)
			r8 := uint32(r >> 8)
			g8 := uint32(g >> 8)
			b8 := uint32(bl >> 8)
			// Win32 AlphaBlend expects premultiplied colour channels.
			r8 = r8 * a8 / 255
			g8 = g8 * a8 / 255
			b8 = b8 * a8 / 255
			i := (y*w + x) * 4
			out[i] = byte(b8)
			out[i+1] = byte(g8)
			out[i+2] = byte(r8)
			out[i+3] = byte(a8)
		}
	}
	return out, int32(w), int32(h)
}

func decodeDiscordAvatar(data []byte) ([]byte, int32, int32, AvatarAnimation, error) {
	if g, err := gif.DecodeAll(bytes.NewReader(data)); err == nil && len(g.Image) > 0 {
		w, h := g.Config.Width, g.Config.Height
		if w > 0 && h > 0 && w <= 2048 && h <= 2048 {
			canvas := image.NewNRGBA(image.Rect(0, 0, w, h))
			frames := make([][]byte, 0, len(g.Image))
			delays := make([]time.Duration, 0, len(g.Image))
			total := time.Duration(0)
			for i, fr := range g.Image {
				before := image.NewNRGBA(canvas.Bounds())
				draw.Draw(before, before.Bounds(), canvas, image.Point{}, draw.Src)
				draw.Draw(canvas, fr.Bounds(), fr, fr.Bounds().Min, draw.Over)
				pix, _, _ := imageToPremultipliedBGRA(canvas)
				frames = append(frames, pix)
				d := 10 * time.Millisecond
				if i < len(g.Delay) && g.Delay[i] > 0 {
					d = time.Duration(g.Delay[i]) * 10 * time.Millisecond
				}
				delays = append(delays, d)
				total += d
				if i < len(g.Disposal) {
					switch g.Disposal[i] {
					case gif.DisposalBackground:
						draw.Draw(canvas, fr.Bounds(), image.Transparent, image.Point{}, draw.Src)
					case gif.DisposalPrevious:
						draw.Draw(canvas, canvas.Bounds(), before, image.Point{}, draw.Src)
					}
				}
			}
			// v352: animated Discord avatars intentionally use their first decoded GIF frame
			// as a stable still image. This avoids platform-specific animation playback issues
			// while preserving the user's actual Discord avatar artwork.
			_ = delays
			_ = total
			return frames[0], int32(w), int32(h), AvatarAnimation{}, nil
		}
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, AvatarAnimation{}, err
	}
	pix, w, h := imageToPremultipliedBGRA(img)
	return pix, w, h, AvatarAnimation{}, nil
}

func avatarAnimationFrame(a AvatarAnimation, fallback []byte) []byte {
	if len(a.Frames) < 2 || a.Total <= 0 || a.Started.IsZero() {
		return fallback
	}
	t := time.Since(a.Started) % a.Total
	acc := time.Duration(0)
	for i, d := range a.Delays {
		acc += d
		if t < acc && i < len(a.Frames) {
			return a.Frames[i]
		}
	}
	return a.Frames[len(a.Frames)-1]
}

func animatedAvatarVisible() bool {
	if overlayMode == OverlayProfile {
		authMu.Lock()
		ok := len(discordAvatarAnim.Frames) > 1 || strings.Contains(strings.ToLower(discordAvatarURL), "/a_")
		authMu.Unlock()
		return ok
	}
	if overlayMode == OverlayRemoteProfile {
		remoteProfileMu.Lock()
		ok := len(remoteAvatarAnim.Frames) > 1 || strings.Contains(strings.ToLower(remoteProfile.AvatarURL), "/a_")
		remoteProfileMu.Unlock()
		return ok
	}
	return false
}
