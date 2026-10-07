"use client";

import { useEffect, useRef } from "react";

import { fetchUserConfig } from "@/services/api/user-config";
import { useUserStore } from "@/stores/use-user-store";

/**
 * 浏览器本地数据按账号命名空间切分，而 zustand persist 在模块加载时就已经读取过一次，
 * 因此登录用户确定后需要重新水合到当前账号的命名空间：既处理同一浏览器换号登录，
 * 也保证首次进入时读到的是自己的数据。
 */
export function useUserLocalStorageScope() {
    const userId = useUserStore((state) => state.user?.id || "");
    const isReady = useUserStore((state) => state.isReady);
    const appliedRef = useRef<string | null>(null);

    useEffect(() => {
        if (!isReady || appliedRef.current === userId) return;
        appliedRef.current = userId;
        void (async () => {
            const token = useUserStore.getState().token;
            const [{ useCanvasStore }, { useAssetStore }] = await Promise.all([import("@/app/(user)/canvas/stores/use-canvas-store"), import("@/stores/use-asset-store")]);
            await useCanvasStore.persist.rehydrate();
            await useAssetStore.persist.rehydrate();
            if (!userId || !token) return;
            const config = await fetchUserConfig(token).catch(() => null);
            const syncEnabled = config?.syncCapabilities?.userData === true;
            const canvasStore = useCanvasStore.getState();
            canvasStore.setSyncEnabled(syncEnabled);
            // 本地画布为空时（例如首次按账号隔离后还没有本地数据）从账号补一次，避免看起来像数据丢失。
            if (syncEnabled && !canvasStore.projects.length) void canvasStore.syncWithRemote(token, true);
        })();
    }, [isReady, userId]);
}
