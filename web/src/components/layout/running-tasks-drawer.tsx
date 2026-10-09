"use client";

import { useEffect, useState } from "react";
import { Drawer, Empty } from "antd";
import { ImagePlus, LoaderCircle, Music2, Video } from "lucide-react";
import { useRouter } from "next/navigation";

import { useRunningTasksPolling } from "@/hooks/use-running-tasks";
import { formatDuration } from "@/lib/image-utils";
import { useTaskStore, type RunningTask, type RunningTaskKind } from "@/stores/use-task-store";

const kindIcons: Record<RunningTaskKind, typeof Video> = {
    video: Video,
    image: ImagePlus,
    audio: Music2,
    music: Music2,
};

const statusLabels: Record<string, string> = {
    queued: "排队中",
    pending: "排队中",
    processing: "执行中",
    in_progress: "执行中",
    running: "执行中",
};

/**
 * 「我的任务」抽屉：列出所有执行中的生成任务，点击回到来源页面并定位该任务。
 * 同时负责启动全局任务轮询，供右上角角标使用。
 */
export function RunningTasksDrawer() {
    useRunningTasksPolling();
    const router = useRouter();
    const open = useTaskStore((state) => state.drawerOpen);
    const closeDrawer = useTaskStore((state) => state.closeDrawer);
    const tasks = useTaskStore((state) => state.tasks);
    const loading = useTaskStore((state) => state.loading);
    const now = useElapsedNow(open && tasks.length > 0);

    const openTask = (task: RunningTask) => {
        if (!task.href) return;
        closeDrawer();
        router.push(task.href);
    };

    return (
        <Drawer title={tasks.length ? `我的任务 · ${tasks.length}` : "我的任务"} placement="right" size={420} open={open} onClose={closeDrawer}>
            {loading && !tasks.length ? (
                <p className="text-sm text-stone-500 dark:text-stone-400">正在加载...</p>
            ) : tasks.length ? (
                <div className="flex flex-col gap-3">
                    {tasks.map((task) => (
                        <RunningTaskCard key={task.id} task={task} now={now} onOpen={() => openTask(task)} />
                    ))}
                </div>
            ) : (
                <Empty className="!mt-16" description="当前没有执行中的任务" />
            )}
        </Drawer>
    );
}

function RunningTaskCard({ task, now, onOpen }: { task: RunningTask; now: number; onOpen: () => void }) {
    const Icon = kindIcons[task.kind];
    const progress = Math.max(0, Math.min(100, Math.floor(task.progress || 0)));
    const clickable = Boolean(task.href);
    const content = (
        <>
            <div className="flex items-start gap-3">
                <span className="mt-0.5 inline-flex size-7 shrink-0 items-center justify-center rounded-md bg-stone-100 text-stone-600 dark:bg-stone-800 dark:text-stone-300">
                    <Icon className="size-4" />
                </span>
                <div className="min-w-0 flex-1">
                    <p className="line-clamp-2 text-sm text-stone-800 dark:text-stone-100">{task.title || task.model || "生成任务"}</p>
                    <p className="mt-1 truncate text-xs text-stone-500 dark:text-stone-400">
                        {[task.userName, task.model, now && task.createdAt ? `已耗时 ${formatDuration(Math.max(0, now - task.createdAt))}` : ""].filter(Boolean).join(" · ")}
                    </p>
                </div>
                <span className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-sky-600 dark:text-sky-400">
                    <LoaderCircle className="size-3.5 animate-spin" />
                    {statusLabels[(task.status || "").toLowerCase()] || task.status || "执行中"}
                </span>
            </div>
            <div className="mt-3 h-1.5 w-full overflow-hidden rounded-full bg-stone-200 dark:bg-stone-800">
                <div className="h-full rounded-full bg-sky-500 transition-all duration-300" style={{ width: `${progress}%` }} />
            </div>
        </>
    );

    if (!clickable) {
        return <div className="rounded-lg border border-stone-200 p-3.5 dark:border-stone-800">{content}</div>;
    }
    return (
        <button type="button" onClick={onOpen} className="w-full rounded-lg border border-stone-200 p-3.5 text-left transition hover:border-stone-400 hover:bg-stone-50 dark:border-stone-800 dark:hover:border-stone-600 dark:hover:bg-stone-900">
            {content}
        </button>
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
