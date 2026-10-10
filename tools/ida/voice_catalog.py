"""Non-mutating IDA 9.4 voice-catalog evidence export; run on a disposable DB copy."""
import hashlib
import json
import struct
from pathlib import Path
import idaapi
import ida_auto
import ida_bytes
import ida_funcs
import ida_ida
import ida_nalt
import ida_segment
import ida_ua
import idautils
import idc

ida_auto.auto_wait()
out = Path(idc.ARGV[1])
source = Path(idc.ARGV[2])
source_bytes = source.read_bytes()
assert hashlib.sha256(source_bytes).digest() == ida_nalt.retrieve_input_file_sha256()
seed_results = []
mode = idc.ARGV[3] if len(idc.ARGV) > 3 else ''
plus = mode == 'seed-plus'
message, message_end, message_cs, message_offset = ((0x2f366, 0x2f9ab, 0x2f36, 6)
                                                  if plus else (0x3273e, 0x32df9, 0x3273, 0xe))
loader, loader_end = (0x59d8, 0x5ad6) if plus else (0x5a76, 0x5b80)
if mode in ('seed', 'seed-plus'):
    # The raw snapshot's single 16-bit segment wraps near branches above 64 KiB.
    # All 111 original far-call operands identify 3273:000E for the message entry.
    code_base = message_cs * 16
    code_end = code_base + 65536
    original = ida_segment.getseg(message)
    if ida_segment.get_segm_base(original) != code_base:
        start, end = original.start_ea, original.end_ea
        ida_segment.del_segm(start, ida_segment.SEGMOD_KEEP)
        for lo, hi, base in [(start, code_base, start), (code_base, code_end, code_base),
                             (code_end, end, code_end)]:
            segment = ida_segment.segment_t()
            segment.start_ea, segment.end_ea = lo, hi
            segment.sel = ida_segment.setup_selector(base >> 4)
            segment.bitness = 0
            assert ida_segment.add_segm_ex(segment, f'raw_{lo:X}', 'CODE', 0)
    # Define only confirmed prologues in the disposable copy of a raw-image DB.
    for address, end in [(message, message_end), (loader, loader_end)]:
        assert ida_bytes.get_bytes(address, 3) == b'\x55\x8b\xec'
        if address == message:
            epilogue = bytes.fromhex('2bc05e5fc9cb' if plus else '8be55dcb')
            assert ida_bytes.get_bytes(end - len(epilogue), len(epilogue)) == epilogue
        ida_bytes.del_items(address, ida_bytes.DELIT_SIMPLE, end - address)
        made = ida_ua.create_insn(address)
        added = ida_funcs.add_func(address, end)
        seed_results.append({'ida_ea': address, 'instruction_created': made, 'function_added': bool(added)})
    ida_auto.auto_wait()
segments = []
for ea in idautils.Segments():
    s = ida_segment.getseg(ea)
    segments.append({'name': ida_segment.get_segm_name(s), 'start': s.start_ea, 'end': s.end_ea,
                     'base': ida_segment.get_segm_base(s), 'bitness': s.bitness})
functions = []
for address in ([message, loader] if mode in ('seed', 'seed-plus')
                else [0x3373e, 0x3273e, 0x4163e, 0x2273e, 0x14976, 0x5a76]):
    f = ida_funcs.get_func(address)
    if f is None:
        continue
    lines = []
    for ea in idautils.FuncItems(f.start_ea):
        lines.append({'ida_ea': ea, 'bytes': ida_bytes.get_bytes(ea, idc.get_item_size(ea)).hex(),
                      'instruction': idc.generate_disasm_line(ea, 0)})
    functions.append({'requested_ida_ea': address, 'name': idc.get_func_name(f.start_ea),
                      'ida_start': f.start_ea, 'ida_end': f.end_ea, 'lines': lines,
                      'xrefs_to': [{'ida_from': x.frm, 'type': x.type} for x in idautils.XrefsTo(f.start_ea)]})
caller_contexts = []
if mode in ('seed', 'seed-plus'):
    base = 0x1100
    signature = b'\x9a' + struct.pack('<HH', message_offset, message_cs)
    prologue = bytes.fromhex('558bec')
    offset = 0
    while True:
        site = source_bytes.find(signature, offset)
        if site < 0:
            break
        offset = site + 1
        start = source_bytes.rfind(prologue, max(0, site - 8192), site)
        if start < 0:
            caller_contexts.append({'linear_call': site + base, 'error': 'no prologue candidate'})
            continue
        ida_bytes.del_items(start + base, ida_bytes.DELIT_SIMPLE, site + 5 - start)
        ea, rows = start + base, []
        while ea < site + base + 5:
            size = ida_ua.create_insn(ea)
            if not size:
                break
            if ea >= site + base - 192:
                rows.append({'linear_ea': ea, 'bytes': ida_bytes.get_bytes(ea, size).hex(),
                             'instruction': idc.generate_disasm_line(ea, 0)})
            ea += size
        caller_contexts.append({'linear_call': site + base, 'prologue_candidate': start + base,
                                'decoded_to_call': ea == site + base + 5, 'context': rows,
                                'near_branch_targets': 'not used; caller CS requires independent validation'})
branch_probe = None
if mode in ('seed', 'seed-plus'):
    function = next(f for f in functions if f['requested_ida_ea'] == message)
    for line in function['lines']:
        raw = bytes.fromhex(line['bytes'])
        if raw[0] == 0xeb and len(raw) == 2:
            delta = struct.unpack('<b', raw[1:])[0]
        elif raw[0] == 0xe9 and len(raw) == 3:
            delta = struct.unpack('<h', raw[1:])[0]
        else:
            continue
        cs_base = message_cs * 16
        target_ip = (line['ida_ea'] - cs_base + len(raw) + delta) & 0xffff
        branch_probe = {'ida_ea': line['ida_ea'], 'raw': line['bytes'],
                        'cs_base_ida': cs_base, 'target_ip_offset': target_ip,
                        'target_ida_ea': cs_base + target_ip, 'disassembly': line['instruction']}
        break
result = {'probe': True, 'tool': 'IDA Pro ' + idaapi.get_kernel_version(),
          'address_space': 'IDA database effective address; segment bases preserved',
          'input_name': source.name, 'input_sha256': hashlib.sha256(source_bytes).hexdigest(),
          'database_input_name': ida_nalt.get_root_filename(), 'function_count': ida_funcs.get_func_qty(),
          'min_ea': ida_ida.inf_get_min_ea(), 'max_ea': ida_ida.inf_get_max_ea(),
          'segments': segments, 'functions': functions, 'seed_results': seed_results,
          'caller_contexts': caller_contexts,
          'segment_registers': [{'ida_ea': ea, 'cs': idc.get_sreg(ea, 'cs'),
                                'ds': idc.get_sreg(ea, 'ds')} for ea in [message, loader]],
          'branch_probe': branch_probe}
out.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
idc.qexit(0)
