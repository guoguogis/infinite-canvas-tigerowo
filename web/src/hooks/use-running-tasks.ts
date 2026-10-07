"use client";

import { useEffect } from "react";

import { VIDEO_POLL_INTERVAL_MS } from "@/services/api/video";
import { useEffectiveConfig } from "@/stores/use-config-store";
import { useTaskStore } from "@/stores/use-task-store";
import { useUserStore } from "@/stores/use-user-store";

/** 执行中任务轮询间隔：有任务时跟随生成轮询节奏，空闲时放慢。 */
const IDLE_POLL_INTERVAL_MS = 15000;

/** 持续刷新「我的任务」，供右上角角标与任务抽屉共用。 */
export function useRunningTasksPolling() {
    const config = useEffectiveConfig();
    const user = useUserStore((state) => state.user);
    const isReady = useUserStore((state) => state.isReady);
    const refresh = useTaskStore((state) => state.refresh);
    const hasRunning = useTaskStore((state) => state.tasks.length > 0);

    useEffect(() => {
        if (!isReady || !user) return;
        let active = true;
        const tick = () => {
            if (active) void refresh(config);
        };
        tick();
        const timer = window.setInterval(tick, hasRunning ? VIDEO_POLL_INTERVAL_MS : IDLE_POLL_INTERVAL_MS);
        return () => {
            active = false;
            window.clearInterval(timer);
        };
    }, [config, hasRunning, isReady, refresh, user]);
}
