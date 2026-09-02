#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_icon="$root/site/assets/appicon-mark.svg"

command -v magick >/dev/null || { echo "ImageMagick is required" >&2; exit 1; }
magick -background none -depth 8 "$source_icon" -resize 64x64 -strip "$root/site/assets/favicon.png"
magick -background none -depth 8 "$source_icon" -resize 180x180 -strip "$root/site/assets/apple-touch-icon.png"
magick -background none -depth 8 "$source_icon" -resize 192x192 -strip "$root/site/assets/icon-192.png"
magick -background none -depth 8 "$source_icon" -resize 512x512 -strip "$root/site/assets/icon-512.png"
