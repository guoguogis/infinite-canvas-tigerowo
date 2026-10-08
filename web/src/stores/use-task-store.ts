"use client";

import { create } from "zustand";

import type { CanvasNodeData } from "@/app/(user)/canvas/types";
import { CanvasNodeType } from "@/app/(user)/canvas/types";
import { isCanvasImageNodeType } from "@/app/(user)/canvas/utils/canvas-panorama";
import { fetchAdminRunningTasks } from "@/services/api/admin";
import { listCanvasImageTasks } from "@/services/api/image";
import { listVideoGenerationTasks } from "@/services/api/video";
import type { AiConfig } from "@/stores/use-config-store";
import { useUserStore } from "@/stores/use-user-store";

export type RunningTaskKind = "video" | "image" | "audio";

export type RunningTask = {
    id: string;
    kind: RunningTaskKind;
    title: string;
    model: string;
    status: string;
    progress: number;
    createdAt: number;
    /** 点击后回到来源页面并定位该任务；管理员查看他人任务时为空。 */
    href: string;
    /** 仅管理员视图：任务所属用户。 */
    userName: string;
};

type TaskStore = {
    tasks: RunningTask[];
    loading: boolean;
    drawerOpen: boolean;
    refresh: (config: AiConfig) => Promise<void>;
    openDrawer: () => void;
    closeDrawer: () => void;
};

export const useTaskStore = create<TaskStore>((set, get) => ({
    tasks: [],
    loading: false,
    drawerOpen: false,
    openDrawer: () => set({ drawerOpen: true }),
    closeDrawer: () => set({ drawerOpen: false }),
    refresh: async (config) => {
        const user = useUserStore.getState().user;
        if (!user) {
            set({ tasks: [], loading: false });
            return;
        }
        if (!get().tasks.length) set({ loading: true });
        try {
            const tasks = user.role === "admin" ? await collectAdminTasks() : await collectOwnTasks(config);
            set({ tasks, loading: false });
        } catch {
            set({ loading: false });
        }
    },
}));

async function collectOwnTasks(config: AiConfig): Promise<RunningTask[]> {
    const [videoTasks, imageTasks, canvasTasks] = await Promise.all([
        listVideoGenerationTasks(config).catch(() => []),
        listCanvasImageTasks(config, ["image-workbench", "workflow"]).catch(() => []),
        collectCanvasTasks(),
    ]);
    const tasks: RunningTask[] = [];
    for (const task of videoTasks) {
        tasks.push({
            id: `video:${task.id}`,
            kind: "video",
            title: "",
            model: task.model || "",
            status: task.status || "processing",
            progress: task.progress || 0,
            createdAt: parseTaskTime(task.created_at ?? task.createdAt),
            href: `/video?task=${encodeURIComponent(task.id)}`,
            userName: "",
        });
    }
    for (const task of imageTasks) {
        tasks.push({
            id: `image:${task.id}`,
            kind: "image",
            title: task.prompt || "",
            model: task.model || "",
            status: task.status || "processing",
            progress: task.progress || 0,
            createdAt: parseTaskTime(task.created_at ?? task.createdAt),
            href: `/image?task=${encodeURIComponent(task.id)}`,
            userName: "",
        });
    }
    tasks.push(...canvasTasks);
    return dedupeTasks(tasks);
}

async function collectCanvasTasks(): Promise<RunningTask[]> {
    const { useCanvasStore } = await import("@/app/(user)/canvas/stores/use-canvas-store");
    const tasks: RunningTask[] = [];
    for (const project of useCanvasStore.getState().projects) {
        for (const node of project.nodes as CanvasNodeData[]) {
            if (node.metadata?.status !== "loading") continue;
            const kind = canvasNodeTaskKind(node);
            if (!kind) continue;
            tasks.push({
                id: `canvas:${project.id}:${node.id}`,
                kind,
                title: node.metadata?.prompt || node.title || "",
                model: node.metadata?.model || "",
                status: "processing",
                progress: node.metadata?.progress || 0,
                createdAt: node.metadata?.startedAt || 0,
                href: `/canvas/${encodeURIComponent(project.id)}?nodeId=${encodeURIComponent(node.id)}`,
                userName: "",
            });
        }
    }
    return tasks;
}

async function collectAdminTasks(): Promise<RunningTask[]> {
    const token = useUserStore.getState().token;
    if (!token) return [];
    const currentUserId = useUserStore.getState().user?.id || "";
    const items = await fetchAdminRunningTasks(token).catch(() => []);
    return dedupeTasks(
        items.map((item) => {
            // 管理员能看到所有用户的任务，但只有属于自己的任务能跳到详情页，其他用户的任务只展示。
            const mine = Boolean(currentUserId) && item.userId === currentUserId;
            const href = mine && item.kind === "video" ? `/video?task=${encodeURIComponent(item.id)}` : mine && item.kind === "image" ? `/image?task=${encodeURIComponent(item.id)}` : "";
            return {
                id: `${item.kind}:${item.id}`,
                kind: item.kind,
                title: item.prompt || "",
                model: item.model || "",
                status: item.status || "processing",
                progress: item.progress || 0,
                createdAt: parseTaskTime(item.createdAt),
                href,
                userName: item.userName || item.userId,
            };
        }),
    );
}

function canvasNodeTaskKind(node: CanvasNodeData): RunningTaskKind | "" {
    if (node.type === CanvasNodeType.Video) return node.metadata?.videoTaskId || node.metadata?.videoTaskVideoId ? "video" : "";
    if (isCanvasImageNodeType(node.type)) return node.metadata?.imageTaskId ? "image" : "";
    if (node.type === CanvasNodeType.Audio) return node.metadata?.audioTaskId ? "audio" : "";
    return "";
}

function dedupeTasks(tasks: RunningTask[]) {
    const seen = new Set<string>();
    return tasks.filter((task) => {
        if (seen.has(task.id)) return false;
        seen.add(task.id);
        return true;
    });
}

function parseTaskTime(value?: string | number) {
    if (typeof value === "number" && Number.isFinite(value) && value > 0) return value < 1e12 ? value * 1000 : value;
    if (typeof value === "string" && value.trim()) {
        const numeric = Number(value);
        if (Number.isFinite(numeric) && numeric > 0) return numeric < 1e12 ? numeric * 1000 : numeric;
        const parsed = Date.parse(value);
        if (!Number.isNaN(parsed)) return parsed;
    }
    return 0;
}
