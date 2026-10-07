"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";

import { AppTopNav } from "@/components/layout/app-top-nav";
import { fetchUserConfig } from "@/services/api/user-config";
import { useUserStore } from "@/stores/use-user-store";

// 只有这些入口允许未登录访问；其余页面统一要求登录。
const publicPrefixes = ["/login", "/register", "/tokendance/callback"];

export default function UserLayout({ children }: { children: ReactNode }) {
    const pathname = usePathname();
    const router = useRouter();
    const user = useUserStore((state) => state.user);
    const isReady = useUserStore((state) => state.isReady);
    const wasLoggedOutRef = useRef(false);
    const isPublicPage = publicPrefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
    const needsAuth = !isPublicPage;
    // 登录/注册入口只展示主窗口，不显示顶部菜单栏。
    const isAuthEntry = pathname === "/login" || pathname.startsWith("/login/") || pathname === "/register" || pathname.startsWith("/register/");

    useEffect(() => {
        if (!isReady || !needsAuth || user) return;
        router.replace(`/login?redirect=${encodeURIComponent(pathname)}`);
    }, [isReady, needsAuth, pathname, router, user]);

    useEffect(() => {
        if (!isReady) return;
        if (!user) {
            wasLoggedOutRef.current = true;
            return;
        }
        const syncCanvasAfterLogin = wasLoggedOutRef.current;
        const token = useUserStore.getState().token;
        if (!token) return;
        wasLoggedOutRef.current = false;
        fetchUserConfig(token).then(async (config) => {
            const syncEnabled = config.syncCapabilities?.userData === true;
            const { useCanvasStore } = await import("@/app/(user)/canvas/stores/use-canvas-store");
            const canvasStore = useCanvasStore.getState();
            canvasStore.setSyncEnabled(syncEnabled);
            if (
                syncCanvasAfterLogin &&
                syncEnabled &&
                canvasStore.hydrated
            ) {
                void canvasStore.syncWithRemote(token, true);
            }
            const { useAssetStore } = await import("@/stores/use-asset-store");
            void useAssetStore.getState().hydrateAccountAssets(token, syncEnabled);
        }).catch(() => { });
    }, [isReady, user]);

    return (
        <div className="flex h-dvh flex-col overflow-hidden bg-background text-foreground">
            {isAuthEntry ? null : <AppTopNav />}
            <div className="min-h-0 flex-1 overflow-hidden">{needsAuth && (!isReady || !user) ? null : children}</div>
        </div>
    );
}
