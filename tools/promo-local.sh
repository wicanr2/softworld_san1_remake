#!/usr/bin/env bash
# 主機只啟動有界容器；正式封包錄影及原版配樂影片僅留本機。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
[[ $# == 1 && "$1" =~ ^v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}$ ]] || { echo '用法：SAN1_PROMO_AUDIO=workplace/<原版錄音> tools/promo-local.sh <完整版號>' >&2; exit 2; }
VER="$1"
AUDIO="${SAN1_PROMO_AUDIO:?需指定已驗證的 DOSBox-X 原版錄音目錄}"
FONT="${SAN1_PROMO_FONT:-/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc}"
ASSEMBLE_IMAGE="san1-promo:ffmpeg5.1.9-r1"
[[ "$AUDIO" == workplace/* && "$AUDIO" != *'..'* ]] || exit 2
for dir in "$ROOT" "$ROOT/org_game" "$ROOT/dist-all/$VER" "$ROOT/workplace" "$ROOT/$AUDIO" "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$dir" || { echo "缺少目錄：$dir" >&2; exit 2; }
done
test -f "$ROOT/$AUDIO/receipt.json" || exit 2
test -f "$FONT" || { echo "缺少字幕字型：$FONT" >&2; exit 2; }
docker image inspect "$ASSEMBLE_IMAGE" >/dev/null
test -f "$ROOT/dist-all/$VER/full-local/san1-$VER-linux-amd64.tar.gz" || exit 2
target="workplace/promo-source/$VER"
if [[ "${SAN1_PROMO_ASSEMBLE_ONLY:-0}" == 1 ]]; then
  test -f "$ROOT/$target/receipt.json" || exit 2
else
  test ! -e "$ROOT/$target" || { echo "拒絕覆寫錄影來源：$target" >&2; exit 2; }
fi
common=(--rm --network none --memory 4g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)"
  --mount "type=bind,src=$ROOT,dst=/src"
  --mount "type=bind,src=$ROOT/org_game,dst=/orig,readonly"
  --mount "type=bind,src=$ROOT/org_game,dst=/src/org_game,readonly"
  --mount "type=bind,src=$ROOT/dist-all/$VER,dst=/src/dist-all/$VER,readonly"
  -w /src -e HOME=/tmp -e GOWORK=off -e GOMAXPROCS=2
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off)
if [[ "${SAN1_PROMO_ASSEMBLE_ONLY:-0}" != 1 ]]; then
timeout 4m docker run "${common[@]}" --name san1-promo-reference \
  -e "SAN1_PROMO_OUT=$target" --entrypoint bash eob-remake-release:1.26.7-ebiten2.9.9-audio -c '
    set -euo pipefail
    test "$(stat -c %u:%g workplace)" = "$(id -u):$(id -g)"
    mkdir -p workplace/promo-source
    test "$(stat -c %u:%g workplace/promo-source)" = "$(id -u):$(id -g)"
    mkdir "$SAN1_PROMO_OUT"
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$SAN1_PROMO_OUT/title-reference.png" >/dev/null
    go run tools/hd-battle-branches-reference.go -out "$SAN1_PROMO_OUT" >/dev/null
  '
timeout 12m docker run "${common[@]}" --cpus 8 --name san1-promo-capture \
  -e "SAN1_PROMO_OUT=$target" -e "SAN1_PROMO_VERSION=$VER" -e LP_NUM_THREADS=2 \
  --entrypoint bash eob-audio-capture:20260922-r2 -c '
    set -euo pipefail
    scratch=$(mktemp -d /tmp/san1-promo.XXXXXX)
    trap '\''rm -rf -- "$scratch"'\'' EXIT
    tar -xzf "dist-all/$SAN1_PROMO_VERSION/full-local/san1-$SAN1_PROMO_VERSION-linux-amd64.tar.gz" -C "$scratch"
    python3 tools/promo-capture-inner.py --out "/src/$SAN1_PROMO_OUT" --package "$scratch/san1-$SAN1_PROMO_VERSION-linux-amd64"
  '
fi
timeout 10m docker run --rm --pull never --name san1-promo-assemble --network none --memory 2g --cpus 2 --pids-limit 128 \
  --log-opt max-size=10m --log-opt max-file=3 --user "$(id -u):$(id -g)" \
  --mount "type=bind,src=$ROOT,dst=/src" \
  --mount "type=bind,src=$ROOT/org_game,dst=/src/org_game,readonly" \
  --mount "type=bind,src=$ROOT/$AUDIO,dst=/audio,readonly" \
  --mount "type=bind,src=$FONT,dst=/promo-font.ttc,readonly" \
  -e SAN1_PROMO_FONT=/promo-font.ttc \
  -w /src --entrypoint bash "$ASSEMBLE_IMAGE" tools/promo-local-inner.sh "$VER"
