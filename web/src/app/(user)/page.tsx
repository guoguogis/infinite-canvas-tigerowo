"use client";

import Link from "next/link";
import { ArrowRight, History, LoaderCircle } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";

import { CanvasDeleteProjectsDialog } from "./canvas/components/canvas-delete-projects-dialog";
import { CanvasProjectCard } from "./canvas/components/canvas-project-card";
import { useCanvasStore } from "./canvas/stores/use-canvas-store";
import { useHomeMediaHistory, type HomeMediaHistoryItem } from "./use-home-media-history";
import { useTaskStore, type RunningTask, type RunningTaskKind } from "@/stores/use-task-store";

const HOME_ROW_LIMIT = 10;

const runningKindLabels: Record<RunningTaskKind, string> = {
    video: "视频",
    image: "图片",
    audio: "音频",
};

export default function IndexPage() {
    const hydrated = useCanvasStore((state) => state.hydrated);
    const projects = useCanvasStore((state) => state.projects);
    const { images, videos, loading: mediaLoading } = useHomeMediaHistory(HOME_ROW_LIMIT);
    const runningTasks = useTaskStore((state) => state.tasks);
    const now = useElapsedNow(runningTasks.length > 0);
    const running = groupRunningTasks(runningTasks);
    const canvases = [...projects].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)).slice(0, HOME_ROW_LIMIT);

    return (
        <main className="h-full overflow-auto bg-background text-stone-950 dark:text-stone-100">
            <div className="mx-auto flex w-full max-w-7xl flex-col gap-10 px-6 py-10">
                <header className="border-b border-stone-200 pb-6 dark:border-stone-800">
                    <p className="text-xs text-stone-500">历史记录</p>
                    <h1 className="mt-3 text-3xl font-semibold">我的历史记录</h1>
                </header>

                <HistoryRow title="我的画布" href="/canvas" count={running.canvas.length + canvases.length} loading={!hydrated} empty="还没有画布">
                    {running.canvas.map((task) => (
                        <RunningTaskCard key={task.id} task={task} now={now} widthClass="w-72" />
                    ))}
                    {canvases.map((project) => (
                        <div key={project.id} className="w-72 shrink-0">
                            <CanvasProjectCard project={project} />
                        </div>
                    ))}
                </HistoryRow>

                <HistoryRow title="我的视频" href="/video" count={running.video.length + videos.length} loading={mediaLoading && !running.video.length} empty="还没有生成视频">
                    {running.video.map((task) => (
                        <RunningTaskCard key={task.id} task={task} now={now} widthClass="w-64" />
                    ))}
                    {videos.map((item) => (
                        <MediaHistoryCard key={item.id} item={item} kind="video" />
                    ))}
                </HistoryRow>

                <HistoryRow title="我的图片" href="/image" count={running.image.length + images.length} loading={mediaLoading && !running.image.length} empty="还没有生成图片">
                    {running.image.map((task) => (
                        <RunningTaskCard key={task.id} task={task} now={now} widthClass="w-64" />
                    ))}
                    {images.map((item) => (
                        <MediaHistoryCard key={item.id} item={item} kind="image" />
                    ))}
                </HistoryRow>
            </div>

            <CanvasDeleteProjectsDialog />
        </main>
    );
}

/** 按来源页面归类执行中任务：画布内的任务归「我的画布」，工作台任务归对应工作台。 */
function groupRunningTasks(tasks: RunningTask[]) {
    const groups = { canvas: [] as RunningTask[], video: [] as RunningTask[], image: [] as RunningTask[] };
    for (const task of tasks) {
        if (task.href.startsWith("/canvas/")) groups.canvas.push(task);
        else if (task.href.startsWith("/video") || task.kind === "video") groups.video.push(task);
        else if (task.href.startsWith("/image") || task.kind === "image") groups.image.push(task);
    }
    return groups;
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

function RunningTaskCard({ task, now, widthClass }: { task: RunningTask; now: number; widthClass: string }) {
    const progress = Math.max(0, Math.min(100, Math.floor(task.progress || 0)));
    const body = (
        <>
            <div className="relative aspect-video overflow-hidden bg-stone-100 dark:bg-stone-900">
                <div className="absolute inset-0 animate-pulse opacity-60" style={{ backgroundImage: "radial-gradient(circle, rgba(120,113,108,0.35) 1.4px, transparent 1.6px)", backgroundSize: "16px 16px" }} />
                <div className="absolute inset-0 flex flex-col items-center justify-center gap-1.5 text-xs text-stone-500 dark:text-stone-400">
                    <LoaderCircle className="size-5 animate-spin" />
                    <span className="animate-pulse font-medium text-sky-500">{progress ? `执行中 ${progress}%` : "执行中"}</span>
                </div>
                <span className="absolute right-1.5 top-1.5 rounded bg-white/85 px-1.5 py-0.5 text-[10px] text-stone-600 dark:bg-stone-950/80 dark:text-stone-300">执行中</span>
            </div>
            <div className="space-y-1.5 p-2.5 text-xs">
                <p className="line-clamp-2 min-h-8 whitespace-pre-wrap text-stone-700 dark:text-stone-200">{task.title || `${runningKindLabels[task.kind]}任务`}</p>
                <p className="truncate text-stone-500">{[task.userName, task.model, task.createdAt && now ? formatHomeTime(task.createdAt) : ""].filter(Boolean).join(" · ") || "生成中"}</p>
                <div className="h-1 w-full overflow-hidden rounded-full bg-stone-200 dark:bg-stone-800">
                    <div className="h-full rounded-full bg-sky-500 transition-all duration-300" style={{ width: `${progress}%` }} />
                </div>
            </div>
        </>
    );
    const className = `${widthClass} shrink-0 overflow-hidden rounded-lg border border-sky-200 bg-background transition dark:border-sky-900`;
    if (!task.href) return <div className={className}>{body}</div>;
    return (
        <Link href={task.href} className={`${className} hover:border-sky-400 dark:hover:border-sky-700`}>
            {body}
        </Link>
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

function useElapsedNow(active: boolean) {
    const [now, setNow] = useState(0);
    useEffect(() => {
        if (!active) return;
        setNow(Date.now());
        const timer = window.setInterval(() => setNow(Date.now()), 1000);
        return () => window.clearInterval(timer);
    }, [active]);
    return now;
}

function formatHomeTime(value: number) {
    if (!value) return "";
    return new Date(value).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}
