/** 音乐创作台的风格、情绪与参数常量。 */

export const musicStyleTags = ["流行", "国风", "电子", "说唱", "民谣", "爵士", "摇滚", "古风", "轻音乐", "氛围"] as const;

export const musicMoodTags = ["治愈", "燃", "怀旧", "深夜", "欢快", "史诗", "思念", "励志"] as const;

export const musicSceneTags = ["短视频", "Vlog", "广告", "游戏", "纪录片", "电台"] as const;

export const musicLanguages = [
    { label: "中文", value: "Chinese" },
    { label: "英文", value: "English" },
    { label: "纯器乐", value: "Instrumental/Non-vocal" },
] as const;

export const musicVocalGenders = [
    { label: "女声", value: "Female" },
    { label: "男声", value: "Male" },
] as const;

export const musicKeyModes = [
    { label: "小调", value: "Minor" },
    { label: "大调", value: "Major" },
] as const;

/** 各模式的时长区间。纯音乐按火山官方样例为 30-120 秒（计费页另写 ≤60，以后台为准）。 */
export const musicDurationRange = {
    song: { min: 30, max: 240, default: 150 },
    instrumental: { min: 30, max: 120, default: 60 },
} as const;

/** 渠道授权级别，用于在模型卡片上标注来源可信度。 */
export const musicProtocolLicense: Record<string, { label: string; tone: "official" | "reseller" | "reverse" }> = {
    "volc-music": { label: "官方 API", tone: "official" },
};

export function musicLicenseOf(protocol?: string) {
    return musicProtocolLicense[(protocol || "").trim().toLowerCase()] || null;
}

export function formatMusicDuration(milliseconds: number) {
    const seconds = Math.max(0, Math.floor(milliseconds / 1000));
    const minutes = Math.floor(seconds / 60);
    return `${String(minutes).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}
