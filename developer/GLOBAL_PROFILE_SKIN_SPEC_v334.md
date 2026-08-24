# Global Profile Skin Overlay Spec (v334)

- Source canvas: **1124 x 174 pixels**
- Format for design handoff: transparent PNG, RGBA.
- Runtime conversion target: premultiplied BGRA, exactly 1124 x 174 x 4 bytes.
- Runtime filenames: `assets/ui/profile_frames/profile_frame_1.bgra` through `profile_frame_7.bgra`.
- The skin is drawn across the entire Global Profile identity panel.
- Draw order: base profile panel -> selected skin overlay -> avatar/rank/text/profile information.
- Transparent regions reveal the standard profile panel below.
- Keep information-heavy areas transparent or low-detail; borders/decorative artwork can use the full canvas.
