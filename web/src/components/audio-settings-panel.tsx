"use client";

import { Select } from "antd";
import { type ReactNode } from "react";

import { GrokTtsVoiceSelect } from "@/components/grok-tts-voice-select";
import { ImageSettingsTheme } from "@/components/image-settings-panel";
import { audioFormatOptions, audioSpeedLabel, audioVoiceOptions, glmTtsFormatOptions, glmTtsVoiceOptions, isGlmTtsModel, normalizeAudioFormatValue, normalizeAudioSpeedValue, normalizeAudioVoiceValue, normalizeGlmTtsFormat, normalizeGlmTtsSpeed, normalizeGlmTtsVoice } from "@/lib/audio-generation";
import { grokTtsFormatOptions, grokTtsLanguageOptions, isGrok2APITtsConfig, normalizeGrokTtsFormat, normalizeGrokTtsLanguage, normalizeGrokTtsSpeed } from "@/lib/grok-tts";
import { doubaoTtsVoiceOptions, isDoubaoTTsConfig, normalizeDoubaoTTsVoice } from "@/lib/doubao-tts";
import { audioVendorForModel, audioVendorOptions } from "@/lib/audio-vendor";
import { isMimoPresetTtsModel, isMimoTtsModel, isMimoVoiceCloneModel, isMimoVoiceDesignModel, mimoTtsFormatOptions, mimoTtsVoiceOptions, normalizeMimoTtsFormat, normalizeMimoTtsVoice } from "@/lib/mimo-tts";
import { isGeminiConfig, isGeminiTtsModel } from "@/lib/gemini";
import { geminiTtsVoiceOptions, normalizeGeminiTtsVoice } from "@/lib/gemini-tts";
import { type CanvasTheme } from "@/lib/canvas-theme";
import type { AiConfig } from "@/stores/use-config-store";

const speedOptions = ["0.75", "1", "1.25", "1.5"];

export type AudioSettingKey = "audioVoice" | "audioFormat" | "audioSpeed" | "audioInstructions" | "grokTtsVoice" | "grokTtsLanguage" | "grokTtsFormat" | "grokTtsSpeed" | "glmTtsVoice" | "glmTtsFormat" | "glmTtsSpeed" | "mimoTtsVoice" | "mimoTtsFormat" | "mimoVoiceDesignPrompt" | "geminiTtsVoice";

type AudioSettingsPanelProps = {
    config: AiConfig;
    onConfigChange: (key: AudioSettingKey, value: string) => void;
    /** 提供后音频设置顶部会出现「厂商」下拉，切换厂商会同时切换音频模型。 */
    onModelChange?: (model: string) => void;
    theme: CanvasTheme;
    showTitle?: boolean;
    className?: string;
};

export function AudioSettingsPanel({ config, onConfigChange, onModelChange, theme, showTitle = true, className = "w-[320px] space-y-4 rounded-2xl px-1 py-0.5" }: AudioSettingsPanelProps) {
    const model = config.model || config.audioModel || "";
    const grok = isGrok2APITtsConfig(config, model);
    const doubao = isDoubaoTTsConfig(config, model);
    const gemini = isGeminiTtsModel(model) && isGeminiConfig(config, model);
    const vendorOptions = onModelChange ? audioVendorOptions(config) : [];

    return (
        <ImageSettingsTheme theme={theme}>
            <div className={className} style={{ color: theme.node.text }} onMouseDown={(event) => event.stopPropagation()}>
                {showTitle ? <div className="text-lg font-semibold">音频设置</div> : null}
                {vendorOptions.length > 1 ? (
                    <SettingGroup title="厂商" color={theme.node.muted}>
                        <Select
                            className="w-full"
                            value={audioVendorForModel(config, model)}
                            options={vendorOptions.map(({ value, label }) => ({ value, label }))}
                            onChange={(value) => {
                                const target = vendorOptions.find((item) => item.value === value);
                                if (target) onModelChange?.(target.model);
                            }}
                        />
                    </SettingGroup>
                ) : null}
                {gemini ? <GeminiAudioSettings config={config} onConfigChange={onConfigChange} theme={theme} /> : isMimoTtsModel(model) ? <MiMoAudioSettings config={config} model={model} onConfigChange={onConfigChange} theme={theme} /> : <AudioSpeechSettings config={config} model={model} glm={isGlmTtsModel(model)} grok={grok} doubao={doubao} onConfigChange={onConfigChange} theme={theme} />}
            </div>
        </ImageSettingsTheme>
    );
}

function GeminiAudioSettings({ config, onConfigChange, theme }: { config: AiConfig; onConfigChange: AudioSettingsPanelProps["onConfigChange"]; theme: CanvasTheme }) {
    return (
        <SettingGroup title="声音" color={theme.node.muted}>
            <ValueSelect value={normalizeGeminiTtsVoice(config.geminiTtsVoice)} options={geminiTtsVoiceOptions} onChange={(value) => onConfigChange("geminiTtsVoice", value)} />
        </SettingGroup>
    );
}

function MiMoAudioSettings({ config, model, onConfigChange, theme }: { config: AiConfig; model: string; onConfigChange: AudioSettingsPanelProps["onConfigChange"]; theme: CanvasTheme }) {
    const format = normalizeMimoTtsFormat(config.mimoTtsFormat);

    return (
        <>
            {isMimoPresetTtsModel(model) ? (
                <SettingGroup title="声音" color={theme.node.muted}>
                    <ValueSelect value={normalizeMimoTtsVoice(config.mimoTtsVoice)} options={mimoTtsVoiceOptions} onChange={(value) => onConfigChange("mimoTtsVoice", value)} />
                </SettingGroup>
            ) : null}
            {isMimoVoiceDesignModel(model) ? (
                <SettingGroup title="音色描述" color={theme.node.muted}>
                    <textarea
                        value={config.mimoVoiceDesignPrompt || ""}
                        placeholder="例如：年轻女性，声音清亮自然，有亲和力。"
                        className="thin-scrollbar h-24 w-full resize-none rounded-xl border bg-transparent px-3 py-2 text-sm leading-5 outline-none"
                        style={{ borderColor: theme.node.stroke, color: theme.node.text }}
                        onChange={(event) => onConfigChange("mimoVoiceDesignPrompt", event.target.value)}
                        onMouseDown={(event) => event.stopPropagation()}
                    />
                </SettingGroup>
            ) : null}
            <SettingGroup title="格式" color={theme.node.muted}>
                <Select className="w-full" value={format} options={[...mimoTtsFormatOptions]} onChange={(value) => onConfigChange("mimoTtsFormat", value)} />
            </SettingGroup>
            {isMimoPresetTtsModel(model) || isMimoVoiceCloneModel(model) ? (
                <SettingGroup title="声音指令" color={theme.node.muted}>
                    <textarea
                        value={config.audioInstructions || ""}
                        placeholder="例如：语速轻快，语气兴奋，结尾略微上扬。"
                        className="thin-scrollbar h-20 w-full resize-none rounded-xl border bg-transparent px-3 py-2 text-sm leading-5 outline-none"
                        style={{ borderColor: theme.node.stroke, color: theme.node.text }}
                        onChange={(event) => onConfigChange("audioInstructions", event.target.value)}
                        onMouseDown={(event) => event.stopPropagation()}
                    />
                </SettingGroup>
            ) : null}
        </>
    );
}

function AudioSpeechSettings({ config, model, glm, grok, doubao, onConfigChange, theme }: { config: AiConfig; model: string; glm: boolean; grok: boolean; doubao: boolean; onConfigChange: AudioSettingsPanelProps["onConfigChange"]; theme: CanvasTheme }) {
    const voice = glm ? normalizeGlmTtsVoice(config.glmTtsVoice) : grok ? config.grokTtsVoice || "eve" : doubao ? normalizeDoubaoTTsVoice(config.audioVoice) : normalizeAudioVoiceValue(config.audioVoice);
    const format = glm ? normalizeGlmTtsFormat(config.glmTtsFormat) : grok ? normalizeGrokTtsFormat(config.grokTtsFormat) : normalizeAudioFormatValue(config.audioFormat);
    const speed = glm ? config.glmTtsSpeed : grok ? config.grokTtsSpeed : config.audioSpeed;
    const voiceOptions = glm ? glmTtsVoiceOptions : doubao ? doubaoTtsVoiceOptions : audioVoiceOptions;
    const formatOptions = glm ? glmTtsFormatOptions : grok ? grokTtsFormatOptions : audioFormatOptions;
    const voiceKey: AudioSettingKey = glm ? "glmTtsVoice" : "audioVoice";
    const formatKey: AudioSettingKey = glm ? "glmTtsFormat" : grok ? "grokTtsFormat" : "audioFormat";
    const speedKey: AudioSettingKey = glm ? "glmTtsSpeed" : grok ? "grokTtsSpeed" : "audioSpeed";
    const normalizeSpeed = (value: string) => (glm ? normalizeGlmTtsSpeed(value) : grok ? normalizeGrokTtsSpeed(value) : normalizeAudioSpeedValue(value));

    return (
        <>
            <SettingGroup title="声音" color={theme.node.muted}>
                {grok ? <GrokTtsVoiceSelect config={config} model={model} value={voice} onChange={(value) => onConfigChange("grokTtsVoice", value)} /> : <ValueSelect value={voice} options={voiceOptions} onChange={(value) => onConfigChange(voiceKey, value)} />}
            </SettingGroup>
            {grok ? (
                <SettingGroup title="语言" color={theme.node.muted}>
                    <Select className="w-full" value={normalizeGrokTtsLanguage(config.grokTtsLanguage)} options={grokTtsLanguageOptions} showSearch optionFilterProp="label" onChange={(value) => onConfigChange("grokTtsLanguage", value)} />
                </SettingGroup>
            ) : null}
            <SettingGroup title="格式" color={theme.node.muted}>
                <Select className="w-full" value={format} options={formatOptions} onChange={(value) => onConfigChange(formatKey, value)} />
            </SettingGroup>
            <SettingGroup title="语速" color={theme.node.muted}>
                <ValueSelect value={speed || "1"} options={speedOptions.map((value) => ({ value, label: audioSpeedLabel(value) }))} onChange={(value) => onConfigChange(speedKey, normalizeSpeed(value))} />
            </SettingGroup>
            {!glm && !grok ? (
                <SettingGroup title="声音指令" color={theme.node.muted}>
                    <textarea
                        value={config.audioInstructions || ""}
                        placeholder="例如：自然、温暖、适合旁白。"
                        className="thin-scrollbar h-20 w-full resize-none rounded-xl border bg-transparent px-3 py-2 text-sm leading-5 outline-none"
                        style={{ borderColor: theme.node.stroke, color: theme.node.text }}
                        onChange={(event) => onConfigChange("audioInstructions", event.target.value)}
                        onMouseDown={(event) => event.stopPropagation()}
                    />
                </SettingGroup>
            ) : null}
        </>
    );
}

// 下拉选择：既能从候选项里选（显示中文名），也能直接输入自定义值（例如音色 ID 或自定义语速）。
function ValueSelect({ value, options, onChange }: { value: string; options: readonly { value: string; label: string }[]; onChange: (value: string) => void }) {
    return (
        <Select
            className="w-full"
            mode="tags"
            maxCount={1}
            value={value ? [value] : []}
            options={[...options]}
            filterOption={(input, option) => `${String(option?.label ?? "")}${String(option?.value ?? "")}`.toLowerCase().includes(input.toLowerCase())}
            onChange={(values: string[]) => onChange(values.length ? values[values.length - 1] : "")}
        />
    );
}

function SettingGroup({ title, color, children }: { title: string; color: string; children: ReactNode }) {
    return (
        <div className="space-y-2.5">
            <div className="text-xs font-medium" style={{ color }}>
                {title}
            </div>
            {children}
        </div>
    );
}
