"""回归测试：提示词管理（/admin/prompts）里预览视频，关闭弹框后必须停止播放。

用法（需先启动前后端）：
    IC_E2E_ADMIN_PASS=... python scripts/e2e/prompt_video_tests.py

会用管理员接口创建一条带 videoUrl 的临时提示词，测完自动删除。
分别验证三种关闭方式：底部「关闭」按钮、右上角 X、ESC。
"""
import os
import re
import sys

import httpx
from playwright.sync_api import sync_playwright

WEB = os.environ.get("IC_E2E_WEB", "http://127.0.0.1:3003")
API = os.environ.get("IC_E2E_BASE", "http://127.0.0.1:8080")
ADMIN_USER = os.environ.get("IC_E2E_ADMIN_USER", "admin")
ADMIN_PASS = os.environ.get("IC_E2E_ADMIN_PASS", "")
BROWSER_CHANNEL = os.environ.get("IC_E2E_BROWSER_CHANNEL", "chrome")
RUN_TAG = os.urandom(4).hex()
FIXTURE_TITLE = f"E2E-视频预览回归-{RUN_TAG}"

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


if not ADMIN_PASS:
    raise SystemExit("缺少环境变量 IC_E2E_ADMIN_PASS")

api = httpx.Client(base_url=API, timeout=90.0)
login = api.post("/api/auth/login", json={"username": ADMIN_USER, "password": ADMIN_PASS}).json()
assert login["code"] == 0, login
admin_token = login["data"]["token"]
headers = {"Authorization": f"Bearer {admin_token}"}

# 从管理员自己的画布数据里挑一个真正是视频的签名地址
projects = api.get("/api/v1/canvas/projects", headers=headers).json()["data"]
candidates = []
for project in projects:
    if not isinstance(project, dict):
        continue
    for node in project.get("nodes") or []:
        blob = str(node)
        for found in re.findall(r"/api/files/[0-9A-Za-z_\-]+/content\?s=[0-9a-f]+", blob):
            if found not in candidates:
                candidates.append(found)

video_path = ""
for candidate in candidates:
    head = api.get(f"{API}{candidate}", headers={"Range": "bytes=0-1"})
    content_type = head.headers.get("content-type", "")
    print(f"  candidate {candidate[:58]} -> {head.status_code} {content_type}")
    if head.status_code in (200, 206) and content_type.startswith("video/"):
        video_path = candidate
        break
if not video_path:
    raise SystemExit(f"画布里没有可用的视频对象，候选 {len(candidates)} 个，无法进行视频预览测试")
video_url = f"{API}{video_path}"
check("准备到可用于预览的真实视频地址", True, video_url[:80])

prompt_id = f"test-fixture-prompt-{RUN_TAG}"
sample = api.get("/api/admin/prompts", headers=headers, params={"page": 1, "pageSize": 1}).json()
sample_items = (sample.get("data") or {}).get("items") or []
created = api.post(
    "/api/admin/prompts",
    headers=headers,
    json={"id": prompt_id, "title": FIXTURE_TITLE, "prompt": "E2E 视频预览回归测试提示词", "category": sample_items[0].get("category") if sample_items else "", "coverUrl": "", "videoUrl": video_url, "tags": ["e2e"]},
)
check("创建带视频的临时提示词成功", created.json().get("code") == 0, f"resp={created.text[:200]}")

PROBE_OPEN = """
() => {
  const modal = document.querySelector('.ant-modal-wrap:not([style*="display: none"]) .ant-modal');
  const host = modal || document;
  const video = host.querySelector('video');
  if (!video) {
    return { video: null, hasCover: Boolean(host.querySelector('img')) };
  }
  window.__e2eVideo = video;
  return { video: true, paused: video.paused, muted: video.muted, loop: video.loop, autoplay: video.autoplay, readyState: video.readyState, error: video.error ? video.error.code : null };
}
"""

PROBE_AFTER = """
() => {
  const video = window.__e2eVideo;
  return {
    domCount: document.querySelectorAll('video').length,
    visibleVideoCount: document.querySelectorAll('.ant-modal-wrap:not([style*="display: none"]) video').length,
    tracked: Boolean(video),
    paused: video ? video.paused : null,
    connected: video ? video.isConnected : null,
  };
}
"""


def close_and_probe(page, how):
    # 精确定位「含视频的那个 modal」，避免命中其它已挂载但隐藏的弹框
    modal = page.locator(".ant-modal").filter(has=page.locator("video")).first
    if how == "footer":
        # antd 会在两个中文字之间插入空格，实际文本是「关 闭」
        modal.locator(".ant-modal-footer button").filter(has_text=re.compile(r"关\s*闭")).first.click()
    elif how == "close-icon":
        modal.locator(".ant-modal-close").first.click()
    else:
        page.keyboard.press("Escape")
    page.wait_for_timeout(2000)
    return page.evaluate(PROBE_AFTER)


try:
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True, channel=BROWSER_CHANNEL)
        page = browser.new_page(viewport={"width": 1440, "height": 1000})
        page.goto(f"{WEB}/login?redirect=/admin/prompts")
        page.wait_for_selector('input[autocomplete="username"]', timeout=30000)
        page.fill('input[autocomplete="username"]', ADMIN_USER)
        page.fill('input[autocomplete="current-password"]', ADMIN_PASS)
        page.click('button[type="submit"]')
        page.wait_for_selector("text=提示词", timeout=30000)
        page.wait_for_timeout(2500)

        search = page.locator('input[placeholder*="搜索"]').first
        check("提示词管理页有搜索框", search.count() > 0, "未找到搜索框")
        search.fill(FIXTURE_TITLE)
        search.press("Enter")
        page.wait_for_timeout(2000)
        if page.locator("tr").filter(has_text=FIXTURE_TITLE).count() == 0:
            page.locator(".ant-input-search-btn").first.click()
            page.wait_for_timeout(2000)

        row = page.locator("tr").filter(has_text=FIXTURE_TITLE).first
        check("提示词列表能搜到夹具记录", row.count() > 0, f"title={FIXTURE_TITLE}")

        for how, label in [("footer", "底部「关闭」按钮"), ("close-icon", "右上角 X"), ("escape", "ESC 键")]:
            row.locator("button").first.click()
            page.wait_for_selector(".ant-modal video", timeout=20000)
            page.wait_for_timeout(3000)

            opened = page.evaluate(PROBE_OPEN)
            check(f"[{label}] 详情弹框里渲染出视频元素", bool(opened and opened.get("video")), f"probe={opened}")
            check(f"[{label}] 预览视频处于播放状态", bool(opened and opened.get("paused") is False), f"probe={opened}")

            after = close_and_probe(page, how)
            print(f"  {label} 关闭后:", after)
            check(f"[{label}] 关闭后页面已无视频元素", after["domCount"] == 0, f"after={after}")
            check(f"[{label}] 关闭后视频停止播放（paused = true）", after["paused"] is True, f"after={after}")

        browser.close()
finally:
    deleted = api.delete(f"/api/admin/prompts/{prompt_id}", headers=headers)
    check("清理临时提示词", deleted.json().get("code") == 0, f"resp={deleted.text[:160]}")
    remaining = api.get("/api/admin/prompts", headers=headers, params={"keyword": FIXTURE_TITLE, "page": 1, "pageSize": 5}).json()
    left = [item for item in (remaining.get("data") or {}).get("items", []) if item.get("id") == prompt_id]
    check("清理后夹具不再出现在列表", not left, f"left={left}")

print(f"\n=== 结果：通过 {passed} / 失败 {len(failures)} ===")
if failures:
    print("\n失败项：")
    for item in failures:
        print("  - " + item)
    sys.exit(1)
print("全部通过")
