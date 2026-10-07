"use client";

import localforage from "localforage";

import { AUTH_TOKEN_KEY } from "@/services/api/auth";
import { useUserStore } from "@/stores/use-user-store";

// 浏览器本地数据按登录账号隔离：每个账号使用独立的 object store，
// 未登录时回退到 guest 命名空间，避免同一台电脑上切换账号看到上一个账号的数据。
const DB_NAME = "infinite-canvas";
const instances = new Map<string, LocalForage>();

/**
 * 从已持久化的登录态里同步解析用户 id。
 * zustand persist 在模块加载时就会读取一次本地数据，那时 useUserStore 还没 hydrate，
 * 因此不能只依赖 store 里的 user，否则会读到 guest 命名空间。
 */
function persistedUserId() {
    if (typeof window === "undefined") return "";
    try {
        const raw = window.localStorage.getItem(AUTH_TOKEN_KEY);
        const token = raw ? JSON.parse(raw)?.state?.token : "";
        const payload = typeof token === "string" && token.includes(".") ? token.split(".")[1] : "";
        if (!payload) return "";
        const normalized = payload.replace(/-/g, "+").replace(/_/g, "/");
        const decoded = JSON.parse(window.atob(normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=")));
        return String(decoded?.userId || decoded?.sub || "");
    } catch {
        return "";
    }
}

function currentScope() {
    const userId = useUserStore.getState().user?.id || persistedUserId();
    return userId ? `u_${userId}` : "guest";
}

function resolveInstance(storeName: string): LocalForage {
    const scopedName = `${currentScope()}__${storeName}`;
    let instance = instances.get(scopedName);
    if (!instance) {
        instance = localforage.createInstance({ name: DB_NAME, storeName: scopedName });
        instances.set(scopedName, instance);
    }
    return instance;
}

/**
 * 获取当前账号命名空间下的 localforage 实例。
 * 返回惰性代理，方法调用时才按当前登录用户解析真实实例，因此可以安全地在模块顶层创建。
 */
export function userLocalStore(storeName: string): LocalForage {
    return new Proxy({} as LocalForage, {
        get(_target, property) {
            const instance = resolveInstance(storeName);
            const value = Reflect.get(instance as object, property);
            return typeof value === "function" ? value.bind(instance) : value;
        },
        has(_target, property) {
            return Reflect.has(resolveInstance(storeName) as object, property);
        },
    }) as LocalForage;
}
