"""音乐生成任务端到端验证驱动。

用法（先启动 scripts/e2e/mockmusic）：

    python scripts/e2e/music_e2e.py run       # 打补丁 -> 成功链路 -> 两个失败分支
    python scripts/e2e/music_e2e.py restore   # 还原 settings / 算力点 / 删除测试数据

设计要点：
* 所有改动都先备份（.tmp-probe/e2e-backup.json），restore 按字段还原，不做整行覆盖。
* 只使用 admin 账号（避免新建用户），算力点从 0 临时调到 100，结束后还原。
* 测试期间把全局对象存储临时指向本地 mock S3，避免往真实的坚果云 WebDAV 写测试文件。
"""

import json
import os
import sqlite3
import sys
import time
import urllib.error
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
DB_PATH = os.path.join(ROOT, 'data', 'infinite-canvas.db')
BACKUP_PATH = os.path.join(ROOT, '.tmp-probe', 'e2e-backup.json')

BASE = 'http://127.0.0.1:8080'
MUSIC_URL = 'http://127.0.0.1:18081'
S3_URL = 'http://127.0.0.1:18082'
ADMIN_USERNAME = 'admin'
ADMIN_PASSWORD = 'infinite-canvas'

TEST_MODEL = 'mock-doubao-music'
TEST_CHANNEL_ID = 'channel-mockmusic0001'
TEST_WRONG_PROTOCOL_ID = 'channel-mockmusic0002'
TEST_STORAGE_ID = 'storage-mocks3000001'
TEST_CREDITS = 2.0
GRANTED_CREDITS = 100.0

created_task_ids = []


def db():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn


def request(method, path, body=None, token=None, raw=False):
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode()
        headers['Content-Type'] = 'application/json'
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req) as resp:
            payload = resp.read()
            status = resp.status
            resp_headers = dict(resp.headers)
    except urllib.error.HTTPError as err:
        payload = err.read()
        status = err.code
        resp_headers = dict(err.headers)
    if raw:
        return status, payload, resp_headers
    try:
        return status, json.loads(payload.decode() or '{}'), resp_headers
    except Exception:
        return status, {'_raw': payload.decode(errors='replace')}, resp_headers


def log(*parts):
    print(*parts, flush=True)


# ---------------------------------------------------------------- 备份 / 打补丁

def read_settings():
    conn = db()
    rows = {r['key']: json.loads(r['value']) for r in conn.execute('select key, value from settings')}
    conn.close()
    return rows


def write_settings(rows):
    conn = db()
    for key, value in rows.items():
        conn.execute('update settings set value = ? where key = ?', (json.dumps(value, ensure_ascii=False), key))
    conn.commit()
    conn.close()


def patch():
    if os.path.exists(BACKUP_PATH):
        raise SystemExit('备份已存在 %s，先执行 restore 再重新 patch' % BACKUP_PATH)
    settings = read_settings()
    baseline = {
        'settings': settings,
        'user_credits': {},
        'max_credit_log_rowid': None,
        'max_storage_object_rowid': None,
        'music_task_count': None,
    }
    conn = db()
    for row in conn.execute('select id, username, credits from users'):
        baseline['user_credits'][row['username']] = {'id': row['id'], 'credits': row['credits']}
    baseline['max_credit_log_rowid'] = conn.execute('select coalesce(max(rowid), 0) from credit_logs').fetchone()[0]
    baseline['max_storage_object_rowid'] = conn.execute('select coalesce(max(rowid), 0) from storage_objects').fetchone()[0]
    baseline['music_task_ids'] = [r['id'] for r in conn.execute('select id from music_tasks')]
    conn.close()
    os.makedirs(os.path.dirname(BACKUP_PATH), exist_ok=True)
    with open(BACKUP_PATH, 'w', encoding='utf-8') as handle:
        json.dump(baseline, handle, ensure_ascii=False, indent=2)

    public = settings['public']
    private = settings['private']

    public['modelChannel']['availableModels'] = list(dict.fromkeys(
        public['modelChannel'].get('availableModels', []) + [TEST_MODEL]))
    public['modelChannel']['modelCosts'] = [{'model': TEST_MODEL, 'credits': TEST_CREDITS}]

    private['channels'] = [c for c in private.get('channels', []) if c.get('id') != TEST_CHANNEL_ID] + [{
        'id': TEST_CHANNEL_ID,
        'protocol': 'volc-music',
        'name': 'e2e-mock-music',
        'baseUrl': MUSIC_URL,
        'apiKey': 'mock-music-key',
        'models': [TEST_MODEL],
        'weight': 1,
        'timeout': 60,
        'enabled': True,
        'remark': '本地 mock 上游（端到端验证临时渠道）',
        'modelCapabilities': {TEST_MODEL: 'music'},
    }]
    private['storage']['providers'] = [{
        'id': TEST_STORAGE_ID,
        'name': 'e2e-mock-s3',
        'type': 's3',
        'endpoint': S3_URL,
        'region': 'auto',
        'bucket': 'mock-bucket',
        'accessKeyId': 'mock-ak',
        'secretAccessKey': 'mock-sk',
        'publicBaseUrl': '',
        'pathPrefix': '',
        'weight': 1,
        'enabled': True,
        'ownerUserId': '',
    }]
    write_settings({'public': public, 'private': private})
    log('[patch] 已写入临时音乐渠道 %s -> %s，模型 %s，成本 %s 算力点' % (TEST_CHANNEL_ID, MUSIC_URL, TEST_MODEL, TEST_CREDITS))
    log('[patch] 全局对象存储临时改为 mock S3 %s（原 webdav 已备份）' % S3_URL)


def restore():
    """还原 settings 字段、算力点与所有测试数据（含对象存储上的远程文件）。"""
    with open(BACKUP_PATH, encoding='utf-8') as handle:
        baseline = json.load(handle)
    saved = baseline['settings']

    # 1) 先把测试任务的音频对象从（真实）对象存储上删掉——必须在还原存储配置之前做。
    token, _ = admin_login()
    conn = db()
    tasks = [dict(r) for r in conn.execute('select id, storage_key from music_tasks')]
    conn.close()
    for task in tasks:
        object_id = (task['storage_key'] or '').split(':', 1)[-1]
        if object_id:
            code, payload, _ = request('DELETE', '/api/v1/files/%s' % object_id, token=token)
            log('[restore] 删除远程音频对象 %s -> %s' % (object_id, json.dumps(payload, ensure_ascii=False)))

    # 2) 只回退本次测试改动的字段，其余字段保留当前值（避免覆盖其他人的配置）。
    current = read_settings()
    mine = {TEST_CHANNEL_ID, TEST_WRONG_PROTOCOL_ID}
    channels = [c for c in current['private']['channels'] if c.get('id') not in mine]
    for original in saved['private']['channels']:
        if not any(c.get('id') == original.get('id') for c in channels):
            channels.append(original)
    current['private']['channels'] = channels
    current['private']['storage']['providers'] = saved['private']['storage']['providers']
    models = [m for m in current['public']['modelChannel']['availableModels'] if m != TEST_MODEL]
    for original in saved['public']['modelChannel']['availableModels']:
        if original not in models:
            models.append(original)
    current['public']['modelChannel']['availableModels'] = models
    costs = [c for c in current['public']['modelChannel']['modelCosts'] if c.get('model') != TEST_MODEL]
    for original in saved['public']['modelChannel']['modelCosts']:
        if not any(c.get('model') == original.get('model') for c in costs):
            costs.append(original)
    current['public']['modelChannel']['modelCosts'] = costs
    write_settings(current)

    conn = db()
    conn.execute('delete from music_tasks')
    conn.execute('delete from storage_objects where rowid > ?', (baseline['max_storage_object_rowid'],))
    conn.execute('delete from credit_logs where rowid > ?', (baseline['max_credit_log_rowid'],))
    for name, info in baseline['user_credits'].items():
        conn.execute('update users set credits = ? where id = ?', (info['credits'], info['id']))
    conn.commit()
    remaining = conn.execute('select count(*) from music_tasks').fetchone()[0]
    remaining_objects = conn.execute('select count(*) from storage_objects').fetchone()[0]
    remaining_logs = conn.execute('select count(*) from credit_logs').fetchone()[0]
    conn.close()
    log('[restore] settings 已还原；music_tasks=%d storage_objects=%d credit_logs=%d' % (remaining, remaining_objects, remaining_logs))
    return remaining, remaining_objects, remaining_logs


# ---------------------------------------------------------------- 接口封装

def admin_login():
    status, payload, _ = request('POST', '/api/admin/login', {'username': ADMIN_USERNAME, 'password': ADMIN_PASSWORD})
    if payload.get('code') != 0:
        raise SystemExit('管理员登录失败: %s' % payload)
    return payload['data']['token'], payload['data']['user']


def set_credits(token, user_id, credits):
    status, payload, _ = request('POST', '/api/admin/users/%s/credits' % user_id, {'credits': credits}, token)
    if payload.get('code') != 0:
        raise SystemExit('调整算力点失败: %s' % payload)
    return payload['data']['credits']


def me(token):
    status, payload, _ = request('GET', '/api/auth/me', token=token)
    return payload.get('data', {})


def create_task(token, prompt, lyrics='', mode='song'):
    body = {
        'mode': mode, 'title': 'e2e 音乐任务', 'prompt': prompt, 'lyrics': lyrics,
        'styleTags': ['电子', '轻快'], 'duration': 60, 'language': 'chinese',
        'vocalGender': 'female', 'keyMode': 'major', 'tempo': 120,
        'model': TEST_MODEL, 'channelId': TEST_CHANNEL_ID,
    }
    status, payload, _ = request('POST', '/api/v1/music-tasks', body, token)
    return status, payload


def get_task(token, task_id):
    return request('GET', '/api/v1/music-tasks/%s' % task_id, token=token)


def poll_until_done(token, task_id, timeout=150):
    started = time.time()
    polls = []
    while time.time() - started < timeout:
        time.sleep(2)
        status, payload, _ = get_task(token, task_id)
        data = payload.get('data', {})
        polls.append({
            't': round(time.time() - started, 1),
            'status': data.get('status'),
            'progress': data.get('progress'),
            'task_id': data.get('task_id'),
            'audio_url': data.get('audio_url'),
        })
        log('   轮询 #%d t=%.1fs status=%s progress=%s task_id=%s' % (
            len(polls), polls[-1]['t'], data.get('status'), data.get('progress'), data.get('task_id')))
        if data.get('status') in ('completed', 'failed'):
            return data, polls, round(time.time() - started, 1)
    return None, polls, round(time.time() - started, 1)


# ---------------------------------------------------------------- 用例

def run():
    token, user = admin_login()
    log('[auth] 管理员登录成功 user=%s id=%s 当前算力点=%s' % (user['username'], user['id'], user['credits']))
    log('[auth] 授予临时算力点 %s' % set_credits(token, user['id'], GRANTED_CREDITS))

    results = {}

    # ---- 用例 1：成功链路
    log('\n=== 用例 1：创建 -> 轮询 -> completed -> 转存自有存储 ===')
    status, payload = create_task(token, '一段轻快的电子音乐，用于端到端验证', lyrics='[verse]\nmock 歌词')
    log('[create] HTTP %s %s' % (status, json.dumps(payload, ensure_ascii=False)))
    if payload.get('code') != 0:
        raise SystemExit('创建音乐任务失败')
    task = payload['data']
    task_id = task['id']
    created_task_ids.append(task_id)
    log('[create] id=%s status=%s credits=%s task_id=%s audio_url=%r' % (
        task_id, task['status'], task.get('credits'), task.get('task_id'), task.get('audio_url')))

    final, polls, elapsed = poll_until_done(token, task_id)
    log('[final] %s' % json.dumps({k: final.get(k) for k in (
        'id', 'status', 'progress', 'audio_url', 'storageKey', 'mime_type', 'bytes', 'duration_ms',
        'lyrics', 'task_id', 'upstream_model', 'error')}, ensure_ascii=False) if final else '超时未结束')
    results['success'] = {'final': final, 'polls': polls, 'elapsed': elapsed, 'create': task}

    # ---- 算力点：完成后不应退还
    after = me(token)
    log('[credits] 完成后余额=%s（授予 %s - 成本 %s = %s 为预期）' % (
        after.get('credits'), GRANTED_CREDITS, TEST_CREDITS, GRANTED_CREDITS - TEST_CREDITS))
    results['credits_after_success'] = after.get('credits')

    # ---- 下载自证：audio_url 指向自有存储
    if final and final.get('audio_url'):
        status_code, body, headers = request('GET', final['audio_url'], raw=True)
        log('[download] GET %s -> HTTP %s bytes=%d content-type=%s' % (
            final['audio_url'], status_code, len(body), headers.get('Content-Type')))
        results['download'] = {'status': status_code, 'bytes': len(body), 'content_type': headers.get('Content-Type')}

    # ---- 用例 2：上游 QuerySong 返回 Status 3
    log('\n=== 用例 2：QuerySong 返回 Status 3（上游已接手后失败） ===')
    status, payload = create_task(token, '这段会被 mock 判为失败 __FAIL_QUERY__')
    if payload.get('code') != 0:
        log('[create] 失败 %s' % json.dumps(payload, ensure_ascii=False))
    else:
        task = payload['data']
        created_task_ids.append(task['id'])
        log('[create] id=%s status=%s credits=%s' % (task['id'], task['status'], task.get('credits')))
        final2, polls2, elapsed2 = poll_until_done(token, task['id'])
        log('[final] %s' % json.dumps({k: final2.get(k) for k in ('id', 'status', 'error', 'error_detail', 'task_id')}, ensure_ascii=False) if final2 else '超时')
        balance = me(token).get('credits')
        log('[credits] 失败后余额=%s' % balance)
        results['fail_query'] = {'final': final2, 'polls': polls2, 'elapsed': elapsed2, 'credits': balance}

    # ---- 用例 3：提交阶段就失败
    log('\n=== 用例 3：GenSongForTime 返回 400（上游未接手） ===')
    status, payload = create_task(token, '这段会让上游拒绝创建任务 __FAIL_SUBMIT__')
    if payload.get('code') != 0:
        log('[create] 失败 %s' % json.dumps(payload, ensure_ascii=False))
    else:
        task = payload['data']
        created_task_ids.append(task['id'])
        log('[create] id=%s status=%s credits=%s' % (task['id'], task['status'], task.get('credits')))
        final3, polls3, elapsed3 = poll_until_done(token, task['id'])
        log('[final] %s' % json.dumps({k: final3.get(k) for k in ('id', 'status', 'error', 'error_detail', 'task_id')}, ensure_ascii=False) if final3 else '超时')
        balance = me(token).get('credits')
        log('[credits] 失败后余额=%s' % balance)
        results['fail_submit'] = {'final': final3, 'polls': polls3, 'elapsed': elapsed3, 'credits': balance}

    # ---- 歌词接口
    log('\n=== 用例 4：歌词接口 ===')
    status, payload, _ = request('POST', '/api/v1/music-tasks/lyrics', {
        'model': TEST_MODEL, 'channelId': TEST_CHANNEL_ID, 'prompt': '关于夏天的歌',
    }, token)
    log('[lyrics] HTTP %s %s' % (status, json.dumps(payload, ensure_ascii=False)))
    results['lyrics'] = payload

    with open(os.path.join(ROOT, '.tmp-probe', 'e2e-results.json'), 'w', encoding='utf-8') as handle:
        json.dump(results, handle, ensure_ascii=False, indent=2)
    log('\n[结果] 已写入 .tmp-probe/e2e-results.json')
    return results


def delete_task(token, task_id):
    return request('DELETE', '/api/v1/music-tasks/%s' % task_id, token=token)


def restore_storage_only():
    """把对象存储还原成项目原有配置，保留音乐 mock 渠道（用于验证真实存储的转存）。"""
    with open(BACKUP_PATH, encoding='utf-8') as handle:
        baseline = json.load(handle)
    current = read_settings()
    current['private']['storage']['providers'] = baseline['settings']['private']['storage']['providers']
    write_settings(current)
    provider = current['private']['storage']['providers'][0]
    log('[storage] 已还原为项目原有存储: %s (%s %s)' % (provider.get('name'), provider.get('type'), provider.get('endpoint')))


def success_only():
    """只跑成功链路：创建 -> 轮询 -> completed -> 转存 -> 自证下载 -> 删除对象。"""
    token, user = admin_login()
    log('[auth] 管理员登录 user=%s 算力点=%s' % (user['username'], user['credits']))
    log('[auth] 授予临时算力点 %s -> %s' % (GRANTED_CREDITS, set_credits(token, user['id'], GRANTED_CREDITS)))
    before = me(token).get('credits')

    status, payload = create_task(token, '一段轻快的电子音乐，用于端到端验证')
    log('[create] HTTP %s %s' % (status, json.dumps(payload, ensure_ascii=False)))
    if payload.get('code') != 0:
        raise SystemExit('创建音乐任务失败')
    task = payload['data']
    created_task_ids.append(task['id'])
    log('[create] id=%s status=%s credits=%s audio_url=%r' % (task['id'], task['status'], task.get('credits'), task.get('audio_url')))

    final, polls, elapsed = poll_until_done(token, task['id'])
    log('[final] %s' % json.dumps({k: final.get(k) for k in (
        'id', 'status', 'progress', 'audio_url', 'storageKey', 'mime_type', 'bytes', 'duration_ms',
        'lyrics', 'task_id', 'upstream_model')}, ensure_ascii=False) if final else '超时未结束')

    checks = {}
    if final:
        audio_url = final.get('audio_url') or ''
        checks['status_completed'] = final.get('status') == 'completed'
        checks['audio_url_is_own_storage'] = audio_url.startswith('/api/files/') and '/content?s=' in audio_url
        checks['audio_url_not_mock'] = '127.0.0.1:18081' not in audio_url
        checks['storage_key'] = final.get('storageKey') or ''
        checks['mime_type'] = final.get('mime_type') or ''
        checks['bytes'] = final.get('bytes') or 0
        checks['duration_ms'] = final.get('duration_ms') or 0
        checks['fields_non_empty'] = bool(checks['storage_key'] and checks['mime_type'] and checks['bytes'] and checks['duration_ms'])
        if checks['audio_url_is_own_storage']:
            code, body, headers = request('GET', audio_url, raw=True)
            log('[download] GET %s -> HTTP %s bytes=%d content-type=%s' % (audio_url, code, len(body), headers.get('Content-Type')))
            checks['download_status'] = code
            checks['download_bytes'] = len(body)
        # 用完就删掉真实存储上的对象，避免留下测试文件。
        object_id = checks['storage_key'].split(':', 1)[-1] if checks['storage_key'] else ''
        if object_id:
            code, payload, _ = request('DELETE', '/api/v1/files/%s' % object_id, token=token)
            log('[cleanup] DELETE /api/v1/files/%s -> %s' % (object_id, json.dumps(payload, ensure_ascii=False)))
            checks['object_delete'] = payload.get('code')
        code, payload, _ = delete_task(token, task['id'])
        log('[cleanup] DELETE /api/v1/music-tasks/%s -> %s' % (task['id'], json.dumps(payload, ensure_ascii=False)))

    after = me(token).get('credits')
    log('[credits] 创建前=%s 完成后=%s（期望 %s，成功后不退费）' % (before, after, before - TEST_CREDITS))
    checks['credits_before'] = before
    checks['credits_after'] = after
    log('[断言] %s' % json.dumps(checks, ensure_ascii=False))
    with open(os.path.join(ROOT, '.tmp-probe', 'e2e-success.json'), 'w', encoding='utf-8') as handle:
        json.dump({'final': final, 'polls': polls, 'elapsed': elapsed, 'checks': checks}, handle, ensure_ascii=False, indent=2)
    return checks


def credit_logs_after(rowid):
    conn = db()
    rows = [dict(r) for r in conn.execute(
        'select id, type, amount, balance, remark, created_at from credit_logs where rowid > ? order by rowid', (rowid,))]
    conn.close()
    return rows


def failures():
    """两条失败分支：QuerySong Status 3（上游已接手）与 GenSongForTime 400（上游未接手）。"""
    with open(BACKUP_PATH, encoding='utf-8') as handle:
        baseline = json.load(handle)
    token, user = admin_login()
    mark = baseline['max_credit_log_rowid']
    log('[auth] 管理员登录 user=%s 算力点=%s' % (user['username'], user['credits']))
    log('[auth] 授予临时算力点 %s -> %s' % (GRANTED_CREDITS, set_credits(token, user['id'], GRANTED_CREDITS)))

    out = {}
    for name, prompt, expect_refund in (
        ('query_status_3', '这段会被 mock 判为失败 __FAIL_QUERY__', False),
        ('submit_http_400', '这段会让上游拒绝创建任务 __FAIL_SUBMIT__', True),
    ):
        log('\n=== 失败分支 %s ===' % name)
        before = me(token).get('credits')
        status, payload = create_task(token, prompt)
        if payload.get('code') != 0:
            log('[create] 失败 %s' % json.dumps(payload, ensure_ascii=False))
            out[name] = {'create': payload}
            continue
        task = payload['data']
        created_task_ids.append(task['id'])
        log('[create] id=%s status=%s credits=%s 余额=%s' % (task['id'], task['status'], task.get('credits'), me(token).get('credits')))
        final, polls, elapsed = poll_until_done(token, task['id'])
        balance = me(token).get('credits')
        log('[final] %s' % json.dumps({k: final.get(k) for k in ('id', 'status', 'error', 'error_detail', 'task_id', 'audio_url', 'storageKey')}, ensure_ascii=False) if final else '超时')
        log('[credits] 创建前=%s 失败后=%s（期望退还=%s）' % (before, balance, expect_refund))
        out[name] = {'before': before, 'after': balance, 'expect_refund': expect_refund, 'final': final, 'polls': polls, 'elapsed': elapsed}
        code, payload, _ = delete_task(token, task['id'])
        log('[cleanup] DELETE /api/v1/music-tasks/%s -> %s' % (task['id'], json.dumps(payload, ensure_ascii=False)))

    logs = credit_logs_after(mark)
    log('\n[credit_logs] 本次测试期间新增 %d 条：' % len(logs))
    for row in logs:
        log('   %s type=%s amount=%s balance=%s remark=%s' % (row['id'], row['type'], row['amount'], row['balance'], row['remark']))
    out['credit_logs'] = logs
    with open(os.path.join(ROOT, '.tmp-probe', 'e2e-failures.json'), 'w', encoding='utf-8') as handle:
        json.dump(out, handle, ensure_ascii=False, indent=2)
    return out


def extra():
    """补充验证：渠道 Protocol 路由（正确/错误协议）+ 纯音乐（instrumental/BGM）分支。"""
    with open(BACKUP_PATH, encoding='utf-8') as handle:
        baseline = json.load(handle)
    current = read_settings()
    public, private = current['public'], current['private']
    public['modelChannel']['availableModels'] = list(dict.fromkeys(
        public['modelChannel'].get('availableModels', []) + [TEST_MODEL]))
    public['modelChannel']['modelCosts'] = [{'model': TEST_MODEL, 'credits': TEST_CREDITS}]
    private['channels'] = [c for c in private['channels'] if c['id'] not in (TEST_CHANNEL_ID, TEST_WRONG_PROTOCOL_ID)] + [{
        'id': TEST_CHANNEL_ID, 'protocol': 'volc-music', 'name': 'e2e-mock-music', 'baseUrl': MUSIC_URL,
        'apiKey': 'mock-music-key', 'models': [TEST_MODEL], 'weight': 1, 'timeout': 60, 'enabled': True,
        'remark': '本地 mock 上游', 'modelCapabilities': {TEST_MODEL: 'music'},
    }, {
        'id': TEST_WRONG_PROTOCOL_ID, 'protocol': 'openai', 'name': 'e2e-错误协议', 'baseUrl': MUSIC_URL,
        'apiKey': 'mock-music-key', 'models': [TEST_MODEL], 'weight': 1, 'timeout': 60, 'enabled': True,
        'remark': '错误协议渠道，用于验证路由拦截', 'modelCapabilities': {TEST_MODEL: 'music'},
    }]
    write_settings(current)

    token, user = admin_login()
    log('[auth] 授予临时算力点 -> %s' % set_credits(token, user['id'], GRANTED_CREDITS))
    out = {}

    log('\n=== 渠道协议路由 ===')
    log('[缺少渠道] %s' % json.dumps(request('POST', '/api/v1/music-tasks', {
        'model': TEST_MODEL, 'prompt': 'x', 'lyrics': 'y'}, token)[1], ensure_ascii=False))
    log('[错误协议 openai] %s' % json.dumps(request('POST', '/api/v1/music-tasks', {
        'model': TEST_MODEL, 'channelId': TEST_WRONG_PROTOCOL_ID, 'prompt': 'x', 'lyrics': 'y'}, token)[1], ensure_ascii=False))

    log('\n=== 纯音乐分支（instrumental -> GenBGMForTime） ===')
    status, payload, _ = request('POST', '/api/v1/music-tasks', {
        'mode': 'instrumental', 'title': 'e2e 纯音乐', 'prompt': '舒缓的钢琴背景音乐', 'duration': 60,
        'model': TEST_MODEL, 'channelId': TEST_CHANNEL_ID}, token)
    log('[create] HTTP %s %s' % (status, json.dumps(payload, ensure_ascii=False)))
    if payload.get('code') == 0:
        task = payload['data']
        created_task_ids.append(task['id'])
        final, polls, elapsed = poll_until_done(token, task['id'])
        log('[final] %s' % json.dumps({k: final.get(k) for k in (
            'id', 'status', 'mode', 'audio_url', 'storageKey', 'mime_type', 'bytes', 'duration_ms', 'task_id')}, ensure_ascii=False) if final else '超时')
        out['bgm'] = final
        if final and final.get('storageKey'):
            object_id = final['storageKey'].split(':', 1)[-1]
            log('[cleanup] DELETE /api/v1/files/%s -> %s' % (object_id, json.dumps(request('DELETE', '/api/v1/files/%s' % object_id, token=token)[1], ensure_ascii=False)))
        log('[cleanup] DELETE task -> %s' % json.dumps(delete_task(token, task['id'])[1], ensure_ascii=False))
    with open(os.path.join(ROOT, '.tmp-probe', 'e2e-extra.json'), 'w', encoding='utf-8') as handle:
        json.dump(out, handle, ensure_ascii=False, indent=2)
    return out


if __name__ == '__main__':
    command = sys.argv[1] if len(sys.argv) > 1 else 'run'
    if command == 'patch':
        patch()
    elif command == 'restore':
        restore()
    elif command == 'run':
        patch()
        run()
    elif command == 'flow':
        run()
    elif command == 'success':
        success_only()
    elif command == 'storage-restore':
        restore_storage_only()
    elif command == 'failures':
        failures()
    elif command == 'extra':
        extra()
    else:
        raise SystemExit('未知命令: %s' % command)
