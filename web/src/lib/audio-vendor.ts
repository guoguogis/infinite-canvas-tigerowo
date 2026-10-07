import { audioVoiceOptions, glmTtsVoiceOptions, isGlmTtsModel } from "@/lib/audio-generation";
import { isAutoDLConfig } from "@/lib/autodl";
import { doubaoTtsVoiceOptions, isDoubaoTTsConfig } from "@/lib/doubao-tts";
import { isGeminiConfig, isGeminiTtsModel } from "@/lib/gemini";
import { geminiTtsVoiceOptions } from "@/lib/gemini-tts";
import { isGrok2APITtsConfig } from "@/lib/grok-tts";
import { isMimoTtsModel, mimoTtsVoiceOptions } from "@/lib/mimo-tts";
import type { AiConfig } from "@/stores/use-config-store";

// 音频厂商决定音色目录，也决定要用哪个音频模型；顺序即匹配优先级，OpenAI 兼容放最后兜底。
export type AudioVendorId = "doubao" | "gemini" | "glm" | "mimo" | "grok" | "autodl" | "openai";

const audioVendors: { id: AudioVendorId; label: string; matches: (config: AiConfig, model: string) => boolean }[] = [
    { id: "doubao", label: "豆包语音合成", matches: (config, model) => isDoubaoTTsConfig(config, model) },
    { id: "gemini", label: "Gemini", matches: (config, model) => isGeminiTtsModel(model) && isGeminiConfig(config, model) },
    { id: "glm", label: "GLM", matches: (_config, model) => isGlmTtsModel(model) },
    { id: "mimo", label: "MiMo", matches: (_config, model) => isMimoTtsModel(model) },
    { id: "grok", label: "Grok", matches: (config, model) => isGrok2APITtsConfig(config, model) },
    { id: "autodl", label: "AutoDL", matches: (config, model) => isAutoDLConfig(config, model) },
    { id: "openai", label: "OpenAI 兼容", matches: () => true },
];

export function audioVendorForModel(config: AiConfig, model: string) {
    return (audioVendors.find((vendor) => vendor.matches(config, model)) || audioVendors[audioVendors.length - 1]).id;
}

// 只列出当前已配置音频模型里真实存在的厂商，避免选到没有模型的厂商。
export function audioVendorOptions(config: AiConfig) {
    const models = config.audioModels || [];
    return audioVendors.flatMap((vendor) => {
        const model = models.find((item) => item && audioVendorForModel(config, item) === vendor.id);
        return model ? [{ value: vendor.id, label: vendor.label, model }] : [];
    });
}

export function audioVendorLabel(config: AiConfig, model: string) {
    const id = audioVendorForModel(config, model);
    return audioVendors.find((vendor) => vendor.id === id)?.label || id;
}

// 厂商对应的音色候选；Grok、AutoDL 需要先取回音色列表或无候选，交给手动输入。
export function audioVendorVoiceOptions(config: AiConfig, model: string) {
    switch (audioVendorForModel(config, model)) {
        case "doubao":
            return [...doubaoTtsVoiceOptions];
        case "glm":
            return [...glmTtsVoiceOptions];
        case "mimo":
            return [...mimoTtsVoiceOptions];
        case "gemini":
            return [...geminiTtsVoiceOptions];
        case "openai":
            return [...audioVoiceOptions];
    }
    return [];
}
