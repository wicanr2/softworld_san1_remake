#!/usr/bin/env bash
# 僅供 tools/release.sh 在有界、無網路的 Docker 容器內呼叫。
set -euo pipefail

phase="${1:-}"
ver="${2:-}"
if [[ ! "$ver" =~ ^v\.[0-9]+\.[0-9]+\.[0-9]+-[0-9]{8}$ ]]; then
  echo "不合規的完整版號：$ver" >&2
  exit 2
fi
stage="/src/workplace/release-build/$ver"
final="/src/dist-all/$ver"
ldflags="-s -w -X main.releaseVersion=$ver"
export TZ=UTC

check_owner() {
  local path="$1"
  [[ ! -e "$path" ]] || [[ "$(stat -c '%u:%g' "$path")" == "$(id -u):$(id -g)" ]] || {
    echo "輸出路徑擁有權不符：$path" >&2
    exit 1
  }
}

case "$phase" in
  build)
    [[ ! -e "$final" ]] || { echo "現行交付已存在，不覆寫：$final" >&2; exit 1; }
    for output in /src/workplace /src/workplace/gocache /src/workplace/gomodcache \
      /src/workplace/release-build "$stage" /src/dist-all; do
      check_owner "$output"
    done
    rm -rf -- "$stage"
    mkdir -p "$stage/bin"
    GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -trimpath -ldflags "$ldflags" \
      -o "$stage/bin/san1-linux-amd64" ./cmd/san1
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" \
      -o "$stage/bin/san1-windows-amd64.exe" ./cmd/san1
    ;;
  build-mac)
    arch="${3:-}"
    [[ "$arch" == amd64 || "$arch" == arm64 ]] || { echo "不支援架構：$arch" >&2; exit 2; }
    [[ -d "$stage/bin" && ! -e "$final" ]] || { echo 'Linux／Windows 階段未完成或交付已存在' >&2; exit 1; }
    [[ "${GOOS:-}" == darwin && "${GOARCH:-}" == "$arch" && "${CGO_ENABLED:-}" == 1 ]] || {
      echo 'macOS 交叉編譯參數不完整' >&2; exit 1;
    }
    go build -trimpath -ldflags "$ldflags" \
      -o "$stage/bin/san1-darwin-$arch" ./cmd/san1
    ;;
  finalize)
    [[ ! -e "$final" ]] || { echo "現行交付已存在，不覆寫：$final" >&2; exit 1; }
    for bin in san1-linux-amd64 san1-windows-amd64.exe san1-darwin-amd64 san1-darwin-arm64; do
      [[ -f "$stage/bin/$bin" ]] || { echo "缺少平台執行檔：$bin" >&2; exit 1; }
      grep -aFq -- "$ver" "$stage/bin/$bin" || { echo "執行檔沒有完整版號：$bin" >&2; exit 1; }
    done
    rm -rf -- "$stage/final" "$stage/packages"
    mkdir -p "$stage/final/patch" "$stage/final/smoke" "$stage/packages"

    package() {
      local platform="$1" bin="$2" output="$3"
      local dir="$stage/packages/san1-$ver-$platform"
      mkdir -p "$dir/fonts"
      if [[ "$platform" == windows-* ]]; then
        cp "$stage/bin/$bin" "$dir/san1.exe"
      else
        cp "$stage/bin/$bin" "$dir/san1"
      fi
      cp /src/LICENSE /src/README.md "$dir/"
      cp /src/fonts/unifont.hex.gz /src/fonts/ascii6x10.hex.gz \
         /src/fonts/kai.hex.gz /src/fonts/li.hex.gz \
         /src/fonts/LICENSE-unifont.txt /src/fonts/LICENSE-x11-misc-fixed.txt \
         /src/fonts/LICENSE-wangfonts.txt "$dir/fonts/"
      cat > "$dir/如何開始.txt" <<EOF
三國演義 remake $ver

本包僅含引擎與另備字型，不含原版執行檔、資料、美術、音樂或語音。
請自備合法持有的原版，將 -root 指向其目錄：

    san1 -root /path/to/三國演義

常用參數：-edition base|plus、-lang zh-Hant|en|ja、-slot 001..006。
預設存檔位置是執行時工作目錄下的 saves/；可用 -saves 指定其他目錄。
存檔不會寫回 -root 指定的原版目錄。
執行 san1 -version 可核對完整版號；授權見 LICENSE 與 fonts/LICENSE-*.txt。
EOF
      if [[ "$platform" == windows-* ]]; then
        chmod 755 "$dir/san1.exe"
      else
        chmod 755 "$dir/san1"
      fi
      # ZIP 的檔案時間不得早於 1980；來源時間由主機 wrapper 固定傳入。
      find "$dir" -exec touch -h -d "@${SOURCE_DATE_EPOCH:-315532800}" {} +
      if [[ "$platform" == windows-* ]]; then
        (cd "$stage/packages" && find "${dir##*/}" -print | LC_ALL=C sort | \
          zip -qX -@ "$stage/final/patch/$output")
      else
        tar -C "$stage/packages" --sort=name \
          --mtime="@${SOURCE_DATE_EPOCH:-315532800}" \
          --owner=0 --group=0 --numeric-owner \
          -cf - "${dir##*/}" | gzip -n -9 > "$stage/final/patch/$output"
      fi
    }

    package linux-amd64 san1-linux-amd64 "san1-$ver-linux-amd64.tar.gz"
    package windows-amd64 san1-windows-amd64.exe "san1-$ver-windows-amd64.zip"
    package darwin-amd64 san1-darwin-amd64 "san1-$ver-darwin-amd64.tar.gz"
    package darwin-arm64 san1-darwin-arm64 "san1-$ver-darwin-arm64.tar.gz"

    # 從壓縮檔重讀目錄及 CRC，並與允許公開的固定檔案清單逐項比較。
    for platform in linux-amd64 windows-amd64 darwin-amd64 darwin-arm64; do
      name="san1-$ver-$platform"
      bin_name=san1
      [[ "$platform" != windows-* ]] || bin_name=san1.exe
      expected="$stage/expected-$platform.txt"
      contents="$stage/final/smoke/archive-contents-$platform.txt"
      printf '%s\n' \
        "$name" "$name/$bin_name" "$name/LICENSE" "$name/README.md" \
        "$name/如何開始.txt" "$name/fonts" \
        "$name/fonts/unifont.hex.gz" "$name/fonts/ascii6x10.hex.gz" \
        "$name/fonts/kai.hex.gz" "$name/fonts/li.hex.gz" \
        "$name/fonts/LICENSE-unifont.txt" \
        "$name/fonts/LICENSE-x11-misc-fixed.txt" \
        "$name/fonts/LICENSE-wangfonts.txt" | LC_ALL=C sort > "$expected"
      if [[ "$platform" == windows-* ]]; then
        archive="$stage/final/patch/$name.zip"
        unzip -tq "$archive" >/dev/null
        unzip -Z1 "$archive" | sed 's#/$##' | LC_ALL=C sort > "$contents"
      else
        archive="$stage/final/patch/$name.tar.gz"
        gzip -t "$archive"
        tar --quoting-style=literal -tzf "$archive" | sed 's#/$##' | LC_ALL=C sort > "$contents"
      fi
      diff -u "$expected" "$contents"
    done

    file "$stage/bin"/* > "$stage/final/smoke/binary-types.txt"
    grep -q 'ELF 64-bit.*x86-64' "$stage/final/smoke/binary-types.txt"
    grep -q 'PE32+.*x86-64' "$stage/final/smoke/binary-types.txt"
    grep -q 'Mach-O 64-bit x86_64' "$stage/final/smoke/binary-types.txt"
    grep -q 'Mach-O 64-bit arm64' "$stage/final/smoke/binary-types.txt"

    linux_dir="$stage/packages/san1-$ver-linux-amd64"
    (cd "$linux_dir" && xvfb-run -a ./san1 -version) > "$stage/final/smoke/linux-version.txt"
    [[ "$(cat "$stage/final/smoke/linux-version.txt")" == "$ver" ]] || {
      echo 'Linux 程式版本不符' >&2; exit 1;
    }
    for edition in base plus; do
      root=/orig/三國演義
      [[ "$edition" != plus ]] || root=/orig/三國演義1加強版
      [[ -d "$root" ]] || { echo "原版素材目錄不存在：$root" >&2; exit 1; }
      rc=0
      (cd "$linux_dir" && timeout -s INT 25 xvfb-run -a ./san1 \
        -edition "$edition" -root "$root" \
        -saves "$stage/smoke-saves/$edition" -music=false) \
        > "$stage/final/smoke/linux-$edition-play.log" 2>&1 || rc=$?
      [[ "$rc" == 124 ]] || {
        echo "Linux $edition 冒煙測試未跑滿 25 秒（退出碼 $rc）" >&2; exit 1;
      }
    done
    cat > "$stage/final/smoke/README.txt" <<EOF
版號：$ver
Linux amd64：-version 與完整版號相同；base／plus 各帶唯讀原版素材執行 25 秒，逾時碼 124。
存檔：預設為執行時工作目錄的 saves/；兩版冒煙測試都以 -saves 指向封包外的獨立目錄。
Windows amd64：PE 類型與封包內容已檢查；本機未作 Windows 原生啟動。
macOS amd64／arm64：Mach-O 類型與封包內容已檢查；本機未作 macOS 原生啟動或公證。
四包均未簽章；這些限制不得寫成跨平台實機驗收完成。
EOF

    python3 - "$stage/final" "$ver" <<'PY'
import hashlib, json, os, pathlib, sys
root, version = pathlib.Path(sys.argv[1]), sys.argv[2]
packages = []
for path in sorted((root / 'patch').iterdir()):
    data = path.read_bytes()
    packages.append({'file': 'patch/' + path.name, 'bytes': len(data),
                     'sha256': hashlib.sha256(data).hexdigest(),
                     'rights': 'public_patch_no_original_assets'})
inputs = {}
for name in ('go.mod', 'go.sum', 'LICENSE', 'README.md',
             'fonts/unifont.hex.gz', 'fonts/ascii6x10.hex.gz',
             'fonts/kai.hex.gz', 'fonts/li.hex.gz'):
    inputs[name] = hashlib.sha256((pathlib.Path('/src') / name).read_bytes()).hexdigest()
manifest = {'version': version, 'source_commit': os.environ['SAN1_SOURCE_COMMIT'],
            'tool_images': {'release': os.environ['SAN1_GO_IMAGE_ID'],
                            'macos': os.environ['SAN1_MAC_IMAGE_ID']},
            'source_date_epoch': int(os.environ['SOURCE_DATE_EPOCH']),
            'input_sha256': inputs, 'packages': packages,
            'native_smoke': {'linux-amd64': 'passed_25s',
                             'windows-amd64': 'not_run',
                             'darwin-amd64': 'not_run',
                             'darwin-arm64': 'not_run'},
            'signing': 'unsigned'}
(root / 'SHA256SUMS.json').write_text(
    json.dumps(manifest, ensure_ascii=False, indent=2, sort_keys=True) + '\n', encoding='utf-8')
PY
    mkdir -p /src/dist-all
    mv "$stage/final" "$final"
    echo "已建立 $final"
    ;;
  *)
    echo '僅接受 build／build-mac／finalize 三個容器內階段' >&2
    exit 2
    ;;
esac
