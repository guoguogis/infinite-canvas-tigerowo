// 共享工具：抓取 Next.js 页面并解析内嵌的 RSC 数据。
import { writeFileSync } from "node:fs";

const USER_AGENT = "Mozilla/5.0";

export async function fetchText(url) {
    const response = await fetch(url, { headers: { "User-Agent": USER_AGENT } });
    if (!response.ok) throw new Error(`${url} -> HTTP ${response.status}`);
    return response.text();
}

export async function fetchJSON(url) {
    return JSON.parse(await fetchText(url));
}

// 从 Next.js App Router 的 flight 载荷里还原出服务端数据。
export async function fetchNextData(pageUrl) {
    const html = await fetchText(pageUrl);
    const chunks = [...html.matchAll(/self\.__next_f\.push\(\[1,\s*"((?:[^"\\]|\\.)*)"\]\)/g)].map((match) => match[1]);
    let payload = "";
    for (const chunk of chunks) {
        try {
            payload += JSON.parse(`"${chunk}"`);
        } catch {
            // 无法解码的片段直接跳过
        }
    }
    return { html, payload };
}

// 按括号配对从文本里截出一个完整 JSON 对象。
export function extractObjectAt(text, startIndex) {
    let depth = 0;
    let inString = false;
    let escaped = false;
    for (let index = startIndex; index < text.length; index += 1) {
        const char = text[index];
        if (inString) {
            if (escaped) escaped = false;
            else if (char === "\\") escaped = true;
            else if (char === '"') inString = false;
            continue;
        }
        if (char === '"') inString = true;
        else if (char === "{") depth += 1;
        else if (char === "}") {
            depth -= 1;
            if (depth === 0) return text.slice(startIndex, index + 1);
        }
    }
    return null;
}

// 取出载荷中满足条件的对象，条件默认要求含 promptsByLocale。
export function extractEmbeddedObject(payload, marker = '"initialSeedData"', accept = (value) => Boolean(value?.promptsByLocale)) {
    const markerIndex = payload.indexOf(marker);
    if (markerIndex < 0) return null;
    const arrayStart = payload.indexOf("[", payload.indexOf(":", markerIndex));
    for (let index = arrayStart; index < payload.length; index += 1) {
        if (payload[index] !== "{") continue;
        const candidate = extractObjectAt(payload, index);
        if (!candidate) continue;
        try {
            const parsed = JSON.parse(candidate);
            if (accept(parsed)) return parsed;
        } catch {
            // 继续找下一个
        }
    }
    return null;
}

// 把资产目录导出成本地占位常量，避免用户手工搬运时遗漏。
export function writeJson(path, value) {
    writeFileSync(path, `${JSON.stringify(value, null, 2)}\n`, "utf8");
}
