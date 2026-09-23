#!/usr/bin/env bash
# 主機只負責啟動隔離容器；建置、封包、驗證與寫檔都在容器內。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ $# -ne 1 || ! "$1" =~ ^v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}$ ]]; then
  echo '用法：tools/release.sh v.<主版>.<次版>.<修訂版>-YYYYMMDD' >&2
  exit 2
fi
VER="$1"
GO_IMAGE="eob-remake-release:1.26.7-ebiten2.9.9-audio"
MAC_IMAGE="eob-remake-macos:1.26.7-ebiten2.9.9-audio"

# 版號與內容必須可回查同一個 commit；AGENTS.md 等未加入版控的
# 本機指示不進封包，追蹤檔案有差異則不能建立正式交付。
git -C "$ROOT" diff --quiet
git -C "$ROOT" diff --cached --quiet
while IFS= read -r untracked; do
  [[ -z "$untracked" || "$untracked" == AGENTS.md ]] || {
    echo "有未追蹤的建置輸入，無法回查來源 commit：$untracked" >&2
    exit 1
  }
done < <(git -C "$ROOT" ls-files --others --exclude-standard)
COMMIT="$(git -C "$ROOT" rev-parse HEAD)"

# 每個 -v 的主機來源先確認存在且形態正確，防止 dockerd 以 root
# 建立同名空目錄。原版素材在兩個容器中都另以唯讀掛載蓋住工作樹。
for d in "$ROOT" "$ROOT/org_game" "$ROOT/workplace/gocache" "$ROOT/workplace/gomodcache"; do
  [[ -d "$d" ]] || { echo "缺少掛載目錄：$d" >&2; exit 1; }
done
for f in "$ROOT/LICENSE" "$ROOT/README.md" "$ROOT/tools/release-inner.sh"; do
  [[ -f "$f" ]] || { echo "缺少輸入檔：$f" >&2; exit 1; }
done
docker image inspect "$GO_IMAGE" "$MAC_IMAGE" >/dev/null
GO_ID="$(docker image inspect "$GO_IMAGE" --format '{{.Id}}')"
MAC_ID="$(docker image inspect "$MAC_IMAGE" --format '{{.Id}}')"

common=(--rm --network none --memory 4g --cpus 2 --pids-limit 256
  --log-opt max-size=10m --log-opt max-file=3
  -u "$(id -u):$(id -g)"
  -v "$ROOT:/src"
  -v "$ROOT/org_game:/src/org_game:ro"
  -v "$ROOT/org_game:/orig:ro"
  -e HOME=/tmp -e GOWORK=off
  -e GOCACHE=/src/workplace/gocache
  -e GOMODCACHE=/src/workplace/gomodcache
  -e GOPROXY=file:///src/workplace/gomodcache/cache/download
  -e GOSUMDB=off
  -e "SAN1_SOURCE_COMMIT=$COMMIT"
  -e "SAN1_GO_IMAGE_ID=$GO_ID"
  -e "SAN1_MAC_IMAGE_ID=$MAC_ID"
  -e "SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH:-315532800}"
  -w /src)

# 中間產物可重建，正式 dist-all 目錄只在全部驗收後建立。
timeout 15m docker run "${common[@]}" "$GO_IMAGE" bash tools/release-inner.sh build "$VER"
for pair in 'amd64 o64-clang' 'arm64 oa64-clang'; do
  read -r arch cc <<<"$pair"
  timeout 15m docker run "${common[@]}" \
    -e GOOS=darwin -e "GOARCH=$arch" -e CGO_ENABLED=1 -e "CC=$cc" \
    "$MAC_IMAGE" bash tools/release-inner.sh build-mac "$VER" "$arch"
done
timeout 10m docker run "${common[@]}" "$GO_IMAGE" bash tools/release-inner.sh finalize "$VER"
echo "本機交付：$ROOT/dist-all/$VER"
