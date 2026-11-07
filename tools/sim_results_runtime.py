"""Validate LamdaUI's generated result data without evaluating Lua.

Package seeds remain hash-checked code. Only the explicitly opted-in runtime data files may
vary after installation; all other addon files retain ordinary integrity checks.
"""
from pathlib import Path
import hashlib
import json
import math
import re

RELATIVE = 'lamdaUI/SimResults.lua'
MARKER = b'-- LamdaUI managed sim results v1\n'
SEED = MARKER + b'LamdaSimResults = {}\n'
MAX_BYTES = 2 * 1024 * 1024
TEMP_NAME = re.compile(r'\.(?:sim-results|kbb-data|overlays)-[0-9]+\Z')
KBB_RELATIVE = 'lamdaUI/KBBData.lua'
KBB_MARKER = b'-- LamdaUI managed KBB data v1\n'
KBB_SEED = KBB_MARKER + b'LamdaKBBData = {}\n'
KBB_MAX_BYTES = 16 * 1024 * 1024
OVERLAYS_RELATIVE = 'lamdaUI/OverlaysData.lua'
OVERLAYS_MARKER = b'-- LamdaUI managed Overlays data v1\n'
OVERLAYS_SEED = OVERLAYS_MARKER + b'LamdaOverlaysData = {["hash"]="4f53cda18c2baa0c",["watches"]={},}\n'
OVERLAYS_MAX_BYTES = 64 * 1024
NUMBER = re.compile(rb'-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?')


class InvalidResults(ValueError):
    pass


class _LiteralReader:
    """The exact small data grammar emitted by the Go bridge, not a Lua parser."""
    def __init__(self, data, node_limit=200000):
        self.data, self.pos, self.nodes = data, 0, 0
        self.node_limit = node_limit

    def whitespace(self):
        while self.pos < len(self.data) and self.data[self.pos] in b' \r\n\t':
            self.pos += 1

    def take(self, token):
        self.whitespace()
        if not self.data.startswith(token, self.pos):
            raise InvalidResults('Unexpected results data syntax')
        self.pos += len(token)

    def string(self):
        self.take(b'"')
        result = bytearray()
        while self.pos < len(self.data):
            c = self.data[self.pos]
            self.pos += 1
            if c == 34:
                if len(result) > 64 * 1024:
                    raise InvalidResults('Results string exceeds limit')
                try:
                    return result.decode('utf-8')
                except UnicodeDecodeError as error:
                    raise InvalidResults('Invalid results text') from error
            if c == 92:
                tail = self.data[self.pos:self.pos + 3]
                if tail[:1] in (b'"', b'\\'):
                    result.extend(tail[:1]); self.pos += 1
                elif len(tail) == 3 and all(48 <= n <= 57 for n in tail) and int(tail) <= 255:
                    result.append(int(tail)); self.pos += 3
                else:
                    raise InvalidResults('Invalid results escape')
            elif c < 32 or c == 127:
                raise InvalidResults('Unescaped results control character')
            else:
                result.append(c)
            if len(result) > 64 * 1024:
                raise InvalidResults('Results string exceeds limit')
        raise InvalidResults('Unterminated results string')

    def value(self, depth=0):
        self.nodes += 1
        if depth > 32 or self.nodes > self.node_limit:
            raise InvalidResults('Results structure exceeds limit')
        self.whitespace()
        c = self.data[self.pos:self.pos + 1]
        if c == b'"':
            return self.string()
        if c == b'{':
            self.pos += 1
            self.whitespace()
            values, fields, mode = [], {}, None
            while self.data[self.pos:self.pos + 1] != b'}':
                self.whitespace()
                if self.data[self.pos:self.pos + 1] == b'[':
                    if mode == 'array':
                        raise InvalidResults('Mixed results table')
                    mode = 'object'
                    self.pos += 1
                    key = self.string()
                    self.take(b']'); self.take(b'=')
                    if key in fields:
                        raise InvalidResults('Duplicate results field')
                    fields[key] = self.value(depth + 1)
                else:
                    if mode == 'object':
                        raise InvalidResults('Mixed results table')
                    mode = 'array'
                    values.append(self.value(depth + 1))
                self.take(b',')
                self.whitespace()
            self.pos += 1
            return values if mode == 'array' else fields
        for token, value in ((b'true', True), (b'false', False), (b'nil', None)):
            if self.data.startswith(token, self.pos):
                self.pos += len(token)
                return value
        match = NUMBER.match(self.data, self.pos)
        if match:
            token = match[0]
            self.pos = match.end()
            value = float(token)
            if not math.isfinite(value):
                raise InvalidResults('Nonfinite results number')
            return int(token) if b'.' not in token and b'e' not in token.lower() else value
        raise InvalidResults('Results must contain only data literals')


def validate_results(data, client):
    if client not in ('retail', 'forever') or not isinstance(data, bytes) or len(data) > MAX_BYTES:
        raise InvalidResults('Invalid results size or client')
    if data == SEED:
        return {}
    parts = data.split(b'\n', 3)
    if len(parts) != 4 or parts[0] + b'\n' != MARKER:
        raise InvalidResults('Results marker missing')
    if not re.fullmatch(rb'-- Content digest: [0-9a-f]{64}', parts[1]):
        raise InvalidResults('Results content digest missing')
    if not re.fullmatch(rb'-- Payload digest: [0-9a-f]{64}', parts[2]):
        raise InvalidResults('Results payload digest missing')
    if hashlib.sha256(parts[3]).hexdigest().encode() != parts[2].split(b': ', 1)[1]:
        raise InvalidResults('Results payload checksum mismatch')
    reader = _LiteralReader(parts[3])
    reader.take(b'LamdaSimResults = ')
    value = reader.value()
    reader.whitespace()
    if reader.pos != len(reader.data):
        raise InvalidResults('Unexpected executable results content')
    if not isinstance(value, dict) or set(value) != {'schema', 'client', 'generatedAt', 'snapshots', 'jobs', 'omittedSnapshots', 'omittedJobs'}:
        raise InvalidResults('Invalid results envelope')
    if type(value['schema']) is not int or type(value['schema']) is not int or value['schema'] != 1 or value['client'] != client:
        raise InvalidResults('Wrong results schema or client')
    if type(value['generatedAt']) is not int or value['generatedAt'] <= 0:
        raise InvalidResults('Invalid results timestamp')
    for key in ('omittedSnapshots', 'omittedJobs'):
        if type(value[key]) is not int or value[key] < 0:
            raise InvalidResults('Invalid results omission count')
    for key, maximum in (('snapshots', 56), ('jobs', 48)):
        entries = value[key]
        if entries == {}:  # An empty Lua table represents an empty JSON array.
            entries = value[key] = []
        if not isinstance(entries, list) or len(entries) > maximum or any(not isinstance(v, dict) for v in entries):
            raise InvalidResults('Invalid results history')
    return value


def toc_enabled(data, client):
    try:
        if len(data) > 64 * 1024:
            return False
        lines = [line.strip().lstrip('\ufeff').replace('\\', '/') for line in data.decode('utf-8').splitlines()]
    except UnicodeDecodeError:
        return False
    clients = [v for v in lines if v.startswith('## X-Lamda-SimResults-Client:')]
    return ('## X-LamdaFrames-Managed: 1' in lines and '## X-Lamda-SimResults: 1' in lines
            and clients == ['## X-Lamda-SimResults-Client: ' + client]
            and lines.count('SimResults.lua') == 1 and 'Modules/Sim.lua' in lines
            and lines.index('SimResults.lua') < lines.index('Modules/Sim.lua'))


def _read(path, limit):
    if path.is_symlink() or any(p.is_symlink() for p in path.parents):
        raise RuntimeError('Refusing linked sim results')
    with path.open('rb') as stream:
        data = stream.read(limit + 1)
    if len(data) > limit:
        raise InvalidResults('Sim results file exceeds limit')
    return data


def kbb_enabled(data, client):
    try:
        if len(data) > 64 * 1024:
            return False
        lines = [line.strip().lstrip('\ufeff').replace('\\', '/') for line in data.decode('utf-8').splitlines()]
    except UnicodeDecodeError:
        return False
    clients = [v for v in lines if v.startswith('## X-Lamda-KBB-Client:')]
    return ('## X-LamdaFrames-Managed: 1' in lines and '## X-Lamda-KBB: 1' in lines
            and clients == ['## X-Lamda-KBB-Client: ' + client]
            and lines.count('KBBData.lua') == 1 and 'Modules/KBBSync.lua' in lines
            and lines.index('KBBData.lua') < lines.index('Modules/KBBSync.lua'))


def validate_kbb(data, client):
    if client not in ('retail', 'forever') or not isinstance(data, bytes) or len(data) > KBB_MAX_BYTES:
        raise InvalidResults('Invalid planner size or client')
    if data == KBB_SEED:
        return {}
    parts = data.split(b'\n', 2)
    if len(parts) != 3 or parts[0] + b'\n' != KBB_MARKER:
        raise InvalidResults('Planner marker missing')
    if parts[1] != b'-- Payload digest: ' + hashlib.sha256(parts[2]).hexdigest().encode():
        raise InvalidResults('Planner checksum mismatch')
    reader = _LiteralReader(parts[2], node_limit=2000000)
    reader.take(b'LamdaKBBData = ')
    value = reader.value(); reader.whitespace()
    if reader.pos != len(reader.data):
        raise InvalidResults('Unexpected executable planner content')
    if not isinstance(value, dict) or set(value) != {'schema', 'client', 'characters'} or type(value['schema']) is not int or value['schema'] != 1 or value['client'] != client:
        raise InvalidResults('Invalid planner envelope')
    chars = value['characters']
    if not isinstance(chars, dict) or len(chars) > 100:
        raise InvalidResults('Invalid planner characters')
    for guid, snapshot in chars.items():
        if not re.fullmatch(r'Player-[A-Za-z0-9-]{1,100}', guid) or not isinstance(snapshot, dict) or set(snapshot) != {'schema','client','guid','name','realm','records'} or type(snapshot['schema']) is not int or snapshot['schema'] != 1 or snapshot['client'] != client or snapshot['guid'] != guid:
            raise InvalidResults('Invalid planner character')
        records = snapshot['records'] or []
        if not isinstance(records, list) or len(records) > 21000:
            raise InvalidResults('Invalid planner record count')
        seen = set()
        for record in records:
            if not isinstance(record, dict) or not {'key','stamp'} <= set(record) or set(record) - {'key','stamp','data','deleted'}:
                raise InvalidResults('Invalid planner record')
            key = record['key']; stamp = record['stamp']
            if not isinstance(key, str) or not re.fullmatch(r'(?:b/[A-Za-z0-9_-]{1,64}|c/[A-Za-z0-9_-]{1,64}/[A-Za-z0-9_-]{1,64})', key) or key in seen:
                raise InvalidResults('Invalid planner key')
            seen.add(key)
            if not isinstance(stamp, dict) or set(stamp) != {'at','seq','actor'} or type(stamp['at']) is not int or not 0 <= stamp['at'] <= 4102444800000 or type(stamp['seq']) is not int or not 0 <= stamp['seq'] <= 2147483647 or stamp['actor'] not in ('game','desktop'):
                raise InvalidResults('Invalid planner timestamp')
            if record.get('deleted') is True:
                if 'data' in record:
                    raise InvalidResults('Deleted planner content')
            elif not isinstance(record.get('data'), dict):
                raise InvalidResults('Missing planner content')
    return value


def overlays_enabled(data, client):
    if client not in ('retail','forever') or len(data)>65536:return False
    try:lines=[v.strip().replace('\\','/') for v in data.decode().splitlines()]
    except UnicodeDecodeError:return False
    return ('## X-LamdaFrames-Managed: 1' in lines and '## X-Lamda-Overlays: 1' in lines
        and '## X-Lamda-Overlays-Client: '+client in lines
        and lines.count('OverlaysData.lua')==1 and 'Modules/Overlays.lua' in lines
        and lines.index('OverlaysData.lua')<lines.index('Modules/Overlays.lua'))


OVERLAY_UNIT_PATTERN=r'(player|pet|target|focus|party[1-4]|raid([1-9]|[1-3][0-9]|40)|boss[1-8]|arena[1-5]|nameplate([1-9]|[1-3][0-9]|40))'
def _overlay_unit(unit):
    return isinstance(unit,str) and re.fullmatch(OVERLAY_UNIT_PATTERN,unit) is not None

def validate_overlays(data, client):
    if client not in ('retail','forever') or not isinstance(data,bytes) or len(data)>OVERLAYS_MAX_BYTES or not data.startswith(OVERLAYS_MARKER):raise InvalidResults('Invalid Overlays data')
    reader=_LiteralReader(data[len(OVERLAYS_MARKER):],node_limit=4096)
    reader.take(b'LamdaOverlaysData = ');value=reader.value();reader.whitespace()
    if reader.pos!=len(reader.data) or not isinstance(value,dict) or not {'hash','watches'}<=set(value) or not set(value)<={'hash','watches','cooldownHash','cooldowns','anchors','units','unitTokens','unitHash','measurements','measurementHash','casts','castHash','combat','identities','identityHash','encounters','bosses','talents','talentHash','equipment','equipmentSets','talentChanges','equipmentChanges'} or ('cooldowns' in value)!=('cooldownHash' in value) or ('measurements' in value)!=('measurementHash' in value) or ('casts' in value)!=('castHash' in value) or ('talents' in value)!=('talentHash' in value) or ('unitTokens' in value)!=('unitHash' in value):raise InvalidResults('Invalid Overlays envelope')
    if 'unitTokens' in value:
        tokens=value['unitTokens']
        if isinstance(tokens,dict) and not tokens:tokens=[]
        if not isinstance(tokens,list) or len(tokens)>98 or any(not _overlay_unit(u) or u in ('pet','target','focus') for u in tokens) or sorted(set(tokens))!=tokens:raise InvalidResults('Invalid clone units')
        if type(value.get('units')) is not bool or value['units']!=bool(tokens):raise InvalidResults('Invalid clone tracking flag')
        digest=hashlib.sha256(json.dumps(tokens,separators=(',',':')).encode()).hexdigest()[:16]
        if value['unitHash']!=digest:raise InvalidResults('Clone unit checksum mismatch')
    if any(k in value for k in ('combat','identities','identityHash')):
        if not all(k in value for k in ('combat','identities','identityHash')) or type(value['combat']) is not bool:raise InvalidResults('Invalid combat tracking flag')
        identities=value['identities']
        if isinstance(identities,dict) and not identities:identities=[]
        if not isinstance(identities,list) or len(identities)>8 or any(not _overlay_unit(u) for u in identities) or len(set(identities))!=len(identities):raise InvalidResults('Invalid combat units')
        if value['combat'] != bool(identities) or value['combat'] and 'player' not in identities:raise InvalidResults('Invalid combat player identity')
        digest=hashlib.sha256(json.dumps(identities,separators=(',',':')).encode()).hexdigest()[:16]
        if value['identityHash']!=digest:raise InvalidResults('Combat watch checksum mismatch')
    casts=value.get('casts') or []
    if not isinstance(casts,list) or len(casts)>45 or any(not _overlay_unit(u) for u in casts) or len(set(casts))!=len(casts):raise InvalidResults('Invalid cast units')
    if not isinstance(value.get('cooldowns') or [],list):raise InvalidResults('Invalid cooldown watches')
    if len(value.get('cooldowns') or [])+len(casts)*2>90:raise InvalidResults('Too many timer sources')
    if 'casts' in value:
        digest=hashlib.sha256(json.dumps(casts,separators=(',',':')).encode()).hexdigest()[:16]
        if value['castHash']!=digest:raise InvalidResults('Cast watch checksum mismatch')
    if 'bosses' in value and type(value['bosses']) is not bool:raise InvalidResults('Invalid boss tracking flag')
    if 'encounters' in value and type(value['encounters']) is not bool:raise InvalidResults('Invalid encounter tracking flag')
    if 'units' in value and type(value['units']) is not bool:raise InvalidResults('Invalid unit tracking flag')
    if 'anchors' in value:
        anchors=value['anchors'] or []
        if not isinstance(anchors,list) or len(anchors)>64:raise InvalidResults('Invalid frame attachments')
        seen=set()
        for a in anchors:
            if not isinstance(a,dict) or set(a)!={'kind','target'} or a['kind'] not in ('frame','unit','nameplate') or not isinstance(a['target'],str):raise InvalidResults('Invalid frame attachment')
            pattern=r'[A-Za-z_][A-Za-z0-9_]{0,79}' if a['kind']=='frame' else r'(player|pet|target|focus|party[1-4]|raid([1-9]|[1-3][0-9]|40)|boss[1-8]|arena[1-5]|nameplate([1-9]|[1-3][0-9]|40))'
            if not re.fullmatch(pattern,a['target']) or (a['kind'],a['target']) in seen:raise InvalidResults('Invalid or duplicate frame attachment')
            seen.add((a['kind'],a['target']))
    watches=value['watches'] or []
    if not isinstance(watches,list) or len(watches)>64:raise InvalidResults('Invalid aura watches')
    canonical=[];seen=set()
    for w in watches:
        if not isinstance(w,dict) or set(w)!={'spell','kind','unit'} or type(w['spell']) is not int or not 1<=w['spell']<10000000 or w['kind'] not in ('buff','debuff') or not _overlay_unit(w['unit']):raise InvalidResults('Invalid aura watch')
        key=(w['spell'],w['kind'],w['unit'])
        if key in seen:raise InvalidResults('Duplicate aura watch')
        seen.add(key);canonical.append({k:w[k] for k in ('spell','kind','unit')})
    digest=hashlib.sha256(json.dumps(canonical,separators=(',',':')).encode()).hexdigest()[:16]
    if value['hash']!=digest:raise InvalidResults('Aura watch checksum mismatch')
    if 'measurements' in value:
        readings=value['measurements'] or []
        if not isinstance(readings,list) or len(readings)>64:raise InvalidResults('Invalid unit measurements')
        canonical=[];seen=set()
        for w in readings:
            if not isinstance(w,dict) or set(w)!={'unit','kind','resource'} or not _overlay_unit(w['unit']) or w['unit']=='player' or w['kind'] not in ('health','resource') or not isinstance(w['resource'],str):raise InvalidResults('Invalid unit measurement')
            if w['kind']=='health':
                if w['resource']!='':raise InvalidResults('Invalid health resource')
            elif w['resource']!='primary' and not re.fullmatch(r'(0|[1-9]|[12][0-9]|3[01])',w['resource']):raise InvalidResults('Invalid power type')
            key=(w['unit'],w['kind'],w['resource'])
            if key in seen:raise InvalidResults('Duplicate unit measurement')
            seen.add(key);canonical.append({k:w[k] for k in ('unit','kind','resource')})
        digest=hashlib.sha256(json.dumps(canonical,separators=(',',':')).encode()).hexdigest()[:16]
        if value['measurementHash']!=digest:raise InvalidResults('Unit measurement checksum mismatch')
    for key in ('talentChanges','equipmentChanges'):
        if key in value and type(value[key]) is not bool:raise InvalidResults('Invalid loadout change flag')
    if value.get('talentChanges') and 'talents' not in value:raise InvalidResults('Talent changes need talent capture')
    if value.get('equipmentChanges') and not value.get('equipment'):raise InvalidResults('Equipment changes need equipment capture')
    if 'equipment' in value or 'equipmentSets' in value:
        if type(value.get('equipment')) is not bool or 'equipmentSets' not in value:raise InvalidResults('Invalid equipment tracking flag')
        sets=value['equipmentSets'] or []
        if not isinstance(sets,list) or len(sets)>8 or sets and not value['equipment']:raise InvalidResults('Invalid equipment set watches')
        seen=set()
        for w in sets:
            if not isinstance(w,dict) or set(w)!={'key','name'} or not isinstance(w['name'],str):raise InvalidResults('Invalid equipment set watch')
            name=w['name']
            if not name.strip() or len(name.encode())>120 or any(c in name for c in ('\r','\n','\x00')):raise InvalidResults('Invalid equipment set name')
            key=hashlib.sha256(name.encode()).hexdigest()[:16]
            if w['key']!=key or key in seen:raise InvalidResults('Invalid equipment set key')
            seen.add(key)
    if 'talents' in value:
        talents=value['talents'] or []
        if not isinstance(talents,list) or len(talents)>16:raise InvalidResults('Invalid talent watches')
        canonical=[];seen=set()
        for w in talents:
            if not isinstance(w,dict) or set(w)!={'kind','id'} or type(w['id']) is not int or w['kind'] not in ('spell','entry','pvp','hero','known'):raise InvalidResults('Invalid talent watch')
            if not 1<=w['id']<10000000:raise InvalidResults('Invalid talent ID')
            key=(w['kind'],w['id'])
            if key in seen:raise InvalidResults('Duplicate talent watch')
            seen.add(key);canonical.append({k:w[k] for k in ('kind','id')})
        digest=hashlib.sha256(json.dumps(canonical,separators=(',',':')).encode()).hexdigest()[:16]
        if value['talentHash']!=digest:raise InvalidResults('Talent watch checksum mismatch')
    if 'cooldowns' in value:
        cooldowns=value['cooldowns'] or []
        if not isinstance(cooldowns,list) or len(cooldowns)>90:raise InvalidResults('Invalid cooldown watches')
        canonical=[];seen=set()
        for w in cooldowns:
            if not isinstance(w,dict) or set(w)!={'kind','id'} or type(w['id']) is not int or w['kind'] not in ('spell','item','slot','gcd'):raise InvalidResults('Invalid cooldown watch')
            if not 1<=w['id']<10000000 or (w['kind']=='slot' and w['id']>19) or (w['kind']=='gcd' and w['id']!=61304):raise InvalidResults('Invalid cooldown ID')
            key=(w['kind'],w['id'])
            if key in seen:raise InvalidResults('Duplicate cooldown watch')
            seen.add(key);canonical.append({k:w[k] for k in ('kind','id')})
        digest=hashlib.sha256(json.dumps(canonical,separators=(',',':')).encode()).hexdigest()[:16]
        if value['cooldownHash']!=digest:raise InvalidResults('Cooldown watch checksum mismatch')
    return value


def preserved_runtime(addons, package, client):
    """Preserve only validated, explicitly opted-in data over known package seeds."""
    preserved = {}
    for relative, seed, limit, enabled, validate in (
            (RELATIVE, SEED, MAX_BYTES, toc_enabled, validate_results),
            (KBB_RELATIVE, KBB_SEED, KBB_MAX_BYTES, kbb_enabled, validate_kbb),
            (OVERLAYS_RELATIVE, OVERLAYS_SEED, OVERLAYS_MAX_BYTES, overlays_enabled, validate_overlays)):
        try:
            if not all(enabled(_read(folder / 'lamdaUI/lamdaUI.toc', 64 * 1024), client) for folder in (addons, package)):
                continue
            if _read(package / relative, limit) != seed:
                continue
            data = _read(addons / relative, limit)
            validate(data, client)
            preserved[relative] = data
        except (FileNotFoundError, InvalidResults):
            continue
    return preserved


def installed_tree(folder, tree, runtime_enabled=False):
    """Ignore only the writer's unpublished atomic temp files, not addon code."""
    if not runtime_enabled:
        return tree(folder)
    result = {}
    if folder.is_symlink():
        raise RuntimeError('Refusing linked installation folder')
    for file in sorted(folder.rglob('*')):
        if file.is_symlink():
            raise RuntimeError('Refusing linked installation contents')
        if file.parent == folder and TEMP_NAME.fullmatch(file.name):
            continue
        if file.is_file():
            result[file.relative_to(folder).as_posix()] = hashlib.sha256(file.read_bytes()).hexdigest()
    return result
