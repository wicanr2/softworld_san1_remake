#!/usr/bin/env python3
"""在 Docker 用 Pillow 合成完整素材圖錄；不輸出原版單張圖到公開目錄。"""
import argparse
import gzip
import hashlib
import json
import math
import os
from collections import Counter, defaultdict
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont


CATEGORIES = [
    "人物肖像", "事件場景", "地形與天候", "旗幟與行軍", "選單與地圖",
    "面板與圖框", "片頭與製作群", "游標與遮罩", "其他拼件",
]
COLS, CELL_W, CELL_H, MARGIN = 24, 128, 160, 24


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def category(name):
    if name.endswith(".FAC"):
        return CATEGORIES[0]
    if name.startswith("SCG"):
        return CATEGORIES[1]
    if name.startswith(("EICON", "WEATHER")):
        return CATEGORIES[2]
    if name.startswith(("WFLAG", "CVSC", "MVM", "MVO")):
        return CATEGORIES[3]
    if name.startswith(("MENU", "MAINMAP")):
        return CATEGORIES[4]
    if name.startswith(("PANEL", "SIDE", "FBR", "8x8")):
        return CATEGORIES[5]
    if name.startswith(("CMARK", "SANT", "TITL", "ENDO", "REC", "UPR", "CP")):
        return CATEGORIES[6]
    if name.startswith(("CUR", "MAPCUR", "MNGCUR")):
        return CATEGORIES[7]
    return CATEGORIES[8]


def read_original(extracted):
    grouped = {}
    counts = {}
    for edition in ("base", "plus"):
        root = extracted / edition
        files = json.loads((root / "manifest.json").read_text())["files"]
        tiles = json.loads((root / "terrain.json").read_text())["files"]
        counts[edition] = {"images": len(files), "terrain": len(tiles)}
        assert len(files) == 613
        for e in files + tiles:
            path = root / e["path"]
            assert sha(path) == e["output_sha256"], path
            key = (e["container"], e["from"], e["source_sha256"])
            if key not in grouped:
                grouped[key] = {
                    "path": path, "name": e["from"], "container": e["container"],
                    "source_sha256": e["source_sha256"], "png_sha256": sha(path),
                    "editions": [],
                }
            grouped[key]["editions"].append(edition)
    return list(grouped.values()), counts


def read_hd(root):
    manifest = json.loads((root / "manifest.json").read_text())
    grouped = defaultdict(list)
    for e in manifest["entries"]:
        grouped[e["file"]].append(e)
    assert len(grouped) == 437 and len(manifest["entries"]) == 904
    rows = []
    for file, entries in sorted(grouped.items()):
        path = root / file
        digest = sha(path)
        with Image.open(path) as im:
            im.load()
            assert all(e["sha256"] == digest and (e["width"], e["height"]) == im.size for e in entries)
        sources = [{k: e[k] for k in ("edition", "container", "name", "source_sha256")} for e in entries]
        rows.append({
            "path": path, "file": file, "name": entries[0]["name"],
            "container": entries[0]["container"], "png_sha256": digest,
            "editions": sorted({e["edition"] for e in entries}), "sources": sources,
        })
    return rows, {"entries": len(manifest["entries"]), "per_edition": dict(Counter(e["edition"] for e in manifest["entries"])), "manifest_sha256": sha(root / "manifest.json")}


class Labels:
    def __init__(self, font_path):
        self.glyphs = {}
        with gzip.open(font_path, "rt") as f:
            for line in f:
                if ":" in line and not line.startswith("#"):
                    code, data = line.strip().split(":", 1)
                    self.glyphs[int(code, 16)] = bytes.fromhex(data)
        self.small = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)

    def heading(self, canvas, pos, text, scale=2, color="#e7d3a0"):
        x, y = pos
        for c in text:
            data = self.glyphs.get(ord(c))
            if data is None:
                raise ValueError(f"總覽字型缺字：{c}")
            w = len(data) // 2
            mask = Image.frombytes("1", (w, 16), data)
            mask = mask.resize((w * scale, 16 * scale), Image.Resampling.NEAREST)
            canvas.paste(color, (x, y), mask)
            x += w * scale


def render(rows, output, title, note, labels, high=False):
    groups = defaultdict(list)
    for row in rows:
        groups[category(row["name"])].append(row)
    height = 128 + MARGIN + sum(52 + math.ceil(len(groups[c]) / COLS) * CELL_H for c in CATEGORIES if groups[c])
    canvas = Image.new("RGB", (MARGIN * 2 + COLS * CELL_W, height), "#131a23")
    draw = ImageDraw.Draw(canvas)
    labels.heading(canvas, (MARGIN, 22), title, scale=3)
    labels.heading(canvas, (MARGIN, 82), note, scale=1, color="#b9c7d4")
    y = 128
    records = []
    for c in CATEGORIES:
        items = sorted(groups[c], key=lambda row: (row["container"], row["name"], row["editions"]))
        if not items:
            continue
        draw.rectangle((MARGIN, y, canvas.width - MARGIN, y + 42), fill="#253342")
        labels.heading(canvas, (MARGIN + 12, y + 5), f"{c}  {len(items)}", scale=2)
        y += 52
        for i, row in enumerate(items):
            x, top = MARGIN + (i % COLS) * CELL_W, y + (i // COLS) * CELL_H
            box = (x + 4, top + 4, x + CELL_W - 4, top + 118)
            draw.rectangle((x + 1, top + 1, x + CELL_W - 1, top + CELL_H - 3), fill="#1c2632")
            for sy in range(box[1], box[3], 8):
                for sx in range(box[0], box[2], 8):
                    fill = "#354251" if ((sx - box[0]) // 8 + (sy - box[1]) // 8) % 2 else "#2c3744"
                    draw.rectangle((sx, sy, min(sx + 7, box[2] - 1), min(sy + 7, box[3] - 1)), fill=fill)
            with Image.open(row["path"]) as src:
                src.load()
                size = src.size
                ratio = min((box[2] - box[0]) / size[0], (box[3] - box[1]) / size[1])
                thumb = src.convert("RGBA").resize((max(1, round(size[0] * ratio)), max(1, round(size[1] * ratio))), Image.Resampling.LANCZOS if high else Image.Resampling.NEAREST)
                canvas.paste(thumb, (box[0] + (box[2] - box[0] - thumb.width) // 2, box[1] + (box[3] - box[1] - thumb.height) // 2), thumb)
            name = row["name"]
            if high and len({e["name"] for e in row["sources"]}) > 1:
                name += "+"
            assert draw.textlength(name, font=labels.small) <= CELL_W - 6, name
            draw.text((x + 4, top + 122), name, font=labels.small, fill="#f2ead6")
            edition = "both" if len(row["editions"]) == 2 else row["editions"][0]
            draw.text((x + 4, top + 139), f"{size[0]}x{size[1]} / {edition}", font=labels.small, fill="#a9bacb")
            record = {k: v for k, v in row.items() if k != "path"}
            record.update(category=c, width=size[0], height=size[1], tile=[x, top, CELL_W, CELL_H])
            records.append(record)
        y += math.ceil(len(items) / COLS) * CELL_H
    canvas.save(output, optimize=True)
    with Image.open(output) as check:
        check.load()
        assert check.size == canvas.size
    return {"file": output.name, "sha256": sha(output), "width": canvas.width, "height": canvas.height, "count": len(records), "categories": {c: len(groups[c]) for c in CATEGORIES if groups[c]}, "items": records}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("/src"))
    parser.add_argument("--extracted", type=Path, default=Path("/src/workplace/asset-overviews"))
    args = parser.parse_args()
    output = args.root / "docs/images"
    assert output.is_dir() and output.stat().st_uid == os.getuid()
    for name in ("assets-original-overview.png", "assets-hd-overview.png", "asset-overviews.json"):
        dest = output / name
        assert not dest.exists() or dest.stat().st_uid == os.getuid(), dest
    original, counts = read_original(args.extracted)
    hd, pack = read_hd(args.root / "hd-assets")
    labels = Labels(args.root / "fonts/unifont.hex.gz")
    a = render(original, output / "assets-original-overview.png", "三國演義　原版素材總覽", "原版與加強版　相同素材合併展示　保留全部圖像與地形槽位　原始像素採最近鄰縮放", labels)
    b = render(hd, output / "assets-hd-overview.png", "三國演義　HD 素材總覽", "寫實手繪　四倍高清　正式素材包全部圖片　透明區採棋盤底　不同資源可能共用同一張圖", labels, high=True)
    index = {"schema": 1, "original_exports": counts, "hd_pack": pack, "original": a, "hd": b}
    (output / "asset-overviews.json").write_text(json.dumps(index, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"original": {k: v for k, v in a.items() if k != "items"}, "hd": {k: v for k, v in b.items() if k != "items"}, "exports": counts}, ensure_ascii=False))


if __name__ == "__main__":
    main()
