#!/usr/bin/env python3
"""從已審查收據匯出規格 022 的型別資料。只在 Docker 內執行。"""
import argparse
import hashlib
import json
import os
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--receipt', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    r = json.loads(args.receipt.read_text())
    if r['checks']['reviewed_candidate_people'] != 220:
        raise ValueError('候選人數不符已確認契約')
    editions = r['placement_proposal']['editions']
    if editions[0]['scenarios'] != editions[1]['scenarios']:
        raise ValueError('兩版配置不一致，須回到規格審查')
    raw = {(p['scenario'], p['name'], p['source_portrait']): p for p in r['records']}
    portraits = {p['source_portrait'] for p in r['records']}
    identities = {(p['name'],p['source_portrait']) for p in r['records']}
    if len(portraits) != len(identities):
        raise ValueError('來源肖像編號不是唯一人物識別')
    result = {'schema':1,'source_sha256':{k:r['input_sha256'][k] for k in ('MAIN.EXE','SNDATA.S5','TAIKI.S5')},
              'review_sha256':hashlib.sha256(args.receipt.read_bytes()).hexdigest(), 'scenarios':[]}
    for selection, plan in zip(r['scenario_selection'], editions[0]['scenarios']):
        source_number = selection['source_scenario']
        rows = []
        for person in sorted(plan['people'],key=lambda p:p['source_portrait']):
            original = raw[source_number,person['name'],person['source_portrait']]
            d = person['disposition']
            rows.append({'name':person['name'], 'portrait':person['source_portrait'],
                         'age':person['age'],'origin':person['anchor_id'],
                         'location':person['proposed_city_id'], 'kind':d['kind'],
                         'faction':d.get('faction_id',255),
                         **original['abilities'], 'source_file':original['source'],
                         'source_offset':original['file_offset']})
        forces = []
        for f in plan['new_forces']:
            lord = f['existing_target_leader']
            matches = [p for p in rows if p['name']==f['leader']]
            if lord is None and len(matches)!=1:
                raise ValueError('新增君主來源不唯一：'+f['leader'])
            forces.append({'id':f['proposed_faction_id'],'city':f['proposed_city_id'],
                           'leader_index':lord['index'] if lord else -1,
                           'leader_portrait':matches[0]['portrait'] if lord is None else -1})
        result['scenarios'].append({'slot':f"{selection['target_scenario']:03}",
                                    'source_scenario':source_number,'people':rows,'factions':forces})
    if args.output.parent.stat().st_uid != os.getuid():
        raise ValueError('輸出目錄不屬於目前使用者')
    if args.output.exists() and args.output.stat().st_uid != os.getuid():
        raise ValueError('輸出檔不屬於目前使用者')
    args.output.write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'output':str(args.output),'people':[len(s['people']) for s in result['scenarios']],
                      'sha256':hashlib.sha256(args.output.read_bytes()).hexdigest()},ensure_ascii=False))


if __name__ == '__main__':
    main()
