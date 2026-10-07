#!/usr/bin/env python
"""infinite-canvas 端到端 API 功能测试：账号隔离、管理员权限、对象下载签名、执行中任务。

用法：
    python scripts/e2e/api_tests.py

环境变量：
    IC_E2E_BASE        后端地址，默认 http://127.0.0.1:8080
    IC_E2E_ADMIN_USER  管理员用户名，默认 admin
    IC_E2E_ADMIN_PASS  管理员密码（必填，不写进仓库）
    IC_E2E_USER_USER   普通用户名，默认 guoguogis
    IC_E2E_USER_PASS   普通用户密码（必填，不写进仓库）
    FIX_ADMIN_VIDEO / FIX_USER_VIDEO / FIX_ADMIN_IMAGE / FIX_USER_IMAGE
                       由 `go run ./scripts/e2e/testctl -mode seed` 输出的夹具 id
"""
import json
import os
import re
import sys

import httpx


def credentials(prefix, default_user):
    username = os.environ.get(f"IC_E2E_{prefix}_USER", default_user)
    password = os.environ.get(f"IC_E2E_{prefix}_PASS", "")
    if not password:
        raise SystemExit(f"缺少环境变量 IC_E2E_{prefix}_PASS")
    return username, password


BASE = os.environ.get("IC_E2E_BASE", "http://127.0.0.1:8080")
ADMIN_CRED = credentials("ADMIN", "admin")
USER_CRED = credentials("USER", "guoguogis")

passed = 0
failures = []


def check(name, ok, detail=""):
    global passed
    if ok:
        passed += 1
        print(f"PASS  {name}")
    else:
        failures.append(f"{name}  [{detail}]")
        print(f"FAIL  {name}  [{detail}]")


client = httpx.Client(base_url=BASE, timeout=90.0)


def login(username, password):
    r = client.post("/api/auth/login", json={"username": username, "password": password})
    payload = r.json()
    if payload.get("code") != 0:
        raise SystemExit(f"登录失败 {username}: {payload}")
    return payload["data"]["token"], payload["data"]["user"]


def call(method, path, token=None, body=None):
    headers = {"Authorization": f"Bearer {token}"} if token else {}
    r = client.request(method, path, headers=headers, json=body)
    try:
        payload = r.json()
    except Exception:
        payload = None
    return r.status_code, payload, len(r.content)


def ids_of(payload):
    data = (payload or {}).get("data")
    if not isinstance(data, list):
        return []
    return [item.get("id") for item in data if isinstance(item, dict)]


def results_of(payload):
    data = (payload or {}).get("data")
    return data if isinstance(data, list) else []


print("\n=== 1. 登录 ===")
admin_token, admin_user = login(*ADMIN_CRED)
user_token, user_user = login(*USER_CRED)
check("admin 登录成功且角色为 admin", admin_user.get("role") == "admin", f"role={admin_user.get('role')}")
check("guoguogis 登录成功且角色为 user", user_user.get("role") == "user", f"role={user_user.get('role')}")
check("两个账号 id 不同", admin_user["id"] != user_user["id"], f"{admin_user['id']} / {user_user['id']}")

print("\n=== 2. 未登录访问用户侧接口应 401 ===")
for path in [
    "/api/v1/canvas/projects",
    "/api/v1/generation-logs/videos",
    "/api/v1/generation-logs/images",
    "/api/v1/video-tasks",
    "/api/v1/canvas/image-tasks",
    "/api/v1/user-config",
    "/api/v1/workflows",
    "/api/v1/comfy-bridges",
    "/api/v1/agent-skills",
]:
    status, _, _ = call("GET", path)
    check(f"无 token GET {path}", status == 401, f"status={status}")

print("\n=== 3. 用户侧数据按账号隔离 ===")
_, admin_projects, _ = call("GET", "/api/v1/canvas/projects", admin_token)
_, user_projects, _ = call("GET", "/api/v1/canvas/projects", user_token)
admin_project_ids = ids_of(admin_projects)
user_project_ids = ids_of(user_projects)
check("admin 能看到自己的画布项目", len(admin_project_ids) > 0, f"count={len(admin_project_ids)}")
check(
    "guoguogis 看不到 admin 的画布项目",
    not set(user_project_ids) & set(admin_project_ids),
    f"userIds={user_project_ids}",
)

_, admin_vlogs, _ = call("GET", "/api/v1/generation-logs/videos", admin_token)
_, user_vlogs, _ = call("GET", "/api/v1/generation-logs/videos", user_token)
check("admin 有视频生成记录", len(results_of(admin_vlogs)) > 0, f"count={len(results_of(admin_vlogs))}")
check("guoguogis 的视频生成记录为空（看不到 admin 的）", len(results_of(user_vlogs)) == 0, f"count={len(results_of(user_vlogs))}")

_, admin_ilogs, _ = call("GET", "/api/v1/generation-logs/images", admin_token)
_, user_ilogs, _ = call("GET", "/api/v1/generation-logs/images", user_token)
check("admin 有图片生成记录", len(results_of(admin_ilogs)) > 0, f"count={len(results_of(admin_ilogs))}")
check("guoguogis 的图片生成记录为空（看不到 admin 的）", len(results_of(user_ilogs)) == 0, f"count={len(results_of(user_ilogs))}")

_, admin_vtasks, _ = call("GET", "/api/v1/video-tasks", admin_token)
_, user_vtasks, _ = call("GET", "/api/v1/video-tasks", user_token)
check("admin 能看到自己的视频任务", len(ids_of(admin_vtasks)) > 0, f"count={len(ids_of(admin_vtasks))}")
check(
    "guoguogis 看不到 admin 的视频任务",
    not set(ids_of(user_vtasks)) & set(ids_of(admin_vtasks)),
    f"userIds={ids_of(user_vtasks)}",
)

_, admin_itasks, _ = call("GET", "/api/v1/canvas/image-tasks", admin_token)
_, user_itasks, _ = call("GET", "/api/v1/canvas/image-tasks", user_token)
check("admin 能看到自己的图片任务", len(ids_of(admin_itasks)) > 0, f"count={len(ids_of(admin_itasks))}")
check(
    "guoguogis 看不到 admin 的图片任务",
    not set(ids_of(user_itasks)) & set(ids_of(admin_itasks)),
    f"userIds={ids_of(user_itasks)}",
)

print("\n=== 4. 写入隔离：guoguogis 新建记录不影响 admin ===")
log_id = f"test-fixture-cardlog-{os.urandom(4).hex()}"
log_payload = {
    "id": log_id,
    "createdAt": 1735689600000,
    "status": "生成中",
    "prompt": "e2e 隔离测试记录",
    "model": "fixture-model",
    "task": {"id": f"test-fixture-cardtask-{log_id}", "status": "processing"},
}
status, saved, _ = call("POST", "/api/v1/generation-logs/videos", user_token, {"logs": [log_payload]})
check("guoguogis 写入生成记录成功", status == 200 and (saved or {}).get("code") == 0, f"status={status} resp={saved}")
check("写入后 guoguogis 能看到 1 条", len(results_of(saved)) == 1, f"count={len(results_of(saved))}")

_, admin_after, _ = call("GET", "/api/v1/generation-logs/videos", admin_token)
check(
    "admin 看不到 guoguogis 新写入的记录",
    log_id not in [item.get("id") for item in results_of(admin_after)],
    f"adminIds={[item.get('id') for item in results_of(admin_after)]}",
)

status, _, _ = call("DELETE", f"/api/v1/generation-logs/videos/{log_id}", admin_token)
_, still_there, _ = call("GET", "/api/v1/generation-logs/videos", user_token)
check(
    "admin 用同样的 id 调删除接口删不掉 guoguogis 的记录",
    status == 200 and log_id in [item.get("id") for item in results_of(still_there)],
    f"status={status} userIds={[item.get('id') for item in results_of(still_there)]}",
)

status, deleted, _ = call("DELETE", f"/api/v1/generation-logs/videos/{log_id}", user_token)
check("guoguogis 能删除自己的记录", status == 200 and (deleted or {}).get("code") == 0, f"status={status}")
_, after_del, _ = call("GET", "/api/v1/generation-logs/videos", user_token)
check("删除后 guoguogis 记录数归零", len(results_of(after_del)) == 0, f"count={len(results_of(after_del))}")

print("\n=== 5. 管理员接口权限 ===")
status, _, _ = call("GET", "/api/admin/tasks")
check("无 token 访问 /api/admin/tasks", status == 401, f"status={status}")
status, _, _ = call("GET", "/api/admin/tasks", user_token)
check("普通用户访问 /api/admin/tasks 被拒", status == 401, f"status={status}")
status, admin_tasks, _ = call("GET", "/api/admin/tasks", admin_token)
check(
    "admin 访问 /api/admin/tasks 成功且返回数组",
    status == 200 and (admin_tasks or {}).get("code") == 0 and isinstance((admin_tasks or {}).get("data"), list),
    f"status={status} type={type((admin_tasks or {}).get('data')).__name__}",
)
for path in [
    "/api/admin/users",
    "/api/admin/ai-logs",
    "/api/admin/credit-logs",
    "/api/admin/assets",
    "/api/admin/settings",
    "/api/admin/prompts",
    "/api/admin/comfy-bridges",
]:
    status, _, _ = call("GET", path, user_token)
    check(f"普通用户访问 {path} 被拒", status == 401, f"status={status}")

print("\n=== 6. 对象下载鉴权（签名 + 归属）===")
project_json = json.dumps(results_of(admin_projects)[0] if results_of(admin_projects) else {}, ensure_ascii=False)
match = re.search(r"/api/files/([0-9A-Za-z_\-]+)/content\?s=([0-9a-f]+)", project_json)
if not match:
    check("从 admin 画布数据取到带签名的对象地址", False, "未匹配到（画布媒体地址可能未回填签名）")
else:
    obj_id, obj_sig = match.group(1), match.group(2)
    check("从 admin 画布数据取到带签名的对象地址", True, f"id={obj_id}")

    status, payload, _ = call("GET", f"/api/files/{obj_id}/content")
    check("content 无签名被拒", status >= 400 or (payload or {}).get("code") != 0, f"status={status} body={payload}")

    status, payload, _ = call("GET", f"/api/files/{obj_id}/content?s=deadbeef")
    check("content 错误签名被拒", status >= 400 or (payload or {}).get("code") != 0, f"status={status}")

    status, payload, size = call("GET", f"/api/files/{obj_id}/content?s={obj_sig}")
    check("content 正确签名可下载到真实字节", status == 200 and payload is None and size > 1000, f"status={status} bytes={size}")

    status, _, _ = call("GET", f"/api/files/{obj_id}")
    check("files/:id 无 token 401", status == 401, f"status={status}")

    status, payload, _ = call("GET", f"/api/files/{obj_id}", user_token)
    check("他人（guoguogis）读取 admin 对象被拒", (payload or {}).get("code") != 0, f"code={(payload or {}).get('code')} msg={(payload or {}).get('msg')}")

    status, payload, _ = call("GET", f"/api/files/{obj_id}", admin_token)
    check("归属者（admin）可读取对象元数据", status == 200 and (payload or {}).get("code") == 0, f"status={status}")
    content_url = ((payload or {}).get("data") or {}).get("contentUrl", "")
    check("对象元数据返回带签名的 contentUrl", bool(re.search(r"\?s=[0-9a-f]{32}$", content_url)), f"contentUrl={content_url}")

print("\n=== 7. 执行中任务隔离与管理端全量 ===")
fix_admin_video = os.environ.get("FIX_ADMIN_VIDEO", "")
fix_user_video = os.environ.get("FIX_USER_VIDEO", "")
fix_admin_image = os.environ.get("FIX_ADMIN_IMAGE", "")
fix_user_image = os.environ.get("FIX_USER_IMAGE", "")
if not all([fix_admin_video, fix_user_video, fix_admin_image, fix_user_image]):
    check("测试夹具已就绪", False, "缺少 FIX_* 环境变量")
else:
    check("测试夹具已就绪", True, "4 条执行中任务")

    _, vt_user, _ = call("GET", "/api/v1/video-tasks", user_token)
    check("guoguogis 的视频任务列表含自己的夹具", fix_user_video in ids_of(vt_user), f"ids={ids_of(vt_user)}")
    check("guoguogis 的视频任务列表不含 admin 的夹具", fix_admin_video not in ids_of(vt_user), f"ids={ids_of(vt_user)}")

    _, vt_admin, _ = call("GET", "/api/v1/video-tasks", admin_token)
    check("admin 的视频任务列表含自己的夹具", fix_admin_video in ids_of(vt_admin), f"ids={ids_of(vt_admin)}")
    check("admin 的用户侧视频任务列表不含 guoguogis 的夹具", fix_user_video not in ids_of(vt_admin), f"ids={ids_of(vt_admin)}")

    _, it_user, _ = call("GET", "/api/v1/canvas/image-tasks", user_token)
    check("guoguogis 的图片任务列表含自己的夹具", fix_user_image in ids_of(it_user), f"ids={ids_of(it_user)}")
    check("guoguogis 的图片任务列表不含 admin 的夹具", fix_admin_image not in ids_of(it_user), f"ids={ids_of(it_user)}")

    status, admin_all, _ = call("GET", "/api/admin/tasks", admin_token)
    all_ids = [item.get("id") for item in results_of(admin_all)]
    check("admin 任务列表含两个账号的视频夹具", fix_admin_video in all_ids and fix_user_video in all_ids, f"ids={all_ids}")
    check("admin 任务列表含两个账号的图片夹具", fix_admin_image in all_ids and fix_user_image in all_ids, f"ids={all_ids}")

    fixtures = {item.get("id"): item for item in results_of(admin_all) if item.get("id") in {fix_admin_video, fix_user_video, fix_admin_image, fix_user_image}}
    check("admin 任务列表返回 4 条夹具", len(fixtures) == 4, f"count={len(fixtures)}")
    check(
        "管理员任务项标注所属用户名",
        all((item.get("userName") or "").strip() for item in fixtures.values()),
        f"names={[item.get('userName') for item in fixtures.values()]}",
    )
    kinds = sorted({item.get("kind") for item in fixtures.values()})
    check("管理员任务项包含 video / image 类型", kinds == ["image", "video"], f"kinds={kinds}")
    check(
        "管理员任务项都处于执行中状态",
        all((item.get("status") or "") in {"queued", "processing", "running", "in_progress"} for item in fixtures.values()),
        f"statuses={[item.get('status') for item in fixtures.values()]}",
    )
    check(
        "管理员任务项带进度值",
        all(isinstance(item.get("progress"), int) for item in fixtures.values()),
        f"progress={[item.get('progress') for item in fixtures.values()]}",
    )
    user_fixture = fixtures.get(fix_user_video) or {}
    admin_fixture = fixtures.get(fix_admin_video) or {}
    check(
        "管理员任务项区分了两个用户",
        user_fixture.get("userId") == user_user["id"] and admin_fixture.get("userId") == admin_user["id"],
        f"{user_fixture.get('userId')} / {admin_fixture.get('userId')}",
    )

    status, filtered, _ = call("GET", f"/api/admin/tasks?userId={user_user['id']}", admin_token)
    filtered_ids = [item.get("id") for item in results_of(filtered)]
    check("管理员可按 userId 过滤任务", fix_user_video in filtered_ids and fix_admin_video not in filtered_ids, f"ids={filtered_ids}")

    status, filtered_bad, _ = call("GET", "/api/admin/tasks?userId=not-exist-user", admin_token)
    check("管理员按不存在的 userId 过滤返回空", len(results_of(filtered_bad)) == 0, f"count={len(results_of(filtered_bad))}")

print(f"\n=== 结果：通过 {passed} / 失败 {len(failures)} ===")
if failures:
    print("\n失败项：")
    for item in failures:
        print("  - " + item)
    sys.exit(1)
print("全部通过")
