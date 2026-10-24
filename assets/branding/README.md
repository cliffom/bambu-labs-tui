# Branding artwork

The icon combines a terminal prompt with a bamboo shoot. All artwork uses a limited palette, solid pixels, and nearest-neighbor scaling.

| File | Purpose |
| --- | --- |
| `icon-32.aseprite` | Layered 32 × 32 master icon |
| `icon.aseprite` | Layered 64 × 64 icon, scaled 2× from the master |
| `icon-32.png` | Small icon with transparent background |
| `icon-64.png` | Standard icon with transparent background |
| `icon-512.png` | Large icon with transparent background, scaled 16× from the master |
| `hero.aseprite` | Layered 400 × 140 banner source with the approved icon |
| `hero.png` | 1200 × 420 README banner, scaled 3× from its source |

Keep the prompt symmetric, the right-facing leaves inside the bezel, and only the central shoot crossing the top border. Use whole-number scaling without smoothing when exporting.

To export with the Aseprite CLI from the repository root:

```sh
aseprite --batch assets/branding/icon-32.aseprite --save-as assets/branding/icon-32.png
aseprite --batch assets/branding/icon.aseprite --save-as assets/branding/icon-64.png
aseprite --batch assets/branding/icon.aseprite --scale 8 --save-as assets/branding/icon-512.png
aseprite --batch assets/branding/hero.aseprite --scale 3 --save-as assets/branding/hero.png
```

The artwork is covered by the repository's MIT license.
