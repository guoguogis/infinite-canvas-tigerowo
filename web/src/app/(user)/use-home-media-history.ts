"use client";

import { useEffect, useState } from "react";
import localforage from "localforage";

import { fetchImageGenerationLogs, fetchVideoGenerationLogs } from "@/services/api/generation-logs";
import { resolveMediaUrl } from "@/services/file-storage";
import { resolveImageUrl } from "@/services/image-storage";
import { useUserStore } from "@/stores/use-user-store";

export type HomeMediaHistoryItem = {
    id: string;
    createdAt: number;
    prompt: string;
    model: string;
    status: string;
    url: string;
};

type StoredImageLog = {
    id?: string;
    createdAt?: number;
    prompt?: string;
    model?: string;
    status?: string;
    images?: Array<{ dataUrl?: string; storageKey?: string }>;
    task?: { id?: string; image_url?: string; url?: string; storageKey?: string };
};

type StoredVideoLog = {
    id?: string;
    createdAt?: number;
    prompt?: string;
    model?: string;
    status?: string;
    video?: { url?: string; storageKey?: string };
    task?: { id?: string; task_id?: string };
};

// 与生图 / 视频工作台共用同一份历史记录，实例名与 storeName 必须和两个页面保持一致。
const imageLogStore = localforage.createInstance({ name: "infinite-canvas", storeName: "image_generation_logs" });
const videoLogStore = localforage.createInstance({ name: "infinite-canvas", storeName: "video_generation_logs" });

export function useHomeMediaHistory(limit: number) {
    const token = useUserStore((state) => state.token);
    const isReady = useUserStore((state) => state.isReady);
    const [images, setImages] = useState<HomeMediaHistoryItem[]>([]);
    const [videos, setVideos] = useState<HomeMediaHistoryItem[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        if (!isReady) return;
        let active = true;
        void (async () => {
            const [localImages, localVideos, remoteImages, remoteVideos] = await Promise.all([
                readStoredLogs<StoredImageLog>(imageLogStore),
                readStoredLogs<StoredVideoLog>(videoLogStore),
                loadRemoteLogs(fetchImageGenerationLogs<StoredImageLog>, token),
                loadRemoteLogs(fetchVideoGenerationLogs<StoredVideoLog>, token),
            ]);
            const [imageItems, videoItems] = await Promise.all([
                buildImageItems(mergeByIdentity(remoteImages, localImages, imageLogKey), limit),
                buildVideoItems(mergeByIdentity(remoteVideos, localVideos, videoLogKey), limit),
            ]);
            if (!active) return;
            setImages(imageItems);
            setVideos(videoItems);
            setLoading(false);
        })();
        return () => {
            active = false;
        };
    }, [isReady, token, limit]);

    return { images, videos, loading };
}

async function readStoredLogs<T>(store: LocalForage) {
    const values: T[] = [];
    await store.iterate<T, void>((value) => {
        values.push(value);
    }).catch(() => undefined);
    return values;
}

async function loadRemoteLogs<T>(load: (token: string) => Promise<T[]>, token: string) {
    if (!token) return [];
    try {
        const logs = await load(token);
        return Array.isArray(logs) ? logs : [];
    } catch {
        return [];
    }
}

function mergeByIdentity<T>(remote: T[], local: T[], keyOf: (log: T) => string) {
    const seen = new Set<string>();
    return [...remote, ...local].filter((log) => {
        const key = keyOf(log);
        if (!key) return true;
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
    });
}

function imageLogKey(log: StoredImageLog) {
    return log.id || log.task?.id || "";
}

function videoLogKey(log: StoredVideoLog) {
    return log.task?.id || log.task?.task_id || log.id || "";
}

function recentLogs<T extends { createdAt?: number }>(logs: T[], limit: number) {
    return [...logs].sort((a, b) => (b.createdAt || 0) - (a.createdAt || 0)).slice(0, limit);
}

async function buildImageItems(logs: StoredImageLog[], limit: number) {
    const items = await Promise.all(
        recentLogs(logs, limit).map(async (log): Promise<HomeMediaHistoryItem> => {
            const image = log.images?.find((item) => item.storageKey || item.dataUrl);
            const url = image ? await resolveImageUrl(image.storageKey, image.dataUrl || "").catch(() => "") : await resolveImageUrl(log.task?.storageKey, log.task?.image_url || log.task?.url || "").catch(() => "");
            return { id: log.id || log.task?.id || url, createdAt: log.createdAt || 0, prompt: log.prompt || "", model: log.model || "", status: log.status || "", url };
        }),
    );
    return items.filter((item) => Boolean(item.url));
}

async function buildVideoItems(logs: StoredVideoLog[], limit: number) {
    const items = await Promise.all(
        recentLogs(logs, limit).map(async (log): Promise<HomeMediaHistoryItem> => {
            const url = log.video ? await resolveMediaUrl(log.video.storageKey, log.video.url).catch(() => "") : "";
            return { id: log.id || videoLogKey(log) || url, createdAt: log.createdAt || 0, prompt: log.prompt || "", model: log.model || "", status: log.status || "", url };
        }),
    );
    return items.filter((item) => Boolean(item.url));
}
