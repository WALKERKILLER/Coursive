#!/usr/bin/env bash
# export-brand-icons.sh — regenerate every Coursive icon from the 4096 px master.
#
# Usage: scripts/export-brand-icons.sh
# Needs ImageMagick 7 (`magick`) and the Inter font for the social preview.
# Output is written under assets/brand/; see assets/brand/README.md for the map.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
brand="$root/assets/brand"
src="$brand/source/coursive-logo-4096.png"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# png <magick arguments…> <out>: run magick, writing <out> without metadata or timestamps.
png() { magick "${@:1:$#-1}" -strip -define png:compression-level=9 -define png:exclude-chunks=date,time "${@: -1}"; }

# Working layers at 1024 px:
#   full   the master, full bleed and opaque
#   clean  the background alone: the mark's area refilled by normalized blur of what surrounds it
#   mask   the mark's coverage: its distance from the clean background
#   mark   the gradient mark alone on transparency, its edge colours unblended from the background
png "$src" -filter Lanczos -resize 1024x1024 "$tmp/full.png"
# refill <keep> <out>: replace pixels outside <keep> (white = background) with a blur of the kept ones.
refill() {
  png "$tmp/full.png" "$1" -compose Multiply -composite -blur 0x64 "$tmp/num.png"
  png "$1" -blur 0x64 "$tmp/den.png"
  png "$tmp/num.png" "$tmp/den.png" -fx 'v > 0.002 ? u / v : u' "$tmp/fill.png"
  png "$tmp/full.png" "$tmp/fill.png" \( "$1" -negate -blur 0x3 \) -composite "$2"
}
# Pass 1: the saturated core of the mark is enough to rebuild an approximate background.
png "$tmp/full.png" -colorspace HSV -channel G -separate +channel -threshold 40% -morphology Dilate Disk:12 -negate "$tmp/keep.png"
refill "$tmp/keep.png" "$tmp/clean1.png"
# Pass 2: anything that differs from that background belongs to the mark, including its light strokes.
png "$tmp/full.png" "$tmp/clean1.png" -compose Difference -composite -separate -evaluate-sequence Max -level 3%,16% "$tmp/mask.png"
png "$tmp/mask.png" -threshold 1% -morphology Dilate Disk:4 -negate "$tmp/keep.png"
refill "$tmp/keep.png" "$tmp/clean.png"
png "$tmp/full.png" "$tmp/clean.png" "$tmp/mask.png" -fx 'cov = u[2]; cov > 0.004 ? (u - (1 - cov) * v) / cov : 0' "$tmp/ink.png"
png "$tmp/ink.png" "$tmp/mask.png" -alpha off -compose CopyOpacity -composite "$tmp/mark.png"

# Shapes: a continuous-corner square (22.5% radius) and a circle.
rounded() { # rounded <in> <size> <out>
  local r=$(( $2 * 225 / 1000 ))
  png "$1" -filter Lanczos -resize "$2x$2" \
    \( -size "$2x$2" xc:none -fill white -draw "roundrectangle 0,0 $(( $2 - 1 )),$(( $2 - 1 )) $r,$r" \) \
    -compose DstIn -composite "$3"
}
circle() { # circle <in> <size> <out>
  png "$1" -filter Lanczos -resize "$2x$2" \
    \( -size "$2x$2" xc:none -fill white -draw "circle $(( $2 / 2 )),$(( $2 / 2 )) $(( $2 / 2 )),0" \) \
    -compose DstIn -composite "$3"
}
# place <in> <canvas> <art> <out>: centre <in> scaled to <art> px on a transparent <canvas>.
place() { png "$1" -filter Lanczos -resize "$3x$3" -background none -gravity center -extent "$2x$2" "$4"; }
# extend <in> <canvas> <art> <out>: like place, but continue the edge pixels to fill the canvas.
extend() {
  local off=$(( ($2 - $3) / 2 ))
  png "$1" -filter Lanczos -resize "$3x$3" -virtual-pixel Edge \
    -define distort:viewport="$2x$2-$off-$off" -distort SRT 0 +repage "$4"
}

rm -rf "$brand/logo" "$brand/platforms" "$brand/social"
mkdir -p "$brand/logo" "$brand/social"

# Logo: README, docs, store listings and anywhere a plain image is needed.
for s in 1024 512 256 128; do
  png "$src" -filter Lanczos -resize "${s}x${s}" -alpha off "$brand/logo/coursive-square-$s.png"
  rounded "$tmp/full.png" "$s" "$brand/logo/coursive-rounded-$s.png"
done
png "$tmp/mark.png" "$brand/logo/coursive-mark-1024.png"
png "$tmp/mask.png" -background white -alpha shape "$brand/logo/coursive-mark-white-1024.png"
png "$tmp/mask.png" -background "#0E2A4D" -alpha shape "$brand/logo/coursive-mark-ink-1024.png"

# Android: legacy launcher icons, adaptive layers (art fills the 72 dp viewport of the
# 108 dp canvas), a themed-icon monochrome layer, and the Play Store listing icon.
android="$brand/platforms/android/res"
for d in mdpi:1 hdpi:1.5 xhdpi:2 xxhdpi:3 xxxhdpi:4; do
  name="${d%%:*}"; f="${d#*:}"
  dir="$android/mipmap-$name"; mkdir -p "$dir"
  legacy=$(awk "BEGIN{print int(48*$f+0.5)}"); body=$(awk "BEGIN{print int(44*$f+0.5)}")
  canvas=$(awk "BEGIN{print int(108*$f+0.5)}"); art=$(awk "BEGIN{print int(72*$f+0.5)}")
  rounded "$tmp/full.png" "$body" "$tmp/legacy.png"; place "$tmp/legacy.png" "$legacy" "$body" "$dir/ic_launcher.png"
  circle "$tmp/full.png" "$body" "$tmp/round.png"; place "$tmp/round.png" "$legacy" "$body" "$dir/ic_launcher_round.png"
  extend "$tmp/clean.png" "$canvas" "$art" "$tmp/bg.png"; png "$tmp/bg.png" -alpha off "$dir/ic_launcher_background.png"
  place "$tmp/mark.png" "$canvas" "$art" "$dir/ic_launcher_foreground.png"
  place "$brand/logo/coursive-mark-white-1024.png" "$canvas" "$art" "$dir/ic_launcher_monochrome.png"
done
mkdir -p "$android/mipmap-anydpi-v26"
for n in ic_launcher ic_launcher_round; do
  cat > "$android/mipmap-anydpi-v26/$n.xml" <<'XML'
<?xml version="1.0" encoding="utf-8"?>
<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">
    <background android:drawable="@mipmap/ic_launcher_background" />
    <foreground android:drawable="@mipmap/ic_launcher_foreground" />
    <monochrome android:drawable="@mipmap/ic_launcher_monochrome" />
</adaptive-icon>
XML
done
png "$src" -filter Lanczos -resize 512x512 -alpha off "$brand/platforms/android/play-store-512.png"

# iOS: one opaque 1024 px icon; the system applies the mask.
ios="$brand/platforms/ios/AppIcon.appiconset"; mkdir -p "$ios"
png "$tmp/full.png" -alpha off "$ios/Icon-App-1024x1024@1x.png"
cat > "$ios/Contents.json" <<'JSON'
{
  "images" : [
    {
      "filename" : "Icon-App-1024x1024@1x.png",
      "idiom" : "universal",
      "platform" : "ios",
      "size" : "1024x1024"
    }
  ],
  "info" : {
    "author" : "xcode",
    "version" : 1
  }
}
JSON

# macOS: the 824 px body on a 1024 px canvas with a soft shadow, per Apple's icon grid.
mac="$brand/platforms/macos/AppIcon.appiconset"; mkdir -p "$mac"
rounded "$tmp/full.png" 824 "$tmp/body.png"
png -size 1024x1024 xc:none \
  \( "$tmp/body.png" -background "#0B1E3F" -shadow 32x12+0+0 \) -gravity center -geometry +0+10 -composite \
  "$tmp/body.png" -gravity center -geometry +0+0 -composite "$tmp/mac.png"
for s in 16 32 64 128 256 512 1024; do
  png "$tmp/mac.png" -filter Lanczos -resize "${s}x${s}" "$mac/app_icon_$s.png"
done
{
  echo '{'; echo '  "images" : ['
  first=1
  for e in 16:1:16 16:2:32 32:1:32 32:2:64 128:1:128 128:2:256 256:1:256 256:2:512 512:1:512 512:2:1024; do
    IFS=: read -r pt scale file <<<"$e"
    [ $first = 1 ] || echo '    },'; first=0
    printf '    {\n      "size" : "%sx%s",\n      "idiom" : "mac",\n      "filename" : "app_icon_%s.png",\n      "scale" : "%sx"\n' "$pt" "$pt" "$file" "$scale"
  done
  echo '    }'; echo '  ],'
  echo '  "info" : {'; echo '    "version" : 1,'; echo '    "author" : "xcode"'; echo '  }'; echo '}'
} > "$mac/Contents.json"

# Windows: one .ico holding every shell size.
mkdir -p "$brand/platforms/windows"
rounded "$tmp/full.png" 256 "$tmp/win.png"
magick "$tmp/win.png" -define icon:auto-resize=256,128,64,48,32,24,16 "$brand/platforms/windows/app_icon.ico"

# Linux: hicolor theme sizes.
for s in 16 22 24 32 48 64 128 256 512; do
  dir="$brand/platforms/linux/hicolor/${s}x${s}/apps"; mkdir -p "$dir"
  rounded "$tmp/full.png" "$s" "$dir/coursive.png"
done

# Web: favicons, touch icon and web-manifest icons (maskable art fills the 80% safe zone).
web="$brand/platforms/web"; mkdir -p "$web"
rounded "$tmp/full.png" 48 "$tmp/fav.png"
magick "$tmp/fav.png" -define icon:auto-resize=48,32,16 "$web/favicon.ico"
rounded "$tmp/full.png" 32 "$web/favicon-32.png"
png "$tmp/full.png" -filter Lanczos -resize 180x180 -alpha off "$web/apple-touch-icon.png"
for s in 192 512; do
  rounded "$tmp/full.png" "$s" "$web/icon-$s.png"
  extend "$tmp/full.png" "$s" $(( s * 8 / 10 )) "$tmp/mask-art.png"
  png "$tmp/mask-art.png" -alpha off "$web/icon-maskable-$s.png"
done

# Social preview (GitHub repository card, also the README banner), 1280×640.
rounded "$tmp/full.png" 336 "$tmp/card-logo.png"
png "$tmp/clean.png" -filter Lanczos -resize 1280x1280^ -gravity center -extent 1280x640 \
  \( "$tmp/card-logo.png" -background "#0B1E3F" -shadow 24x14+0+10 \) -gravity west -geometry +112+8 -composite \
  "$tmp/card-logo.png" -gravity west -geometry +128+0 -composite \
  -gravity northwest -fill "#0E2A4D" -font Inter-Display-Bold -pointsize 112 -annotate +540+168 'Coursive' \
  -fill "#2E5A8A" -font Inter-Display-Medium -pointsize 30 -annotate +546+336 'Course + Cursive' \
  -fill "#33506F" -font Inter-Display-Regular -pointsize 30 \
  -annotate +546+390 'Weave lectures, transcripts, slides, handwriting' \
  -annotate +546+436 'and AI into one knowledge canvas.' \
  "$brand/social/coursive-social-1280x640.png"

# exports.json records the master's hash and every export, so CI can tell a stale or hand-edited set.
{
  printf '{\n  "generator": "scripts/export-brand-icons.sh",\n'
  printf '  "source": {"path": "source/coursive-logo-4096.png", "sha256": "%s"},\n' "$(sha256sum "$src" | cut -d' ' -f1)"
  printf '  "files": [\n'
  (cd "$brand" && find logo platforms social -type f | LC_ALL=C sort) | while read -r rel; do
    printf '    {"path": "%s", "sha256": "%s"}\n' "$rel" "$(sha256sum "$brand/$rel" | cut -d' ' -f1)"
  done | sed '$!s/$/,/'
  printf '  ]\n}\n'
} > "$brand/exports.json"

echo "export-brand-icons: wrote $(find "$brand" -type f ! -path '*/source/*' ! -name exports.json | wc -l) files under assets/brand/"
