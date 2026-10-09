import { apiDelete, apiGet, apiPost } from "./request";
import { useUserStore } from "@/stores/use-user-store";

/** 音乐生成通常几十秒到几分钟，轮询比视频慢一些。 */
export const MUSIC_POLL_INTERVAL_MS = 10000;
const MUSIC_POLL_TIMEOUT_MS = 15 * 60 * 1000;

export type MusicMode = "song" | "instrumental";

export type MusicTask = {
    id: string;
    object?: string;
    mode?: string;
    title?: string;
    model?: string;
    channelId?: string;
    channelName?: string;
    protocol?: string;
    source?: string;
    source_id?: string;
    status?: string;
    progress?: number;
    prompt?: string;
    lyrics?: string;
    styleTags?: string[];
    duration?: number;
    language?: string;
    vocalGender?: string;
    keyMode?: string;
    tempo?: number | string;
    duration_ms?: number;
    mime_type?: string;
    bytes?: number;
    credits?: number;
    task_id?: string;
    upstream_model?: string;
    created_at?: string;
    updated_at?: string;
    started_at?: string;
    completed_at?: string;
    url?: string;
    audio_url?: string;
    storageKey?: string;
    data?: Array<{ url?: string }>;
    error?: { message?: string } | string;
    error_detail?: string;
};

export type MusicTaskCreateInput = {
    mode: MusicMode;
    model: string;
    channelId?: string;
    userChannelId?: string;
    title?: string;
    prompt?: string;
    lyrics?: string;
    styleTags?: string[];
    duration?: number;
    language?: string;
    vocalGender?: string;
    keyMode?: string;
    tempo?: number;
};

export type MusicTaskListQuery = {
    status?: string;
    keyword?: string;
    page?: number;
    pageSize?: number;
};

export type MusicTaskList = {
    items: MusicTask[];
    total: number;
    page: number;
    pageSize: number;
};

export type CreatedMusicTask = { task: MusicTask; startedAt: number };
export type MusicProgressHandler = (progress: number, task: MusicTask) => void;

export type MusicLyricsInput = { prompt: string; genre?: string; mood?: string; gender?: string; model?: string; channelId?: string };

export class MusicRequestError extends Error {
    detail?: string;

    constructor(message: string, detail?: unknown) {
        super(message);
        this.name = "MusicRequestError";
        this.detail = typeof detail === "string" ? detail : detail ? JSON.stringify(detail).slice(0, 500) : undefined;
    }
}

/** 完成任务的可播放地址：后端已把音频转存到自有存储并返回可用地址。 */
export function musicTaskAudioUrl(task: MusicTask) {
    return task.audio_url || task.url || task.data?.[0]?.url || "";
}

export function musicTaskStatusText(status?: string) {
    switch ((status || "").toLowerCase()) {
        case "completed":
            return "已完成";
        case "failed":
            return "生成失败";
        case "processing":
            return "执行中";
        default:
            return "排队中";
    }
}

export function isMusicTaskCompleted(status?: string) {
    return ["completed", "complete", "done", "succeeded", "success"].includes((status || "").toLowerCase());
}

export function isMusicTaskFailed(status?: string) {
    return ["failed", "fail", "error", "cancelled", "canceled"].includes((status || "").toLowerCase());
}

export function isMusicTaskRunning(status?: string) {
    return !isMusicTaskCompleted(status) && !isMusicTaskFailed(status);
}

export function musicTaskErrorMessage(task: MusicTask) {
    if (typeof task.error === "string") return task.error;
    return task.error?.message || task.error_detail || "音乐生成失败";
}

export async function createMusicGenerationTask(input: MusicTaskCreateInput): Promise<CreatedMusicTask> {
    const task = await apiPost<MusicTask>("/api/v1/music-tasks", input, requireToken());
    if (!task?.id) throw new MusicRequestError("音乐任务创建失败", task);
    return { task, startedAt: Date.now() };
}

export async function listMusicTasks(query: MusicTaskListQuery = {}): Promise<MusicTaskList> {
    const payload = await apiGet<MusicTaskList>("/api/v1/music-tasks", { ...query }, requireToken());
    if (!payload) return { items: [], total: 0, page: 1, pageSize: 0 };
    return { items: Array.isArray(payload.items) ? payload.items : [], total: payload.total || 0, page: payload.page || 1, pageSize: payload.pageSize || 0 };
}

export async function getMusicTask(id: string): Promise<MusicTask> {
    return apiGet<MusicTask>(`/api/v1/music-tasks/${encodeURIComponent(id)}`, undefined, requireToken());
}

export async function deleteMusicTask(id: string): Promise<void> {
    await apiDelete(`/api/v1/music-tasks/${encodeURIComponent(id)}`, requireToken());
}

export async function generateMusicLyrics(input: MusicLyricsInput): Promise<string> {
    const payload = await apiPost<{ lyrics?: string }>("/api/v1/music-tasks/lyrics", input, requireToken());
    return payload?.lyrics || "";
}

/**
 * 轮询到任务进入终态。上游调用在服务端后台轮询器里进行，
 * 这里只负责按节奏查详情并回调进度。
 */
export async function pollCreatedMusicTask(task: MusicTask, options: { onProgress?: MusicProgressHandler; startedAt?: number } = {}): Promise<MusicTask> {
    const startedAt = options.startedAt || Date.now();
    let current = task;
    while (isMusicTaskRunning(current.status)) {
        if (Date.now() - startedAt > MUSIC_POLL_TIMEOUT_MS) {
            throw new MusicRequestError("音乐生成超时，请稍后在「我的音乐」中查看", current);
        }
        await delay(MUSIC_POLL_INTERVAL_MS);
        const next = await getMusicTask(current.id);
        current = next;
        options.onProgress?.(Math.max(0, Math.min(100, next.progress || 0)), next);
    }
    if (isMusicTaskFailed(current.status)) throw new MusicRequestError(musicTaskErrorMessage(current), current);
    return current;
}

function requireToken() {
    const token = readToken();
    if (!token) throw new MusicRequestError("未登录或登录已过期");
    return token;
}

function readToken() {
    return useUserStore.getState().token || "";
}

function delay(milliseconds: number) {
    return new Promise((resolve) => setTimeout(resolve, milliseconds));
}
