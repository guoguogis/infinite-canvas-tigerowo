import { apiGet, compactApiParams } from "@/services/api/request";

export type Prompt = {
    id: string;
    title: string;
    coverUrl: string;
    prompt: string;
    tags: string[];
    category: string;
    githubUrl: string;
    videoUrl: string;
    preview: string;
    createdAt: string;
    updatedAt: string;
};

export const ALL_PROMPTS_OPTION = "全部";

export type PromptListResponse = {
    items: Prompt[];
    tags: string[];
    categories: string[];
    total: number;
};

export async function fetchPrompts({ keyword = "", tag = [], category = ALL_PROMPTS_OPTION, page, pageSize, all }: { keyword?: string; tag?: string[]; category?: string; page?: number; pageSize?: number; all?: boolean } = {}) {
    return apiGet<PromptListResponse>(
        "/api/prompts",
        compactApiParams({
            ...(keyword ? { keyword } : {}),
            ...(tag.length ? { tag } : {}),
            ...(category !== ALL_PROMPTS_OPTION ? { category } : {}),
            ...(page ? { page } : {}),
            ...(pageSize ? { pageSize } : {}),
            ...(all ? { all: 1 } : {}),
        }),
    );
}

export function formatPromptDate(value: string) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? "" : new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" }).format(date);
}
