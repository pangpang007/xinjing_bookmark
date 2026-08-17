#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$ROOT/assets/fonts"
OUT="$DIR/SourceHanSansCN-Regular.otf"

mkdir -p "$DIR"
if [[ -f "$OUT" ]]; then
  echo "font already exists: $OUT"
  exit 0
fi

URLS=(
  "https://cdn.jsdelivr.net/gh/notofonts/noto-cjk@Sans2.004/Sans/SubsetOTF/SC/NotoSansSC-Regular.otf"
  "https://github.com/notofonts/noto-cjk/raw/Sans2.004/Sans/SubsetOTF/SC/NotoSansSC-Regular.otf"
)

for url in "${URLS[@]}"; do
  echo "downloading $url"
  if curl -L --fail --retry 3 -o "$OUT" "$url"; then
    echo "saved $OUT"
    exit 0
  fi
  rm -f "$OUT"
done

echo "failed to download Source Han Sans / Noto Sans SC" >&2
exit 1
