# Branding artwork

[Documentation index](../README.md)

The logo and hero source and exported files below live in [assets/branding/](../../assets/branding/).

The icon combines a terminal prompt with a bamboo shoot. All artwork uses a limited palette, solid pixels, and nearest-neighbor scaling.

| File | Purpose |
| --- | --- |
| `icon-32.aseprite` | Layered 32 × 32 master icon |
| `icon.aseprite` | Layered 64 × 64 icon, scaled 2× from the master |
| `icon-32.png` | Small icon with transparent background |
| `icon-64.png` | Standard icon with transparent background |
| `icon-512.png` | Large icon with transparent background, scaled 16× from the master |
| `hero.aseprite` | Layered 400 × 140 BambuStand banner source with the approved icon and editable lettering |
| `hero.png` | 1200 × 420 README banner (“Your farm. Two views.”), scaled 3× from its source |

Keep the prompt symmetric, the right-facing leaves inside the bezel, and only the central shoot crossing the top border. Use whole-number scaling without smoothing when exporting.

To export with the Aseprite CLI from the repository root:

```sh
aseprite --batch assets/branding/icon-32.aseprite --save-as assets/branding/icon-32.png
aseprite --batch assets/branding/icon.aseprite --save-as assets/branding/icon-64.png
aseprite --batch assets/branding/icon.aseprite --scale 8 --save-as assets/branding/icon-512.png
aseprite --batch assets/branding/hero.aseprite --scale 3 --save-as assets/branding/hero.png
```

The artwork is covered by the repository's MIT license.

## Printer sprites

The optional web dashboard uses five generated pixel-style hardware depictions in [the embedded printer assets](../../internal/webserver/assets/printers/), matching the logo/hero's charcoal, muted teal, sage, and mint visual family. Supported illustrations are H2D, X2D, X1 Carbon, P1S, and A1 mini. They are decorative model references: artwork does not establish printer state, active accessories, installed options, or hardware validation. Unknown model names retain the generic illustration.

Built-in image generation produced one transparent PNG per model on 2026-10-26, using the matching official Bambu Studio model render for hardware shape and this repository's icon/hero for style. Reference render files were used during generation and are not bundled in the project. The [exact prompt set](printer-sprite-prompts.md) records the generation brief. Keep the existing logo/hero source untouched when revising printer art.

| Model | Official hardware reference | Generated asset |
| --- | --- | --- |
| H2D | [Bambu Lab H2D](https://github.com/bambulab/BambuStudio/blob/master/resources/profiles/BBL/Bambu%20Lab%20H2D_cover.png) | [h2d PNG](../../internal/webserver/assets/printers/h2d-ab6291473de5.png) |
| X2D | [Bambu Lab X2D](https://github.com/bambulab/BambuStudio/blob/master/resources/profiles/BBL/Bambu%20Lab%20X2D_cover.png) | [x2d PNG](../../internal/webserver/assets/printers/x2d-b7f68caf0fdf.png) |
| X1 Carbon | [Bambu Lab X1 Carbon](https://github.com/bambulab/BambuStudio/blob/master/resources/profiles/BBL/Bambu%20Lab%20X1%20Carbon_cover.png) | [x1c PNG](../../internal/webserver/assets/printers/x1c-72b4402798e9.png) |
| P1S | [Bambu Lab P1S](https://github.com/bambulab/BambuStudio/blob/master/resources/profiles/BBL/Bambu%20Lab%20P1S_cover.png) | [p1s PNG](../../internal/webserver/assets/printers/p1s-1cf611b647f6.png) |
| A1 mini | [Bambu Lab A1 mini](https://github.com/bambulab/BambuStudio/blob/master/resources/profiles/BBL/Bambu%20Lab%20A1%20mini_cover.png) | [a1-mini PNG](../../internal/webserver/assets/printers/a1-mini-99cd4423fabe.png) |

Preserve PNG alpha, transparent padding, recognizable model silhouettes, and text-free artwork. Render with CSS `image-rendering: pixelated` and `object-fit: contain`; never stretch a sprite or apply telemetry state to its colors. Content-versioned filenames let the server cache these public illustrations locally without caching telemetry snapshots. When replacing artwork, use a new content digest in its filename, update the model allowlist, and inspect it at actual card sizes on desktop and phone layouts. Do not infer a supported model from a user-assigned printer name or a partial model-name match.
