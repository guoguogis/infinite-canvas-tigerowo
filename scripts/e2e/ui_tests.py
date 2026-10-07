#!/usr/bin/env python
"""infinite-canvas 浏览器端功能测试（Playwright + Chrome）：
账号本地数据隔离、我的任务角标与抽屉、跳回执行中任务、管理员全量视图、登录守卫。

用法（需先启动前后端，并用 testctl seed 插入夹具）：
    python scripts/e2e/ui_tests.py

环境变量：
    IC_E2E_WEB         前端地址，默认 http://127.0.0.1:3003
    IC_E2E_BASE        后端地址，默认 http://127.0.0.1:8080
    IC_E2E_ADMIN_USER  管理员用户名，默认 admin
    IC_E2E_ADMIN_PASS  管理员密码（必填，不写进仓库）
    IC_E2E_USER_USER   普通用户名，默认 guoguogis
    IC_E2E_USER_PASS   普通用户密码（必填，不写进仓库）
    FIX_USER_VIDEO     普通用户的执行中视频任务 id（用于跳转定位断言）
    IC_E2E_BROWSER_CHANNEL  Playwright 浏览器通道，默认 chrome
"""
import json
import os
import re
import sys
import time

import httpx
from playwright.sync_api import sync_playwright


def credentials(prefix, default_user):
    username = os.environ.get(f"IC_E2E_{prefix}_USER", default_user)
    password = os.environ.get(f"IC_E2E_{prefix}_PASS", "")
    if not password:
        raise SystemExit(f"缺少环境变量 IC_E2E_{prefix}_PASS")
    return username, password


WEB = os.environ.get("IC_E2E_WEB", "http://127.0.0.1:3003")
API = os.environ.get("IC_E2E_BASE", "http://127.0.0.1:8080")
BROWSER_CHANNEL = os.environ.get("IC_E2E_BROWSER_CHANNEL", "chrome")
ADMIN_CRED = credentials("ADMIN", "admin")
USER_CRED = credentials("USER", "guoguogis")
ADMIN_ID = ""
USER_ID = ""
FIX_USER_VIDEO = os.environ.get("FIX_USER_VIDEO", "test-fixture-video-user")
RUN_TAG = os.urandom(4).hex()
CANVAS_FIXTURE_ID = f"test-fixture-canvas-{RUN_TAG}"
CANVAS_FIXTURE_TITLE = f"E2E-guguogis-私有画布-{RUN_TAG}"
CANVAS_FIXTURE_NODE = f"test-fixture-node-{RUN_TAG}"

passed = 0
failures = []
notes = []


def check(name, ok, detail=""):
    global passed
    if ok:
        passed += 1
        print(f"PASS  {name}")
    else:
        failures.append(f"{name}  [{detail}]")
        print(f"FAIL  {name}  [{detail}]")


def note(text):
    notes.append(text)
    print(f"NOTE  {text}")


# ---------- 测试前置：通过接口准备 guoguogis 的私有画布（含一个执行中节点） ----------

def api_login(client, username, password):
    payload = client.post("/api/auth/login", json={"username": username, "password": password}).json()
    assert payload["code"] == 0, payload
    return payload["data"]["token"], payload["data"]["user"]


api = httpx.Client(base_url=API, timeout=60.0)
user_token, user_account = api_login(api, *USER_CRED)
admin_token, admin_account = api_login(api, *ADMIN_CRED)
USER_ID = user_account["id"]
ADMIN_ID = admin_account["id"]

admin_projects = api.get("/api/v1/canvas/projects", headers={"Authorization": f"Bearer {admin_token}"}).json()["data"]
admin_titles = [item.get("title", "") for item in admin_projects if isinstance(item, dict)]
# 先硬清理上一次可能残留的同名夹具（testctl -mode clean 已在外部执行），保证断言可重复
user_projects_before = api.get("/api/v1/canvas/projects", headers={"Authorization": f"Bearer {user_token}"}).json()["data"]
check("初始状态下 guoguogis 账号没有画布项目", len(user_projects_before) == 0, f"count={len(user_projects_before)}")
check("初始状态下 admin 账号有画布项目", len(admin_titles) > 0, f"titles={admin_titles}")

now = "2026-01-01T00:00:00.000Z"
canvas_fixture = {
    "id": CANVAS_FIXTURE_ID,
    "title": CANVAS_FIXTURE_TITLE,
    "createdAt": now,
    "updatedAt": now,
    "nodes": [
        {
            "id": CANVAS_FIXTURE_NODE,
            "type": "video",
            "title": "E2E 执行中视频节点",
            "position": {"x": 0, "y": 0},
            "width": 480,
            "height": 300,
            "metadata": {"status": "loading", "videoTaskId": "test-fixture-canvas-video-task", "prompt": "E2E 画布执行中任务", "model": "fixture-canvas-model", "startedAt": 1767225600000},
        }
    ],
    "connections": [],
    "chatSessions": [],
    "activeChatId": None,
    "agentConfig": None,
    "autoTitlePending": False,
    "backgroundMode": "lines",
    "showImageInfo": False,
    "viewport": {"x": 0, "y": 0, "k": 1},
    "sidePanel": {"open": True, "width": 280},
    "agentPanel": {"open": False, "width": 464},
}
created = api.post(
    "/api/v1/canvas/projects",
    headers={"Authorization": f"Bearer {user_token}"},
    json={"data": canvas_fixture},
)
check("为 guoguogis 准备私有画布项目成功", created.json().get("code") == 0, f"resp={created.text[:160]}")

# ---------- 选择器与断言辅助 ----------

# antd 6 的抽屉内容是 .ant-drawer-section（antd 5 为 .ant-drawer-content）
DRAWER_SECTION = ".ant-drawer-section"
DRAWER = ".ant-drawer"
# localforage 自带的探测库不属于业务数据
IGNORED_STORES = {"local-forage-detect-blob-support"}


def scoped_to(name, user_id):
    return name.startswith(f"u_{user_id}__")


def open_task_drawer(page, already_open=False):
    if not already_open:
        page.click('button[aria-label="账户菜单"]')
        page.wait_for_selector(".ant-dropdown-menu-item", timeout=15000)
    item = page.locator(".ant-dropdown-menu-item").filter(has_text="我的任务").first
    item.click()
    page.wait_for_selector(DRAWER, timeout=15000)
    page.wait_for_timeout(1200)
    return page.locator(DRAWER_SECTION).first.inner_text()

# ---------- 浏览器端检查 ----------

IDB_STORES_JS = """
async () => {
  if (!indexedDB.databases) return { error: "indexedDB.databases 不可用" };
  const dbs = await indexedDB.databases();
  const target = dbs.find((d) => d.name === "infinite-canvas");
  if (!target) return { version: 0, stores: [] };
  return await new Promise((resolve) => {
    const req = indexedDB.open("infinite-canvas");
    req.onerror = () => resolve({ error: "打开 infinite-canvas 数据库失败" });
    req.onsuccess = () => {
      const names = Array.from(req.result.objectStoreNames);
      req.result.close();
      resolve({ version: target.version, stores: names });
    };
  });
}
"""


def login(page, username, password):
    page.goto(f"{WEB}/login")
    page.wait_for_selector('input[autocomplete="username"]', timeout=30000)
    page.fill('input[autocomplete="username"]', username)
    page.fill('input[autocomplete="current-password"]', password)
    page.click('button[type="submit"]')
    page.wait_for_selector("text=我的历史记录", timeout=30000)
    # 应用内有常驻轮询，networkidle 永远不会到达，改用显式等待
    page.wait_for_timeout(1200)


def logout(page):
    close = page.locator(".ant-drawer-close")
    if close.count():
        close.first.click()
        page.wait_for_timeout(900)
    page.click('button[aria-label="账户菜单"]')
    page.wait_for_selector(".ant-dropdown-menu-item", timeout=15000)
    page.locator(".ant-dropdown-menu-item").filter(has_text="退出登录").first.click()
    page.wait_for_selector('input[autocomplete="username"]', timeout=30000)


def row_text(page, title):
    return page.locator("section").filter(has_text=title).first.inner_text()


def wait_for_row(page, title, predicate, timeout=25000):
    """首页数据来自本地存储 + 账号同步，需要等待同步完成。"""
    deadline = time.time() + timeout / 1000
    text = ""
    while time.time() < deadline:
        text = row_text(page, title)
        if predicate(text):
            return text
        page.wait_for_timeout(500)
    return text


def badge_count(page):
    locator = page.locator(".ant-badge-count")
    if locator.count() == 0:
        return 0
    text = (locator.first.get_attribute("title") or locator.first.inner_text() or "").strip()
    match = re.search(r"\d+", text)
    return int(match.group(0)) if match else 0


def idb_stores(page):
    return page.evaluate(IDB_STORES_JS)


def wait_for_badge(page, minimum, timeout=20000):
    deadline = time.time() + timeout / 1000
    while time.time() < deadline:
        if badge_count(page) >= minimum:
            return badge_count(page)
        page.wait_for_timeout(500)
    return badge_count(page)


with sync_playwright() as p:
    browser = p.chromium.launch(headless=True, channel=BROWSER_CHANNEL)
    # 同一个浏览器 profile：模拟“同一台电脑切换账号登录”
    context = browser.new_context(viewport={"width": 1440, "height": 1000})
    page = context.new_page()

    print("\n=== A. 登录守卫 ===")
    page.goto(f"{WEB}/canvas")
    page.wait_for_selector('input[autocomplete="username"]', timeout=30000)
    check("未登录访问 /canvas 被跳转到登录页", "/login" in page.url, f"url={page.url}")
    check("跳转带回 redirect 参数", "redirect=%2Fcanvas" in page.url or "redirect=/canvas" in page.url, f"url={page.url}")

    print("\n=== B. 以 guoguogis 登录：本地命名空间与我的任务 ===")
    login(page, *USER_CRED)
    check("登录后进入控制台首页", "我的历史记录" in page.content(), f"url={page.url}")

    stores = idb_stores(page)
    check("能够读取浏览器 IndexedDB 存储清单", not stores.get("error"), f"{stores}")
    user_stores = stores.get("stores", [])
    note(f"guoguogis 命名空间下的 object store：{sorted(user_stores)}")
    own_stores = [name for name in user_stores if scoped_to(name, USER_ID)]
    admin_stores_present = [name for name in user_stores if scoped_to(name, ADMIN_ID)]
    unexpected_stores = [name for name in user_stores if name not in IGNORED_STORES and not scoped_to(name, USER_ID) and not name.startswith("guest__")]
    check("guoguogis 登录后使用自己的本地命名空间", len(own_stores) > 0, f"stores={sorted(user_stores)}")
    check("guoguogis 的浏览器里不存在 admin 的命名空间", not admin_stores_present, f"admin={admin_stores_present}")
    check(
        "除访客命名空间与 localforage 探测库外，本地存储都属于 guoguogis",
        not unexpected_stores,
        f"unexpected={unexpected_stores}",
    )

    canvas_row = wait_for_row(page, "我的画布", lambda text: CANVAS_FIXTURE_TITLE in text)
    check("首页我的画布行显示 guoguogis 自己的画布", CANVAS_FIXTURE_TITLE in canvas_row, f"row={canvas_row[:200]}")
    check("首页我的画布行看不到 admin 的画布", not any(title and title in canvas_row for title in admin_titles), f"row={canvas_row[:200]}")

    video_row = wait_for_row(page, "我的视频", lambda text: "执行中" in text)
    check("首页我的视频行出现执行中任务卡片", "执行中" in video_row, f"row={video_row[:200]}")
    check("首页我的画布行出现执行中任务卡片", "执行中" in canvas_row, f"row={canvas_row[:200]}")

    count = wait_for_badge(page, 2)
    note(f"guoguogis 角标数量 = {count}")
    check("头像角标显示执行中任务数量（≥2）", count >= 2, f"badge={count}")

    page.click('button[aria-label="账户菜单"]')
    page.wait_for_selector(".ant-dropdown-menu-item", timeout=15000)
    menu_text = page.locator(".ant-dropdown").first.inner_text()
    check("头像菜单出现「我的任务」入口并带数量", "我的任务" in menu_text and re.search(r"我的任务\s*\n?\s*\d+", menu_text) is not None, f"menu={menu_text[:120]}")
    drawer = open_task_drawer(page, already_open=True)
    check("「我的任务」抽屉打开并标题正确", "我的任务" in drawer, f"drawer={drawer[:160]}")
    check("抽屉里列出执行中任务", "执行中" in drawer, f"drawer={drawer[:300]}")
    check("抽屉里的任务带进度条", page.locator(f"{DRAWER_SECTION} .bg-sky-500").count() > 0, "未找到进度条元素")
    check("普通用户抽屉里不出现其他账号用户名", "admin" not in drawer, f"drawer={drawer[:300]}")

    print("\n=== C. 从任务抽屉跳回执行中任务 ===")
    drawer_task = page.locator(f"{DRAWER_SECTION} button").filter(has_text="fixture-video-model").first
    if drawer_task.count() == 0:
        check("抽屉中存在视频夹具任务可点击", False, f"drawer={drawer[:300]}")
    else:
        check("抽屉中存在视频夹具任务可点击", True)
        drawer_task.click()
        page.wait_for_url(re.compile(r"/video\?task="), timeout=20000)
        page.wait_for_timeout(3500)
        check("点击任务后跳转到 /video?task=<id>", f"task={FIX_USER_VIDEO}" in page.url, f"url={page.url}")
        page.wait_for_timeout(2500)
        cards = page.locator("[data-log-id]")
        check("视频页渲染出该任务的记录卡片", cards.count() > 0, f"cards={cards.count()}")
        if cards.count() > 0:
            classes = [cards.nth(index).get_attribute("class") or "" for index in range(cards.count())]
            check("该任务卡片被高亮定位", any("border-stone-900" in value for value in classes), f"classes={classes}")

    print("\n=== D. 画布执行中任务深链 ===")
    page.goto(f"{WEB}/canvas/{CANVAS_FIXTURE_ID}?nodeId={CANVAS_FIXTURE_NODE}")
    page.wait_for_timeout(4000)
    node = page.locator(f'[data-node-id="{CANVAS_FIXTURE_NODE}"]')
    check("画布页通过 ?nodeId= 渲染出目标节点", node.count() > 0, f"url={page.url} nodes={node.count()}")
    check("画布页 URL 保留 nodeId 参数", f"nodeId={CANVAS_FIXTURE_NODE}" in page.url, f"url={page.url}")

    print("\n=== E. 同一浏览器切换到 admin ===")
    logout(page)
    login(page, *ADMIN_CRED)

    admin_stores = idb_stores(page).get("stores", [])
    note(f"切换后 object store 总数：{len(admin_stores)}")
    admin_own = [name for name in admin_stores if scoped_to(name, ADMIN_ID)]
    user_kept = [name for name in admin_stores if scoped_to(name, USER_ID)]
    check("admin 登录后使用自己的本地命名空间", len(admin_own) > 0, f"stores={sorted(admin_stores)}")
    check("guoguogis 的命名空间仍然保留（未被清空）", len(user_kept) > 0, f"stores={sorted(admin_stores)}")
    check(
        "两个账号的命名空间互不重叠",
        not (set(admin_own) & set(user_kept)),
        f"admin={sorted(admin_own)} user={sorted(user_kept)}",
    )

    admin_canvas_row = wait_for_row(page, "我的画布", lambda text: any(title and title in text for title in admin_titles))
    check("admin 首页能看到自己的画布", any(title and title in admin_canvas_row for title in admin_titles), f"row={admin_canvas_row[:200]}")
    check("admin 首页看不到 guoguogis 的私有画布", CANVAS_FIXTURE_TITLE not in admin_canvas_row, f"row={admin_canvas_row[:200]}")

    admin_count = wait_for_badge(page, 4)
    note(f"admin 角标数量 = {admin_count}")
    check("admin 角标至少包含全部 4 条夹具任务", admin_count >= 4, f"badge={admin_count}")

    admin_drawer = open_task_drawer(page)
    check("管理员抽屉包含其他账号（guoguogis）的任务", "guoguogis" in admin_drawer, f"drawer={admin_drawer[:400]}")
    check("管理员抽屉里同时有 admin 自己的任务", "admin" in admin_drawer, f"drawer={admin_drawer[:400]}")
    drawer_task_count = len(re.findall(r"执行中", admin_drawer))
    note(f"管理员抽屉中「执行中」标记出现 {drawer_task_count} 次")
    check("管理员抽屉列出全部 4 条夹具任务", drawer_task_count >= 4, f"occurrences={drawer_task_count}")

    print("\n=== F. 切回 guoguogis：本地数据仍在 ===")
    logout(page)
    login(page, *USER_CRED)
    back_row = wait_for_row(page, "我的画布", lambda text: CANVAS_FIXTURE_TITLE in text)
    check("切回 guoguogis 后自己的画布仍在", CANVAS_FIXTURE_TITLE in back_row, f"row={back_row[:200]}")
    back_count = wait_for_badge(page, 2)
    check("切回后角标恢复为 guoguogis 的任务数（≥2）", back_count >= 2, f"badge={back_count}")

    browser.close()

# ---------- 清理测试夹具 ----------
cleanup_headers = {"Authorization": f"Bearer {user_token}"}
delete_project = api.post("/api/v1/canvas/projects/delete", headers=cleanup_headers, json={"ids": [CANVAS_FIXTURE_ID]})
check("清理 guoguogis 的画布夹具", delete_project.json().get("code") == 0, f"resp={delete_project.text[:160]}")
remaining = api.get("/api/v1/canvas/projects", headers=cleanup_headers).json()["data"]
check("清理后 guoguogis 画布列表为空", len(remaining) == 0, f"count={len(remaining)}")

print(f"\n=== 结果：通过 {passed} / 失败 {len(failures)} ===")
for item in notes:
    print("NOTE  " + item)
if failures:
    print("\n失败项：")
    for item in failures:
        print("  - " + item)
    sys.exit(1)
print("全部通过")
