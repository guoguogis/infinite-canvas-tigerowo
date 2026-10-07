"use client";

import Link from "next/link";
import { ArrowRight, History } from "lucide-react";
import type { ReactNode } from "react";

import { CanvasDeleteProjectsDialog } from "./canvas/components/canvas-delete-projects-dialog";
import { CanvasProjectCard } from "./canvas/components/canvas-project-card";
import { useCanvasStore } from "./canvas/stores/use-canvas-store";
import { useHomeMediaHistory, type HomeMediaHistoryItem } from "./use-home-media-history";

const HOME_ROW_LIMIT = 10;

export default function IndexPage() {
    const hydrated = useCanvasStore((state) => state.hydrated);
    const projects = useCanvasStore((state) => state.projects);
    const { images, videos, loading: mediaLoading } = useHomeMediaHistory(HOME_ROW_LIMIT);
    const canvases = [...projects].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)).slice(0, HOME_ROW_LIMIT);

    return (
        <main className="h-full overflow-auto bg-background text-stone-950 dark:text-stone-100">
            <div className="mx-auto flex w-full max-w-7xl flex-col gap-10 px-6 py-10">
                <header className="border-b border-stone-200 pb-6 dark:border-stone-800">
                    <p className="text-xs text-stone-500">历史记录</p>
                    <h1 className="mt-3 text-3xl font-semibold">我的历史记录</h1>
                </header>

                <HistoryRow title="我的画布" href="/canvas" count={canvases.length} loading={!hydrated} empty="还没有画布">
                    {canvases.map((project) => (
                        <div key={project.id} className="w-72 shrink-0">
                            <CanvasProjectCard project={project} />
                        </div>
                    ))}
                </HistoryRow>

                <HistoryRow title="我的视频" href="/video" count={videos.length} loading={mediaLoading} empty="还没有生成视频">
                    {videos.map((item) => (
                        <MediaHistoryCard key={item.id} item={item} kind="video" />
                    ))}
                </HistoryRow>

                <HistoryRow title="我的图片" href="/image" count={images.length} loading={mediaLoading} empty="还没有生成图片">
                    {images.map((item) => (
                        <MediaHistoryCard key={item.id} item={item} kind="image" />
                    ))}
                </HistoryRow>
            </div>

            <CanvasDeleteProjectsDialog />
        </main>
    );
}

function HistoryRow({ title, href, count, loading, empty, children }: { title: string; href: string; count: number; loading: boolean; empty: string; children: ReactNode }) {
    return (
        <section className="flex flex-col gap-4">
            <div className="flex items-center justify-between gap-4">
                <div className="flex min-w-0 items-center gap-2">
                    <History className="size-4 shrink-0 text-stone-400" />
                    <h2 className="truncate text-lg font-semibold">{title}</h2>
                </div>
                <Link href={href} className="flex shrink-0 items-center gap-1 text-sm text-stone-500 transition hover:text-stone-950 dark:text-stone-400 dark:hover:text-stone-100">
                    查看全部
                    <ArrowRight className="size-4" />
                </Link>
            </div>
            {loading ? (
                <p className="text-sm text-stone-500">正在加载...</p>
            ) : count ? (
                <div className="thin-scrollbar flex gap-4 overflow-x-auto pb-2">{children}</div>
            ) : (
                <p className="text-sm text-stone-500">{empty}</p>
            )}
        </section>
    );
}

function MediaHistoryCard({ item, kind }: { item: HomeMediaHistoryItem; kind: "image" | "video" }) {
    return (
        <Link href={`/${kind}`} className="w-64 shrink-0 overflow-hidden rounded-lg border border-stone-200 bg-background transition hover:border-stone-400 dark:border-stone-800 dark:hover:border-stone-600">
            <div className="relative aspect-video bg-stone-100 dark:bg-stone-900">
                {kind === "video" ? <video src={item.url} muted playsInline preload="metadata" className="size-full bg-black object-cover" /> : <img src={item.url} alt={item.prompt || "历史图片"} className="size-full object-cover" />}
                {item.status && item.status !== "成功" ? <span className="absolute right-1.5 top-1.5 rounded bg-white/85 px-1.5 py-0.5 text-[10px] text-stone-600 dark:bg-stone-950/80 dark:text-stone-300">{item.status}</span> : null}
            </div>
            <div className="space-y-1 p-2.5 text-xs">
                <p className="line-clamp-2 min-h-8 whitespace-pre-wrap text-stone-700 dark:text-stone-200">{item.prompt || "未填写提示词"}</p>
                <p className="truncate text-stone-500">{[item.model, formatHomeTime(item.createdAt)].filter(Boolean).join(" · ")}</p>
            </div>
        </Link>
    );
}

function formatHomeTime(value: number) {
    if (!value) return "";
    return new Date(value).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}
