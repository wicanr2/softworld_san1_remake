#!/usr/bin/env bash
# 高清素材的正常玩家路徑，各批收據分開保存。
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ORIG="${SAN1_ORIG:-$ROOT/org_game}"
case "${1:-}" in
  '') target=workplace/hd-window/player ;;
  --scene-plus) target=workplace/hd-window/player/scene-plus ;;
  --custom) target=workplace/hd-window/player/custom-v1 ;;
  --portraits) target=workplace/hd-window/player/portraits-v2 ;;
  --lords) target=workplace/hd-window/player/lords-v3 ;;
  --lords-all) target=workplace/hd-window/player/lords-v4 ;;
  --commanders) target=workplace/hd-window/player/commanders-v6 ;;
  --commanders-next) target=workplace/hd-window/player/commanders-v10 ;;
  --officers) target=workplace/hd-window/player/commanders-v11 ;;
  --officers-all) target=workplace/hd-window/player/commanders-v12 ;;
  --officers-002) target=workplace/hd-window/player/commanders-v13 ;;
  --officers-003) target=workplace/hd-window/player/commanders-v14 ;;
  *) echo "用法：bash tools/verify-hd-player.sh [--scene-plus|--custom|--portraits|--lords|--lords-all|--commanders|--commanders-next|--officers|--officers-all|--officers-002|--officers-003]" >&2; exit 2 ;;
esac
[[ $# -le 1 ]] || exit 2
pack=workplace/hd-assets
case "${1:-}" in
  --custom) pack=workplace/hd-assets-custom-v1 ;;
  --portraits) pack=workplace/hd-assets-portraits-v2 ;;
  --lords) pack=workplace/hd-assets-portraits-v3 ;;
  --lords-all) pack=workplace/hd-assets-portraits-v4 ;;
  --commanders) pack=workplace/hd-assets-portraits-v6 ;;
  --commanders-next) pack=workplace/hd-assets-portraits-v10 ;;
  --officers) pack=workplace/hd-assets-portraits-v11 ;;
  --officers-all) pack=workplace/hd-assets-portraits-v12 ;;
  --officers-002) pack=workplace/hd-assets-portraits-v13 ;;
  --officers-003) pack=workplace/hd-assets-portraits-v14 ;;
esac
for dir in "$ROOT" "$ORIG" "$ROOT/workplace/hd-window" "$ROOT/workplace/hd-assets" \
  "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  test -d "$dir" || { echo "缺少目錄：$dir" >&2; exit 2; }
done
if [[ "${1:-}" == --custom ]]; then
  test -d "$ROOT/workplace/hd-assets-custom-v1" || { echo '缺少本機自創君主素材包' >&2; exit 2; }
fi
if [[ "${1:-}" == --portraits ]]; then
  test -d "$ROOT/workplace/hd-assets-portraits-v2" || { echo '缺少本機肖像素材包 v2' >&2; exit 2; }
fi
if [[ "${1:-}" == --lords ]]; then
  test -d "$ROOT/workplace/hd-assets-portraits-v3" || { echo '缺少本機肖像素材包 v3' >&2; exit 2; }
fi
if [[ "${1:-}" == --lords-all ]]; then
  test -d "$ROOT/workplace/hd-assets-portraits-v4" || { echo '缺少本機肖像素材包 v4' >&2; exit 2; }
fi
if [[ "${1:-}" == --commanders ]]; then
  test -d "$ROOT/workplace/hd-assets-portraits-v6" || { echo '缺少本機肖像素材包 v6' >&2; exit 2; }
fi
if [[ "${1:-}" == --commanders-next || "${1:-}" == --officers || "${1:-}" == --officers-all || "${1:-}" == --officers-002 || "${1:-}" == --officers-003 ]]; then
  test -d "$ROOT/$pack" || { echo '缺少本機肖像素材包' >&2; exit 2; }
  for edition in base plus; do
    test -d "$ROOT/workplace/hd-inventory/$edition/img/DATA3" || { echo '缺少肖像來源參考目錄' >&2; exit 2; }
  done
fi
common=(--rm --network none --memory 3g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3 -u "$(id -u):$(id -g)"
  -v "$ROOT:/src" -v "$ORIG:/src/org_game:ro" -v "$ORIG:/orig:ro"
  -v "$ROOT/$pack:/src/$pack:ro" -w /src)
if [[ "${1:-}" == --commanders-next || "${1:-}" == --officers || "${1:-}" == --officers-all || "${1:-}" == --officers-002 || "${1:-}" == --officers-003 ]]; then
  common+=(-v "$ROOT/workplace/hd-inventory:/src/workplace/hd-inventory:ro")
fi
timeout 3m docker run "${common[@]}" --name san1-hd-player-build \
  -e SAN1_HD_PLAYER_OUT="$target" \
  -e GOCACHE=/src/workplace/gocache -e GOMODCACHE=/src/workplace/gomodcache \
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download -e GOSUMDB=off -e GOWORK=off \
  --entrypoint /bin/bash rich2-go-ebiten:latest -c '
    set -euo pipefail
    target="$SAN1_HD_PLAYER_OUT"
    for path in workplace/hd-window "$target"; do
      if [[ -e "$path" && "$(stat -c %u:%g "$path")" != "$(id -u):$(id -g)" ]]; then
        echo "輸出擁有權不符：$path" >&2; exit 2
      fi
    done
    mkdir -p "$target"
    go build -trimpath -o "$target/san1-window-check" ./cmd/san1
    go run ./cmd/san1dump -root /orig/三國演義 -screen title -png "$target/title-reference.png" >/dev/null
    python3 -m py_compile tools/verify-window-inner.py tools/verify-hd-player-inner.py
    cp tools/verify-hd-player.sh tools/verify-hd-player-inner.py tools/verify-window-inner.py "$target/"
  '
gui_limit=12m
# 四十張卡、十次新局及逐次選單畫面同步，仍設外層上限。
if [[ "${1:-}" == --lords-all ]]; then gui_limit=28m; fi
# 二十張卡、八次新局；十二分鐘批次在最後收尾時逾時。
if [[ "${1:-}" == --commanders ]]; then gui_limit=18m; fi
# 四十張卡、兩次新局，逐張額外核對原版肖像。
if [[ "${1:-}" == --commanders-next || "${1:-}" == --officers ]]; then gui_limit=28m; fi
# 九十張卡、兩次新局，逐張核對來源、高清與恢復。
if [[ "${1:-}" == --officers-all ]]; then gui_limit=60m; fi
# 劇本 002 的九十二張卡、兩次新局。
if [[ "${1:-}" == --officers-002 ]]; then gui_limit=60m; fi
# 劇本 003 的六十八張卡、兩次新局。
if [[ "${1:-}" == --officers-003 ]]; then gui_limit=60m; fi
timeout "$gui_limit" docker run "${common[@]}" --name san1-hd-player-gui \
  --entrypoint python3 eob-audio-capture:20260922-r2 tools/verify-hd-player-inner.py "$@"
