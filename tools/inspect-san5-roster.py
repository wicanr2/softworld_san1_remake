#!/usr/bin/env python3
"""唯讀審查 San5 人物來源。輸出研究資料，不供正式遊戲載入。

契約與限制：docs/re/14-san5-roster.md、docs/spec/022-expanded-roster.md。
在 Docker 內執行，原始輸入唯讀掛載。只依賴 Python 3 與 GNU iconv。
"""

import argparse
import hashlib
import json
import platform
import struct
import subprocess
from collections import defaultdict
from functools import lru_cache
from pathlib import Path


HASHES = {
    "MAIN.EXE": "6c329698bde9722dad66e84c251073246e06a7d1a923fc1d0ba22ee53a543fde",
    "SNDATA.S5": "a43a2a7a9611ab57502f7e328b8cbe9e37951e96900a76f26cbf5ca9a0ffe8fa",
    "TAIKI.S5": "0f63ea49f7e77051d8dce16c2afac89225a51c6d8dcce4ff5d0d6a9bd4a250e0",
}

# 人工審查的異名候選。定位僅限固定來源；不作通用文字正規化。
# 身分對應為 L2，原始姓名、肖像編號及本作姓名一律保留。
ALIAS_REVIEW = [
    ("劉瞱", 167, "劉曄", 69),
    ("糜竺", 161, "麋竺", 75),
    ("糜芳", 162, "麋芳", 120),
    ("費禕", 109, "費褘", 228),
    ("邢道榮", 236, "刑道榮", 169),
    ("雷銅", 227, "雷同", 212),
    ("文鴦", 164, "文鴛", 286),
    ("鮑隆", 468, "鮑龍", 172),
    ("嚴畯", 333, "嚴峻", 329),
]

# DRAFT 城郡錨點，只供配置草案使用。這些分組不是歷史地理邊界的斷言。
CITY_ANCHOR_REVIEW = {
    "襄平": 1, "北平": 2, "代縣": 2, "晉陽": 5, "南皮": 3, "平原": 8,
    "鄴": 4, "北海": 7, "濮陽": 11, "陳留": 11, "洛陽": 15, "弘農": 14,
    "長安": 16, "安定": 17, "天水": 18, "西涼": 19, "下邳": 10, "徐州": 9,
    "許昌": 13, "譙": 12, "汝南": 13, "宛": 27, "新野": 27, "襄陽": 28,
    "上庸": 35, "江夏": 29, "江陵": 29, "武陵": 32, "長沙": 31,
    "桂陽": 33, "零陵": 34, "壽春": 22, "建業": 21, "吳": 23, "會稽": 23,
    "廬江": 22, "豫章": 24, "漢中": 35, "下弁": 35, "梓潼": 36,
    "成都": 36, "永安": 30, "江州": 37, "建寧": 40, "雲南": 38,
    "廬陵": 25, "鄱陽": 24,
}
SUCCESSION_REVIEW = {
    (2, "孫堅"): "孫策", (2, "劉焉"): "劉璋",
    (3, "孫策"): "孫權", (6, "曹操"): "曹丕",
}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def blocks(data):
    count = struct.unpack_from("<H", data)[0]
    require(count == 7, "來源不是已審查的七劇本容器")
    header = 2 + count * 6
    end = header
    result = []
    for i in range(count):
        relative, size = struct.unpack_from("<IH", data, 2 + i * 6)
        start = header + relative
        require(start == end, f"劇本 {i + 1} 目錄不連續")
        end = start + size
        require(end <= len(data), "劇本越界")
        result.append((start, size))
    require(end == len(data), "劇本目錄未覆蓋完整來源檔")
    return result


def font_index(code):
    # IDA sub_3CA6A 的回傳值是從 1 起算，字模檔序號從 0 起算。
    high, low = code
    if 0x30 <= low <= 0x39:
        trail = low - 0x30
    elif 0x41 <= low <= 0x5A:
        trail = low - 0x41 + 10
    elif 0x61 <= low <= 0x7A:
        trail = low - 0x61 + 36
    elif 0x80 <= low <= 0xFD:
        trail = low - 0x80 + 62
    else:
        raise ValueError(f"未支援的姓名尾碼 {code.hex()}")
    if 0x92 <= high <= 0xAF:
        return 590 + (high - 0x92) * 188 + trail, 1
    if 0xB0 <= high <= 0xD8:
        return 6086 + (high - 0xB0) * 188 + trail, 2
    raise ValueError(f"未支援的姓名首碼 {code.hex()}")


def decode(code):
    index, plane = font_index(code)
    ordinal = index + 2606 if plane == 1 else index - 6086
    row, col = divmod(ordinal, 94)
    require(0 <= row < 94, f"CNS 字序越界 {code.hex()}")
    euc = bytes([0xA1 + row, 0xA1 + col])
    if plane == 2:
        euc = b"\x8e\xa2" + euc
    char = subprocess.run(
        ["iconv", "-f", "EUC-TW", "-t", "UTF-8"],
        input=euc, capture_output=True, check=True,
    ).stdout.decode("utf-8")
    require(len(char) == 1, "字碼未解成單一字元")
    return {"font_index": index, "euc_tw": euc.hex(), "character": char}


def source_placements(data, records):
    """只讀取來源位置；不把不同遊戲的城市或君主自動等同。"""
    result = []
    for scenario, (start, _) in enumerate(blocks(data), 1):
        people = {r["slot"]: r for r in records
                  if r["scenario"] == scenario and r["kind"] == "active"}
        cities = []
        for i in range(47):
            offset = start + 0x3DB + 47 * 30 + i * 32
            raw = data[offset:offset + 32]
            end = raw.find(b"\0")
            require(end >= 0 and end % 2 == 0, "來源城市名稱未對齊")
            name = "".join(decode(raw[j:j + 2])["character"] for j in range(0, end, 2))
            cities.append({"id": i + 1, "name": name, "file_offset": offset,
                           "raw": raw.hex()})
        factions = []
        for i in range(21):
            offset = start + 0xF3D + i * 77
            raw = data[offset:offset + 77]
            lord = struct.unpack_from("<H", raw)[0]
            require(lord == 0xFFFF or lord in people, "來源君主索引越界")
            factions.append({"id": i + 1, "leader_slot": lord,
                             "leader_name": people[lord]["name"] if lord != 0xFFFF else None,
                             "file_offset": offset, "raw": raw.hex()})
        for person in [r for r in records if r["scenario"] == scenario]:
            dynamic = bytes.fromhex(person["dynamic"])
            city, faction = dynamic[20:22]
            require(1 <= city <= 47 or city == 0xFF, "來源人物城市索引越界")
            require(1 <= faction <= 21 or faction == 0xFF, "來源人物勢力索引越界")
            person["source_placement"] = {
                "city_id": city, "city_name": cities[city - 1]["name"] if city != 0xFF else None,
                "faction_id": faction,
                "leader_name": factions[faction - 1]["leader_name"] if faction != 0xFF else None,
                "city_file_offset": person["dynamic_offset"] + 20,
                "faction_file_offset": person["dynamic_offset"] + 21,
            }
        result.append({"scenario": scenario, "cities": cities, "factions": factions})
    return result


def target_review(root, edition, records, selection, own_names):
    """比對字面相同的君主與城市。未命中一律留待審查，不猜補。"""
    inputs = {name: (root / name).read_bytes()
              for name in ["DATA2.GRP", "DATA2.IDX", "DATA2.NAM"]}
    grp, idx, nam = (inputs[n] for n in ["DATA2.GRP", "DATA2.IDX", "DATA2.NAM"])
    require(not grp.startswith(b"MZ"), "本作 DATA2 不是裸容器")
    require(len(idx) % 4 == 0 and len(nam) == len(idx) // 4 * 16, "本作容器目錄長度不符")
    tables = {}
    start = 0
    for i in range(len(idx) // 4):
        end = struct.unpack_from("<I", idx, i * 4)[0]
        require(start <= end <= len(grp), "本作容器索引越界")
        name = nam[i * 16:(i + 1) * 16].split(b"\0")[0].decode("ascii").replace(" ", "")
        require(name not in tables, "本作容器名稱重複")
        tables[name] = grp[start:end]
        start = end
    require(start == len(grp), "本作容器目錄未覆蓋完整檔案")
    result = []
    for choice in selection:
        scenario = choice["target_scenario"]
        mas, sta, gen = (tables[f"BASE{kind}.{scenario:03}"] for kind in ["MAS", "STA", "GEN"])
        require((len(mas), len(sta), len(gen)) == (72 * 16, 176 * 43, 30 * 350),
                "本作劇本表長度不符")
        cities = [{"id": i, "name": sta[i * 176:i * 176 + 4].decode("cp950"),
                   "owner_id": sta[i * 176 + 30],
                   "map_x": struct.unpack_from("<H", sta, i * 176 + 6)[0],
                   "map_y": struct.unpack_from("<H", sta, i * 176 + 8)[0]}
                  for i in range(1, 43)]
        target_people = []
        for i in range(350):
            raw = gen[i * 30:(i + 1) * 30]
            name = raw[:6].strip(b" \0").decode("cp950")
            if name in own_names:
                target_people.append({"index": i, "name": name,
                                      "status": raw[17], "faction_id": raw[18],
                                      "location": raw[19]})
        lords = []
        for i in range(16):
            controller, slot = struct.unpack_from("<HH", mas, i * 72)
            if slot >= 350 or controller == 0xFFFF:
                continue
            name = gen[slot * 30:slot * 30 + 6].strip(b" \0").decode("cp950")
            if name in own_names:
                lords.append({"id": i, "leader_name": name, "leader_slot": slot,
                              "controller": controller})
        lord_ids = {p["leader_name"]: p["id"] for p in lords}
        city_ids = {p["name"]: p["id"] for p in cities}
        candidates = []
        for person in records:
            if person["scenario"] != choice["source_scenario"] or person["name"] in own_names:
                continue
            place = person["source_placement"]
            candidates.append({"name": person["name"], "source_portrait": person["source_portrait"],
                               "source_kind": person["kind"], "source_slot": person["slot"],
                               **place,
                               "same_spelling_target_faction": lord_ids.get(place["leader_name"]),
                               "same_spelling_target_city": city_ids.get(place["city_name"])})
        exceptions = defaultdict(list)
        for person in candidates:
            if person["source_kind"] == "active" and person["faction_id"] != 0xFF \
                    and person["same_spelling_target_faction"] is None:
                exceptions[person["leader_name"]].append(person["name"])
        result.append({**choice, "target_lords": lords, "target_cities": cities,
                       "target_people": target_people,
                       "unmatched_active_faction_candidates": dict(exceptions),
                       "candidates": candidates})
    return {"edition": edition,
            "input_sha256": {name: sha(raw) for name, raw in inputs.items()}, "scenarios": result}


def identity_review(identities, people, selection):
    """人工異名審查的可重生摘要；不是正式名單或登場規則。"""
    own = defaultdict(list)
    for person in people:
        own[person["name"]].append(person)
    aliases = {}
    for source_name, portrait, target_name, target_index in ALIAS_REVIEW:
        key = (source_name, portrait)
        require(key in identities, f"異名來源定位不存在：{key}")
        require(source_name not in own, f"異名來源已直接命中本作：{source_name}")
        require(len(own[target_name]) == 1, f"異名目標不唯一：{target_name}")
        require(own[target_name][0]["index"] == target_index,
                f"異名目標槽位不符：{target_name}")
        require(key not in aliases, f"異名來源重複：{key}")
        aliases[key] = (target_name, target_index)
    existing = []
    additions = []
    claimed = set()
    for key, sightings in sorted(identities.items()):
        name, portrait = key
        target = aliases.get(key)
        match = "reviewed_alias_L2"
        if target is None and name in own:
            require(len(own[name]) == 1, f"字面目標不唯一：{name}")
            target = (name, own[name][0]["index"])
            match = "same_spelling; identity_L2"
        entry = {"source_name": name, "source_portrait": portrait,
                 "scenarios": sorted({r["scenario"] for r in sightings})}
        if target is None:
            additions.append(entry)
        else:
            require(target[1] not in claimed, f"兩個來源人物指向同一本作人物：{target}")
            claimed.add(target[1])
            existing.append({**entry, "target_name": target[0],
                             "target_index": target[1], "match": match})
    new_keys = {(r["source_name"], r["source_portrait"]) for r in additions}
    scenarios = []
    for choice in selection:
        selected = [r for key, sightings in identities.items() if key in new_keys
                    for r in sightings if r["scenario"] == choice["source_scenario"]]
        rows = []
        for r in selected:
            birth_byte = bytes.fromhex(r["static"])[21]
            rows.append({"name": r["name"], "source_portrait": r["source_portrait"],
                         "source_kind": r["kind"], "source_slot": r["slot"],
                         "source": r["source"], "static_file_offset": r["file_offset"],
                         "age_field_file_offset": r["file_offset"] + 21,
                         "age_field_byte": birth_byte,
                         "age_formula_result": choice["target_year"] - birth_byte - 99,
                         "source_placement": r["source_placement"]})
        scenarios.append({**choice, "candidate_people": len(rows),
                          "source_active": sum(r["source_kind"] == "active" for r in rows),
                          "source_reserve": sum(r["source_kind"] == "reserve" for r in rows),
                          "age_below_15": sum(r["age_formula_result"] < 15 for r in rows),
                          "age_formula": "target_year - static[0x15] - 99; signed arithmetic for review; not an availability rule",
                          "people": rows})
    return {"status": "RESEARCH_ONLY; L2 identity review; not approved production data",
            "same_spelling_people": len(existing) - len(aliases),
            "reviewed_alias_people": len(aliases), "matched_existing_people": len(existing),
            "reviewed_candidate_people": len(additions),
            "reviewed_candidate_names": len({r["source_name"] for r in additions}),
            "existing": existing, "candidates": additions, "scenarios": scenarios}


def placement_proposal(targets, reviewed, records):
    """可丟棄配置草案；只改輸出，不寫入正式劇本或原始資料。"""
    results = []
    for target in targets:
        scenarios = []
        for base, source in zip(target["scenarios"], reviewed["scenarios"]):
            number = base["target_scenario"]
            cities = {p["id"]: p for p in base["target_cities"]}
            lords = {p["leader_name"]: p for p in base["target_lords"]}
            by_id = {p["id"]: p for p in base["target_lords"]}
            people = {p["name"]: p for p in base["target_people"]}
            new_forces = {}
            mapping = {}
            for row in source["people"]:
                place = row["source_placement"]
                leader = place["leader_name"]
                if row["age_formula_result"] < 15 or row["source_kind"] == "reserve" or leader is None:
                    continue
                if leader in mapping:
                    continue
                successor = SUCCESSION_REVIEW.get((number, leader))
                if leader in lords:
                    mapping[leader] = {"kind": "existing", "faction_id": lords[leader]["id"],
                                       "reason": "same_lord"}
                elif successor is not None:
                    require(successor in lords, f"繼承勢力未在用：{successor}")
                    mapping[leader] = {"kind": "existing", "faction_id": lords[successor]["id"],
                                       "reason": "succession_review", "successor": successor}
                elif leader in people and people[leader]["status"] == 12:
                    mapping[leader] = {"kind": "free", "reason": "original_lord_dead"}
                elif leader in people and people[leader]["status"] in range(4):
                    fid = people[leader]["faction_id"]
                    require(fid in by_id, f"原有在職人物勢力失聯：{leader}")
                    mapping[leader] = {"kind": "existing", "faction_id": fid,
                                       "reason": "original_lord_now_officer"}
                else:
                    sightings = [r for r in records if r["scenario"] == source["source_scenario"]
                                 and r["kind"] == "active" and r["name"] == leader]
                    require(len(sightings) == 1, f"來源君主定位不唯一：{leader}")
                    city = sightings[0]["source_placement"]["city_name"]
                    require(city in CITY_ANCHOR_REVIEW, f"來源城市沒有 DRAFT 錨點：{city}")
                    new_forces[leader] = {"leader": leader, "source_faction_id": place["faction_id"],
                                          "source_city": city, "anchor_id": CITY_ANCHOR_REVIEW[city],
                                          "existing_target_leader": people.get(leader)}
                    mapping[leader] = {"kind": "new", "reason": "source_faction_absent"}
            ordered = sorted(new_forces.values(), key=lambda f: f["source_faction_id"])
            free = sorted(p["id"] for p in cities.values() if p["owner_id"] == 255)
            require(len(free) >= len(ordered), "無主郡不足，不可佔用原有領土")
            unowned_count = len(free)
            assigned_by_leader = {}
            # 先保留本身無主的來源錨點；同一錨點衝突按來源勢力 ID 處理。
            for force in ordered:
                if force["anchor_id"] in free:
                    assigned_by_leader[force["leader"]] = force["anchor_id"]
                    free.remove(force["anchor_id"])
            pending = [f for f in ordered if f["leader"] not in assigned_by_leader]
            # 剩餘項最小化地圖座標平方距離總和；同分按郡號序列固定結果。
            @lru_cache(maxsize=None)
            def assign(mask):
                row = mask.bit_count()
                if row == len(pending):
                    return 0, ()
                anchor = cities[pending[row]["anchor_id"]]
                best = None
                for j, city_id in enumerate(free):
                    if mask & (1 << j):
                        continue
                    city = cities[city_id]
                    score = (anchor["map_x"] - city["map_x"]) ** 2 + (anchor["map_y"] - city["map_y"]) ** 2
                    tail, ids = assign(mask | (1 << j))
                    candidate = score + tail, (city_id,) + ids
                    if best is None or candidate < best:
                        best = candidate
                return best
            distance, assigned = assign(0)
            assign.cache_clear()
            assigned_by_leader.update((f["leader"], cid) for f, cid in zip(pending, assigned))
            for i, force in enumerate(ordered):
                city_id = assigned_by_leader[force["leader"]]
                force.update({"proposed_faction_id": 16 + i, "proposed_city_id": city_id,
                              "proposed_city_name": cities[city_id]["name"],
                              "anchor_name": cities[force["anchor_id"]]["name"]})
                mapping[force["leader"]].update({"faction_id": 16 + i, "city_id": city_id})
            rows = []
            for p in source["people"]:
                anchor = CITY_ANCHOR_REVIEW[p["source_placement"]["city_name"]]
                disposition = {"kind": "free", "reason": "source_free_or_reserve"}
                leader = p["source_placement"]["leader_name"]
                if p["age_formula_result"] < 15:
                    disposition = {"kind": "not_debuted", "reason": "below_15"}
                elif p["source_kind"] == "active" and leader is not None:
                    disposition = dict(mapping[leader])
                city_id = anchor
                if disposition["kind"] == "existing":
                    fid = disposition["faction_id"]
                    if cities[anchor]["owner_id"] != fid:
                        lord_name = by_id[fid]["leader_name"]
                        city_id = people[lord_name]["location"]
                    require(city_id in cities and cities[city_id]["owner_id"] == fid,
                            f"既有君主所在地不屬該勢力：{fid}")
                elif disposition["kind"] == "new":
                    city_id = disposition["city_id"]
                rows.append({"name": p["name"], "source_portrait": p["source_portrait"],
                             "age": p["age_formula_result"], "anchor_id": anchor,
                             "proposed_city_id": city_id, "proposed_city_name": cities[city_id]["name"],
                             "disposition": disposition})
            scenarios.append({"target_scenario": number, "new_forces": ordered,
                              "faction_mapping": mapping, "assignment_cost": distance,
                              "assignment_policy": "unowned_anchor_first; source_faction_id_tiebreak; remaining_min_sum_squared_map_distance",
                              "unowned_city_count": unowned_count, "people": rows})
        results.append({"edition": target["edition"], "scenarios": scenarios})
    return {"status": "DRAFT_DISPOSABLE_PROPOSAL; city anchors, disposition and assignments require spec review",
            "city_anchors": CITY_ANCHOR_REVIEW, "editions": results}


def inspect(source, prototype, catalog, targets):
    data = {}
    for name, expected in HASHES.items():
        data[name] = (source / name).read_bytes()
        require(sha(data[name]) == expected, f"{name} SHA-256 不符")
    records = []
    summaries = []
    mapping = {}
    for name in ["SNDATA.S5", "TAIKI.S5"]:
        for scenario, (start, size) in enumerate(blocks(data[name]), 1):
            active = name == "SNDATA.S5"
            body = size - 5518 - 64 if active else size
            require(body >= 0 and body % 69 == 0, "人物區段未對齊 31+38")
            count = body // 69
            summaries.append({"source": name, "scenario": scenario,
                              "block_offset": start, "block_size": size, "count": count})
            if active:
                summaries[-1]["stored_date"] = {
                    "year": struct.unpack_from("<H", data[name], start + 0x241)[0],
                    "month": data[name][start + 0x247],
                    "year_file_offset": start + 0x241,
                    "month_file_offset": start + 0x247,
                }
            for i in range(count):
                dyn_off = start + 5518 + i * 31 if active else start + i * 69
                stat_off = start + 5518 + count * 31 + i * 38 if active else dyn_off + 31
                dynamic = data[name][dyn_off:dyn_off + 31]
                static = data[name][stat_off:stat_off + 38]
                end = static[:20].find(b"\0")
                require(end >= 0 and end % 2 == 0, "姓名沒有完整雙位元組結尾")
                raw_name = static[:end]
                chars = []
                for k in range(0, len(raw_name), 2):
                    code = raw_name[k:k + 2]
                    if code.hex() not in mapping:
                        mapping[code.hex()] = decode(code)
                    chars.append(mapping[code.hex()]["character"])
                records.append({
                    "scenario": scenario, "kind": "active" if active else "reserve",
                    "slot": i + 1, "name": "".join(chars), "raw_name": raw_name.hex(),
                    "source": name, "file_offset": stat_off, "dynamic_offset": dyn_off,
                    "static": static.hex(), "dynamic": dynamic.hex(),
                    "source_portrait": struct.unpack_from("<H", static, 30)[0],
                    "abilities": dict(zip(["war", "intel", "politics", "charm"], dynamic[12:16])),
                })
    old = json.loads(prototype.read_text())
    fields = ["scenario", "kind", "slot", "name", "raw_name", "source", "file_offset", "static", "dynamic"]
    def signature(record):
        return tuple(record[f] for f in fields)
    require(sorted(map(signature, records)) == sorted(map(signature, old["records"])),
            "獨立原始檔回讀與上輪試作不符")
    require(mapping == old["glyph_map"], "獨立 CNS 解碼與已審查字形映射不符")
    placements = source_placements(data["SNDATA.S5"], records)
    labels = []
    for ea, expected in [(0x599A0, "武力"), (0x599AA, "智力"),
                         (0x599B4, "政治"), (0x599BE, "魅力")]:
        offset = ea - 0x10000 + 0x4800
        raw = data["MAIN.EXE"][offset:offset + 4]
        actual = decode(raw[:2])["character"] + decode(raw[2:])["character"]
        require(actual == expected, f"能力標籤不符 {ea:#x}")
        labels.append({"ida_ea": hex(ea), "file_offset": offset,
                       "raw": raw.hex(), "text": actual})
    people = json.loads(catalog.read_text())["people"]
    own_names = {p["name"] for p in people}
    identities = defaultdict(list)
    for record in records:
        identities[record["name"], record["source_portrait"]].append(record)
    candidates = []
    for (name, portrait), sightings in sorted(identities.items()):
        if name in own_names:
            continue
        first = min(sightings, key=lambda r: r["scenario"])
        candidates.append({"name": name, "source_portrait": portrait,
                           "abilities_first_scenario": first["abilities"],
                           "scenarios": sorted({r["scenario"] for r in sightings}),
                           "first_source": first["source"], "first_file_offset": first["file_offset"]})
    # 僅決定採哪份資料；不推測中間年分的出生、死亡、移籍與城市。
    source_dates = [(b["stored_date"]["year"], b["stored_date"]["month"], b["scenario"])
                    for b in summaries if b["source"] == "SNDATA.S5"]
    selection = []
    for scenario, year in enumerate([189, 195, 201, 208, 215, 220], 1):
        eligible = [d for d in source_dates if d[:2] <= (year, 1)]
        require(bool(eligible), "本作劇本之前沒有 San5 資料")
        source_year, source_month, source_scenario = max(eligible)
        selection.append({"target_scenario": scenario, "target_year": year, "target_month": 1,
                          "source_scenario": source_scenario, "stored_source_year": source_year,
                          "stored_source_month": source_month,
                          "policy": "latest_source_not_after_target; user approved 2026-10-11"})
    iconv_version = subprocess.run(["iconv", "--version"], capture_output=True, text=True, check=True).stdout.splitlines()[0]
    reviewed = identity_review(identities, people, selection)
    target_results = [target_review(root, edition, records, selection, own_names)
                      for edition, root in targets]
    return {
        "schema": 2, "status": "RESEARCH_ONLY; candidates are not the approved roster",
        "tools": {"python": platform.python_version(), "iconv": iconv_version},
        "input_sha256": {**HASHES, "prototype": sha(prototype.read_bytes()),
                         "own_catalog": sha(catalog.read_bytes())},
        "script_sha256": sha(Path(__file__).read_bytes()),
        "address_space": "檔案位移；能力標籤另標 IDA 線性位址，root file=IDA-0x10000+0x4800",
        "checks": {"raw_records": len(records), "unique_names": len({r['name'] for r in records}),
                   "name_portrait_pairs": len(identities), "glyphs": len(mapping),
                   "raw_record_mismatches": 0, "glyph_mapping_mismatches": 0,
                    "exact_spelling_candidate_people": len(candidates),
                    "reviewed_alias_people": reviewed["reviewed_alias_people"],
                    "reviewed_candidate_people": reviewed["reviewed_candidate_people"]},
        "scenario_blocks": summaries, "scenario_selection": selection, "ability_labels": labels,
        "source_placements": placements,
        "target_review": target_results,
        "identity_review": reviewed,
        "placement_proposal": placement_proposal(target_results, reviewed, records),
        "exact_spelling_candidates": candidates, "glyph_map": mapping, "records": records,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--prototype", type=Path, required=True)
    parser.add_argument("--catalog", type=Path, required=True)
    parser.add_argument("--target-base", type=Path)
    parser.add_argument("--target-plus", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    targets = [(edition, root) for edition, root in [("base", args.target_base), ("plus", args.target_plus)]
               if root is not None]
    result = inspect(args.source, args.prototype, args.catalog, targets)
    require(args.output.parent.is_dir(), "輸出目錄不存在")
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(result["checks"], ensure_ascii=False))


if __name__ == "__main__":
    main()
