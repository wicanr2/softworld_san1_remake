"""Non-destructive candidate export for map-coordinate and selection timers."""
import hashlib
import json
import sys
import ida_auto
import ida_bytes
import ida_funcs
import ida_nalt
import ida_pro
import idautils
import idc
import idaapi

ida_auto.auto_wait()
path = ida_nalt.get_input_file_path()
rows = []
functions = list(idautils.Functions())
for start in functions:
    items = list(idautils.FuncItems(start))
    constants = {idc.get_operand_value(ea, n) for ea in items for n in range(3)
                 if idc.get_operand_type(ea, n) == idc.o_imm}
    if not ({0x50, 0x2c}.issubset(constants) or
            {0x200, 0x0f}.issubset(constants)):
        continue
    rows.append({'original_name': ida_funcs.get_func_name(start),
                 'original_address': hex(start), 'grade': 'candidate; semantics unconfirmed',
                 'xrefs': [hex(x.frm) for x in idautils.XrefsTo(start) if x.iscode],
                 'instructions': [{'address': hex(ea), 'bytes':
                    (ida_bytes.get_bytes(ea, idc.get_item_size(ea)) or b'').hex(),
                    'instruction': idc.generate_disasm_line(ea, 0)} for ea in items]})
with open(sys.argv[1], 'w', encoding='utf8') as output:
    json.dump({'tool': 'IDA Pro ' + idaapi.get_kernel_version(),
               'address_space': 'IDA database linear EA; not runtime physical address',
               'input_file': path, 'input_sha256': hashlib.sha256(open(path, 'rb').read()).hexdigest(),
               'function_count': len(functions), 'candidates': rows}, output, ensure_ascii=False, indent=2)
ida_pro.qexit(0)
