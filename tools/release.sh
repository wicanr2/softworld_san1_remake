#!/usr/bin/env bash
# 打包發行版：Linux／Windows／macOS。
#
#   tools/release.sh [版本字串]
#
# 產物在 workplace/release/（gitignore）。**不含任何原版檔案**——
# 玩家要自備原版目錄，用 -root 指過去。
#
# 三個平台的交叉編譯條件不一樣：
#
#   Windows  CGO_ENABLED=0 就過。Ebiten 在 Windows 走 syscall 不走 cgo。
#   Linux    要 cgo（X11／GL），所以用本機 amd64 的 Go 容器直接建。
#   macOS    要 cgo（Objective-C），用 osxcross 的容器；沒有那個 image
#            就跳過並說明，不假裝打包成功。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VER="${1:-$(cd "$ROOT" && git describe --tags --always --dirty 2>/dev/null || echo dev)}"
OUT="$ROOT/workplace/release"
MAC_IMAGE="${SAN1_MAC_IMAGE:-eob-remake-macos:1.26.7-ebiten2.9.9-audio}"

# SOURCE_DATE 是包裡所有檔案的時間戳。**固定值不是「現在」**：
# 時間戳進了 tar 與 zip 的檔頭，用現在的話同一份內容每次雜湊都不同。
# 可以用 SOURCE_DATE_EPOCH 覆蓋（reproducible-builds.org 的慣例）。
SOURCE_DATE="@${SOURCE_DATE_EPOCH:-0}"

# **舊版本的包要先清掉。** 檔名帶版本號，所以舊包不會被覆蓋而是留在旁邊，
# 而最後的 sha256sum 掃的是整個目錄——校驗碼清單於是會混進上一次建的東西，
# 而且看起來完全正常（每一行的雜湊都是對的，只是那個檔不屬於這一版）。
rm -rf "$OUT"
mkdir -p "$OUT"
echo "版本 $VER"

# stage 把一個平台的產物擺成可以壓縮的樣子。
stage() {
  local name="$1" bin="$2" as="${3:-}"
  local d="$OUT/stage/$name"
  mkdir -p "$d"
  # 包裡的執行檔一律叫 san1（Windows 是 san1.exe）：平台後綴是給
  # 建置目錄用的，玩家打的指令不該因為平台不同而不一樣。
  cp "$bin" "$d/${as:-$(basename "$bin")}"
  cp "$ROOT/LICENSE" "$d/"
  cp "$ROOT/README.md" "$d/"
  mkdir -p "$d/fonts"
  # 四套字型與**它們各自的授權**一起帶：unifont 是 GPL v2 ＋ 字型例外，
  # 授權文字要跟著字型走（先前只帶了字型本身）。ascii6x10 是英文在原版
  # 版面放不下時的小字級（docs/spec/014 §3.2）。kai／li 是主選單那兩項
  # 切換的楷書與隸書（Issue #71），**GPL v2、沒有字型例外**，所以
  # `LICENSE-wangfonts.txt` 一定要在包裡——少了它就是散布違反條款。
  cp "$ROOT/fonts/unifont.hex.gz" "$ROOT/fonts/ascii6x10.hex.gz" \
     "$ROOT/fonts/kai.hex.gz" "$ROOT/fonts/li.hex.gz" \
     "$ROOT/fonts/LICENSE-unifont.txt" "$ROOT/fonts/LICENSE-x11-misc-fixed.txt" \
     "$ROOT/fonts/LICENSE-wangfonts.txt" "$d/fonts/"
  cat > "$d/如何開始.txt" <<'TXT'
三國演義 remake

這個包**不含原版遊戲檔案**。要玩得先有你自己那一份原版，
再用 -root 指到它的目錄：

    san1 -root /path/to/三國演義

其他常用參數：

    -lang zh-Hant|en|ja   介面語言
    -slot 001..006        劇本
    -scale 2              視窗放大倍率
    -music=false          關掉配樂

授權見 LICENSE。
TXT
}

echo "== Windows（CGO_ENABLED=0）"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  "$ROOT/tools/go.sh" build -trimpath -ldflags "-s -w" \
  -o /src/workplace/release/san1.exe ./cmd/san1
stage "san1-$VER-windows-amd64" "$OUT/san1.exe"
rm -f "$OUT/san1.exe"

echo "== Linux（cgo）"
"$ROOT/tools/go.sh" build -trimpath -ldflags "-s -w" \
  -o /src/workplace/release/san1 ./cmd/san1
stage "san1-$VER-linux-amd64" "$OUT/san1"
rm -f "$OUT/san1"

echo "== macOS（osxcross）"
if docker image inspect "$MAC_IMAGE" >/dev/null 2>&1; then
  for pair in "amd64 o64-clang darwin-amd64" "arm64 oa64-clang darwin-arm64"; do
    set -- $pair
    arch="$1" cc="$2" tag="$3"
    # macOS 這一段失敗不該拖垮另外兩個平台：那兩個已經建好了，
    # 中途 set -e 掉出去的話連壓縮都不會跑，看起來像整個發行流程壞了。
    if ! docker run --rm --network none \
      --memory 4g --cpus 2 --pids-limit 256 \
      --log-opt max-size=10m --log-opt max-file=3 \
      -u "$(id -u):$(id -g)" \
      -v "$ROOT:/src" \
      -v "$ROOT/workplace/gocache:/gocache" \
      -v "$ROOT/workplace/gomodcache:/gomodcache" \
      -e GOCACHE=/gocache -e GOMODCACHE=/gomodcache \
      -e GOPROXY=file:///gomodcache/cache/download -e GOSUMDB=off \
      -e GOWORK=off \
      -e HOME=/tmp -w /src "$MAC_IMAGE" \
      env GOOS=darwin GOARCH="$arch" CGO_ENABLED=1 CC="$cc" \
      go build -trimpath -o "/src/workplace/release/san1-$tag" ./cmd/san1
    then
      echo "  ⚠ $tag 建置失敗，跳過（另外兩個平台不受影響）"
      continue
    fi
    stage "san1-$VER-$tag" "$OUT/san1-$tag" san1
    rm -f "$OUT/san1-$tag"
  done
else
  echo "  跳過：沒有 $MAC_IMAGE。要 macOS 版得先備好 osxcross 的 image"
  echo "  （skill osxcross-macos-cross-build）。**這不是打包成功**。"
fi

# ── 簽章 ───────────────────────────────────────────────────────
#
# **憑證不進這個 repo，也不進容器。** 三個平台各自要的東西：
#
#   Windows  SAN1_WIN_PFX ＝ .pfx 檔、SAN1_WIN_PFX_PASS ＝ 它的密碼
#            用 osslsigncode 簽（Linux 上簽 PE 的標準做法）
#   macOS    SAN1_MAC_IDENTITY ＝ "Developer ID Application: …"
#            **要在真的 macOS 上跑**：codesign 與 notarytool 都不能
#            交叉執行，osxcross 只負責編譯。這裡只產出待簽清單。
#   Linux    沒有平台級的簽章慣例；發行用 SHA256SUMS ＋ GPG 分離簽章
#            （SAN1_GPG_KEY ＝ 金鑰 ID）
#
# **沒有設就跳過並說明，不假裝簽過**——與 macOS 那一段同一個原則。
sign_windows() {
  local exe="$1"
  if [[ -z "${SAN1_WIN_PFX:-}" ]]; then
    echo "  跳過 Windows 簽章：沒設 SAN1_WIN_PFX。**這不是簽過**"
    return 0
  fi
  if ! command -v osslsigncode >/dev/null 2>&1; then
    echo "  ⚠ 沒有 osslsigncode，Windows 簽章跳過"
    return 0
  fi
  osslsigncode sign -pkcs12 "$SAN1_WIN_PFX" \
    -pass "${SAN1_WIN_PFX_PASS:-}" \
    -n "三國演義 remake" -i "https://github.com/wicanr2/softworld_san1_remake" \
    -t http://timestamp.digicert.com \
    -in "$exe" -out "$exe.signed" && mv "$exe.signed" "$exe"
  echo "  Windows 執行檔已簽章"
}

# ── 冒煙測試 ───────────────────────────────────────────────────
#
# **建得出來不等於跑得起來。** Ebiten 在 package init 就開 GLFW，
# 少一個共享函式庫、字型路徑寫錯、資產讀法改過——這些都不會讓
# `go build` 失敗，只會讓玩家一按下去就閃退。
#
# 所以這裡真的把包解開來跑一次：先 `-h`（確認起得來），再帶原版素材
# 跑 25 秒（確認載得進去、不會中途崩）。跑滿 25 秒被 timeout 砍掉
# （退出碼 124）才是通過——**自己結束反而是壞消息**。
#
# 只驗 Linux：另外三個平台在這台機器上執行不了，硬要驗會變成假綠。
smoke_linux() {
  local d="$1"
  if [[ -z "${SAN1_ORIG:-$ROOT/org_game}" ]] || [[ ! -d "${SAN1_ORIG:-$ROOT/org_game}" ]]; then
    echo "  跳過冒煙測試：沒有原版素材"
    return 0
  fi
  local orig="${SAN1_ORIG:-$ROOT/org_game}"
  docker run --rm --network none --memory 2g --cpus 2 --pids-limit 128 \
    --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" \
    -v "$d:/pkg:ro" -v "$orig:/orig:ro" -e HOME=/tmp -w /pkg \
    "${SAN1_GO_IMAGE:-rich2-go-ebiten:latest}" bash -c '
      command -v xvfb-run >/dev/null || { echo "  跳過：image 裡沒有 xvfb-run"; exit 0; }
      xvfb-run -a ./san1 -h >/dev/null 2>&1 || true
      # **不要 set -e**：timeout 砍掉的那個 124 才是我們要的結果，
      # set -e 會在讀到 $? 之前就把腳本結束掉。
      rc=0
      xvfb-run -a timeout -s INT 25 ./san1 -root "/orig/三國演義" -music=false \
        >/tmp/smoke.log 2>&1 || rc=$?
      if [[ $rc -ne 124 ]]; then
        echo "  ✗ 冒煙測試：跑了不到 25 秒就結束（退出碼 $rc）"
        tail -20 /tmp/smoke.log
        exit 1
      fi
      echo "  ✓ 冒煙測試：帶原版素材跑滿 25 秒沒有崩"
    '
}

echo
echo "== 冒煙測試（Linux）"
for d in "$OUT"/stage/*linux*/; do
  [[ -d "$d" ]] || continue
  smoke_linux "$d"
done

echo
echo "== 簽章"
for d in "$OUT"/stage/*windows*/; do
  [[ -d "$d" ]] || continue
  sign_windows "$d/san1.exe"
done
if [[ -n "${SAN1_MAC_IDENTITY:-}" ]]; then
  echo "  ⚠ macOS 的 codesign／notarytool 不能在 Linux 上跑。"
  echo "    待簽的執行檔：$(ls -d "$OUT"/stage/*darwin*/ 2>/dev/null | tr '\n' ' ')"
  echo "    在 macOS 上跑 docs/release/01 §簽章 那一節的三道指令。"
else
  echo "  跳過 macOS 簽章：沒設 SAN1_MAC_IDENTITY。**這不是簽過**"
fi

echo
echo "== 壓縮"
# **包要逐位元組可重現。** 沒有憑證的時候，`SHA256SUMS` 是唯一能讓別人
# 獨立驗證「這個包確實是這份原始碼建出來的」的東西——而它只有在同一份
# 原始碼每次都壓出同一個位元組串時才有意義。
#
# 預設的 tar／gzip／zip 會把**修改時間、擁有者、檔案順序**寫進去，
# 於是同一份內容每次的雜湊都不一樣，看起來完全正常。
#
#   tar   --sort=name 固定順序、--mtime 固定時間、--owner/--group/--numeric-owner
#         固定擁有者；gzip -n 不寫檔名與時戳
#   zip   -X 不寫額外屬性；檔案的 mtime 要先自己統一（zip 沒有 --mtime）
#
# Go 那一邊 `-trimpath` 已經拿掉了建置路徑。
export TZ=UTC
find "$OUT/stage" -exec touch -h -d "$SOURCE_DATE" {} +
cd "$OUT/stage"
for d in */; do
  d="${d%/}"
  case "$d" in
    *windows*)
      (cd "$OUT/stage" && find "$d" -print | LC_ALL=C sort | \
        zip -qX -@ "$OUT/$d.zip")
      echo "  $d.zip" ;;
    *)
      tar --sort=name --mtime="$SOURCE_DATE" \
          --owner=0 --group=0 --numeric-owner \
          -cf - "$d" | gzip -n -9 > "$OUT/$d.tar.gz"
      echo "  $d.tar.gz" ;;
  esac
done
cd "$ROOT"
rm -rf "$OUT/stage"

echo
echo "校驗碼："
(cd "$OUT" && sha256sum ./*.zip ./*.tar.gz 2>/dev/null | tee SHA256SUMS)
if [[ -n "${SAN1_GPG_KEY:-}" ]] && command -v gpg >/dev/null 2>&1; then
  (cd "$OUT" && gpg --batch --yes --local-user "$SAN1_GPG_KEY" \
    --detach-sign --armor SHA256SUMS) && echo "  SHA256SUMS.asc 已簽"
else
  echo "  跳過 SHA256SUMS 的 GPG 簽章：沒設 SAN1_GPG_KEY。**這不是簽過**"
fi
echo
echo "⚠ 這些包不含原版檔案；玩家要自備原版目錄。"
