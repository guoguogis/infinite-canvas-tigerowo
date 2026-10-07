// 生成 service/prompt_source_builtin_data.go：把上游数据转换成本项目格式并压缩内置。
import { gzipSync, constants } from "node:zlib";
import { writeFileSync } from "node:fs";

import { fetchText, fetchJSON, fetchNextData, extractEmbeddedObject } from "./shared.mjs";

const SNAPSHOT = "?x-oss-process=video/snapshot,t_0,f_jpg,w_720";

const DATASETS = [
    {
        id: "seedance-2-prompts",
        name: "Seedance 2.0 Prompts",
        homepage: "https://github.com/AtlasCloudAI/awesome-seedance-2-prompts",
        build: buildSeedance,
    },
    {
        id: "lanshu-video-prompts",
        name: "Lanshu AI Video Kit",
        homepage: "https://github.com/cclank/lanshu-awesome-ai-video-kit",
        build: buildLanshu,
    },
    {
        id: "atlas-minimax-h3-prompts",
        name: "MiniMax H3 Prompt Library",
        homepage: "https://www.atlascloud.ai/prompts-hub/minimax-h3-prompt",
        build: () => buildHub("minimax-h3-prompt", "atlas-minimax-h3-prompts"),
    },
];

const str = (value) => (typeof value === "string" ? value.trim() : typeof value === "number" ? String(value) : "");
const strList = (value) => (Array.isArray(value) ? value.map(str).filter(Boolean) : []);
const uniqueTags = (...groups) => [...new Set(groups.flat().map(str).filter(Boolean))];
const tagFrom = (value) => {
    const text = str(value).replace(/-/g, " ");
    return text ? text[0].toUpperCase() + text.slice(1) : "";
};
const videoCover = (url) => (str(url) ? str(url) + SNAPSHOT : "");

async function buildSeedance() {
    const rows = await fetchJSON("https://raw.githubusercontent.com/AtlasCloudAI/awesome-seedance-2-prompts/main/data/prompts.json");
    return rows
        .filter((row) => str(row.title) && str(row.prompt))
        .map((row) => ({
            id: `seedance-2-prompts-${str(row.id)}`,
            title: str(row.title),
            prompt: str(row.prompt),
            coverUrl: videoCover(row.video_url),
            videoUrl: str(row.video_url),
            tags: uniqueTags([tagFrom(row.category), str(row.source_platform)]),
            preview: str(row.description),
            author: str(row.author_name),
            sourceUrl: str(row.source_link),
        }));
}

async function buildLanshu() {
    const root = await fetchJSON("https://raw.githubusercontent.com/cclank/lanshu-awesome-ai-video-kit/main/prompts/data/all-prompts.json");
    const categoryNames = Object.fromEntries((root.categories ?? []).map((item) => [str(item.id), str(item.zh)]));
    const modelNames = Object.fromEntries(Object.entries(root.models ?? {}).map(([code, value]) => [code, str(value?.name)]));
    return (root.prompts ?? [])
        .filter((row) => str(row.title) && str(row.prompt))
        .map((row) => {
            const category = categoryNames[str(row.category)] || tagFrom(row.category);
            const model = modelNames[str(row.model)] || tagFrom(row.model);
            const note = str(row.notes);
            return {
                id: `lanshu-video-prompts-${str(row.id)}`,
                title: str(row.title),
                prompt: str(row.prompt),
                coverUrl: "",
                tags: uniqueTags([category, model], strList(row.tags)),
                preview: note,
                author: category || str(row.source?.name),
                sourceUrl: str(row.source?.url),
            };
        });
}

async function buildHub(slug, id) {
    const pageUrl = `https://www.atlascloud.ai/prompts-hub/${slug}`;
    const { payload } = await fetchNextData(pageUrl);
    const seed = extractEmbeddedObject(payload);
    if (!seed) throw new Error(`未能解析 ${pageUrl} 的内嵌数据`);
    const rows = seed.promptsByLocale?.["zh-CN"] ?? seed.promptsByLocale?.en ?? [];
    return rows
        .filter((row) => str(row.title) && str(row.prompt))
        .map((row) => {
            const images = strList(row.remote_images);
            const videos = strList(row.remote_videos);
            return {
                id: `${id}-${str(row.id)}`,
                title: str(row.title),
                prompt: str(row.prompt),
                coverUrl: images[0] || videoCover(videos[0]),
                videoUrl: videos[0] ?? "",
                tags: uniqueTags(strList(row.prompt_categories)),
                preview: str(row.description),
                author: str(row.author_name),
                sourceUrl: str(row.source_link) || pageUrl,
            };
        });
}

const chunk = (text) => text.match(/.{1,100}/gs) ?? [];
const constPrefix = (id) => id.toUpperCase().replace(/-/g, "_");
const varName = (id) =>
    id
        .split("-")
        .map((part, index) => (index === 0 ? part : part.charAt(0).toUpperCase() + part.slice(1)))
        .join("") + "PromptSourceData";

const lines = [
    "package service",
    "",
    "// Generated file. Do not edit by hand.",
    "// Sources are listed on each prompt source homepage; regenerate and sync from the prompt source page.",
    "",
];

const summary = [];
for (const dataset of DATASETS) {
    const items = await dataset.build();
    const encoded = gzipSync(Buffer.from(JSON.stringify(items), "utf8"), { level: constants.Z_BEST_COMPRESSION }).toString("base64");
    const withCover = items.filter((item) => item.coverUrl).length;
    summary.push(`${dataset.id.padEnd(28)} ${String(items.length).padStart(4)} 条, 有封面 ${String(withCover).padStart(4)}, base64 ${(encoded.length / 1024).toFixed(0)} KB`);

    lines.push("const (");
    lines.push(`\t${constPrefix(dataset.id)}_ID   = ${JSON.stringify(dataset.id)}`);
    lines.push(`\t${constPrefix(dataset.id)}_NAME = ${JSON.stringify(dataset.name)}`);
    lines.push(`\t${constPrefix(dataset.id)}_HOME = ${JSON.stringify(dataset.homepage)}`);
    lines.push(")", "");

    lines.push(`var ${varName(dataset.id)} = []string{`);
    for (const part of chunk(encoded)) lines.push(`\t${JSON.stringify(part)},`);
    lines.push("}", "");
}

writeFileSync("service/prompt_source_builtin_data.go", lines.join("\n"), "utf8");
console.log(summary.join("\n"));
console.log("written service/prompt_source_builtin_data.go");
