"use client";

import { Button, Input, Tag, theme } from "antd";
import { saveAs } from "file-saver";
import { ClipboardPaste, Download, LoaderCircle, Music2, PanelBottom, PanelLeft, SlidersHorizontal, Sparkles, Trash2 } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { AudioPlayer } from "@/components/music/audio-player";
import { modelCreditCost } from "@/constant/credits";
import { useCopyText } from "@/hooks/use-copy-text";
import { downloadRemoteMedia } from "@/services/file-storage";
import {
    createMusicGenerationTask,
    deleteMusicTask,
    generateMusicLyrics,
    isMusicTaskFailed,
    isMusicTaskRunning,
    listMusicTasks,
    musicTaskAudioUrl,
    musicTaskErrorMessage,
    musicTaskStatusText,
    pollCreatedMusicTask,
    type MusicMode,
    type MusicTask,
} from "@/services/api/music";
import { formatMusicDuration, musicDurationRange, musicKeyModes, musicLanguages, musicLicenseOf, musicMoodTags, musicStyleTags, musicVocalGenders, type musicProtocolLicense } from "@/lib/music-styles";
import { cn } from "@/lib/utils";
import { useAssetStore } from "@/stores/use-asset-store";
import { selectableModelsByCapability, useConfigStore, useEffectiveConfig } from "@/stores/use-config-store";

const HISTORY_PAGE_SIZE = 20;
type WorkbenchLayout = "side" | "bottom";
/** 与视频创作台分开持久化，避免两个工作台互相影响。 */
const MUSIC_WORKBENCH_LAYOUT_KEY = "infinite-canvas:music-workbench-layout";

export default function MusicPage() {
    return (
        <Suspense fallback={null}>
            <MusicPageContent />
        </Suspense>
    );
}

function MusicPageContent() {
    const searchParams = useSearchParams();
    const focusTaskId = searchParams.get("task") || "";
    const config = useEffectiveConfig();
    const openConfigDialog = useConfigStore((state) => state.openConfigDialog);
    const addAsset = useAssetStore((state) => state.addAsset);
    const copyText = useCopyText();

    const musicModels = useMemo(() => selectableModelsByCapability(config, "music"), [config]);
    const [model, setModel] = useState("");
    const [mode, setMode] = useState<MusicMode>("song");
    const [title, setTitle] = useState("");
    const [prompt, setPrompt] = useState("");
    const [lyrics, setLyrics] = useState("");
    const [styleTags, setStyleTags] = useState<string[]>(["流行", "国风"]);
    const [moodTags, setMoodTags] = useState<string[]>(["治愈"]);
    const [duration, setDuration] = useState<number>(musicDurationRange.song.default);
    const [language, setLanguage] = useState<string>(musicLanguages[0].value);
    const [vocalGender, setVocalGender] = useState<string>(musicVocalGenders[0].value);
    const [keyMode, setKeyMode] = useState<string>(musicKeyModes[0].value);
    const [tempo, setTempo] = useState("88");

    const [sessionTasks, setSessionTasks] = useState<MusicTask[]>([]);
    const [history, setHistory] = useState<MusicTask[]>([]);
    const [loading, setLoading] = useState(true);
    const [submitting, setSubmitting] = useState(false);
    const [batchCount, setBatchCount] = useState(1);
    const [error, setError] = useState("");
    const [workbenchLayout, setWorkbenchLayoutState] = useState<WorkbenchLayout>("side");
    const [bottomSettingsCollapsed, setBottomSettingsCollapsed] = useState(true);
    const pollingRef = useRef<Set<string>>(new Set());
    /** 同一次提交产生的任务归为一批，用于 A/B 试听（后端一次调用只出一个任务）。 */
    const batchRef = useRef<Record<string, string>>({});

    const instrumental = mode === "instrumental";
    const range = instrumental ? musicDurationRange.instrumental : musicDurationRange.song;

    useEffect(() => {
        setDuration(instrumental ? musicDurationRange.instrumental.default : musicDurationRange.song.default);
    }, [instrumental]);

    useEffect(() => {
        if (!model && musicModels.length) setModel(musicModels[0]);
    }, [model, musicModels]);

    /** 布局偏好与视频创作台分开存储。 */
    useEffect(() => {
        try {
            const storedLayout = window.localStorage?.getItem(MUSIC_WORKBENCH_LAYOUT_KEY);
            if (storedLayout === "side" || storedLayout === "bottom") setWorkbenchLayoutState(storedLayout);
        } catch {
            // localStorage 不可用时保持默认布局。
        }
    }, []);

    const setWorkbenchLayout = (layout: WorkbenchLayout) => {
        setWorkbenchLayoutState(layout);
        try {
            window.localStorage?.setItem(MUSIC_WORKBENCH_LAYOUT_KEY, layout);
        } catch {
            // 无法持久化时仅保留当前会话的内存布局。
        }
    };

    const refreshHistory = useCallback(async () => {
        try {
            const payload = await listMusicTasks({ pageSize: HISTORY_PAGE_SIZE });
            setHistory(payload.items);
        } catch {
            // 未登录或后端未启动时不阻塞页面。
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        void refreshHistory();
    }, [refreshHistory]);

    /** 任务进入终态后刷新历史；执行中则按节奏轮询。 */
    const followTask = useCallback(
        (task: MusicTask) => {
            if (pollingRef.current.has(task.id)) return;
            pollingRef.current.add(task.id);
            void pollCreatedMusicTask(task, {
                onProgress: (progress, next) => {
                    setSessionTasks((items) => items.map((item) => (item.id === next.id ? { ...item, ...next, progress } : item)));
                },
            })
                .then((finished) => {
                    setSessionTasks((items) => items.map((item) => (item.id === finished.id ? finished : item)));
                    void refreshHistory();
                })
                .catch((pollError: Error) => {
                    setError(pollError.message);
                    setSessionTasks((items) => items.map((item) => (item.id === task.id ? { ...item, status: "failed", error: { message: pollError.message } } : item)));
                    void refreshHistory();
                })
                .finally(() => {
                    pollingRef.current.delete(task.id);
                });
        },
        [refreshHistory],
    );

    /** 页面加载时接管尚未完成的任务。 */
    useEffect(() => {
        const running = history.filter((task) => isMusicTaskRunning(task.status));
        if (!running.length) return;
        setSessionTasks((items) => (items.length ? items : running));
        running.forEach((task) => followTask(task));
    }, [history, followTask]);

    const channelId = useMemo(() => {
        const channels = config.channelMode === "remote" ? config.publicChannels : config.localChannels;
        const matched = channels.find((channel) => (channel.models || []).includes(model));
        return matched?.id || "";
    }, [config.channelMode, config.localChannels, config.publicChannels, model]);

    const cost = modelCreditCost(useConfigStore((state) => state.publicSettings?.modelChannel.modelCosts), model);

    const toggleTag = (value: string, current: string[], onChange: (next: string[]) => void) => {
        onChange(current.includes(value) ? current.filter((item) => item !== value) : [...current, value]);
    };

    /** 读取剪贴板并写入指定输入框。 */
    const pasteInto = async (apply: (text: string) => void) => {
        try {
            const text = await navigator.clipboard?.readText();
            if (text?.trim()) apply(text);
            else setError("剪贴板没有可用文本");
        } catch {
            setError("读取剪贴板失败，请检查浏览器权限");
        }
    };

    const submit = async () => {
        setError("");
        if (!model) {
            setError("请先在设置中配置音乐模型与渠道");
            return;
        }
        if (!channelId) {
            setError("未找到该模型所属渠道，请在设置中检查渠道配置");
            return;
        }
        setSubmitting(true);
        try {
            const batchId = `${Date.now()}`;
            const channelPayload = config.channelMode === "remote" ? { channelId } : { userChannelId: channelId };
            const createdTasks: MusicTask[] = [];
            for (let index = 0; index < batchCount; index += 1) {
                const created = await createMusicGenerationTask({
                    mode,
                    model,
                    ...channelPayload,
                    title: title.trim() || undefined,
                    prompt: prompt.trim() || undefined,
                    lyrics: instrumental ? undefined : lyrics.trim() || undefined,
                    styleTags,
                    duration,
                    language: instrumental ? "Instrumental/Non-vocal" : language,
                    vocalGender: instrumental ? undefined : vocalGender,
                    keyMode,
                    tempo: Number(tempo) || undefined,
                });
                batchRef.current[created.task.id] = batchId;
                createdTasks.push(created.task);
            }
            setSessionTasks((items) => [...createdTasks, ...items]);
            createdTasks.forEach((task) => followTask(task));
        } catch (submitError) {
            const message = (submitError as Error).message || "";
            setError(message.includes("渠道") ? "还没有可用的音乐渠道，请先在设置中添加「火山引擎音乐」、「腾讯云 TokenHub」或「MiniMax 音乐」渠道并配置模型" : message);
        } finally {
            setSubmitting(false);
        }
    };

    const writeLyrics = async () => {
        setError("");
        if (!prompt.trim()) {
            setError("请先填写风格描述，再让 AI 写词");
            return;
        }
        try {
            const generated = await generateMusicLyrics({ prompt: prompt.trim(), genre: styleTags.join(","), mood: moodTags.join(","), gender: vocalGender, model, channelId });
            if (generated) setLyrics(generated);
            else setError("该渠道未返回歌词");
        } catch (lyricsError) {
            setError((lyricsError as Error).message);
        }
    };

    const removeTask = async (task: MusicTask) => {
        try {
            await deleteMusicTask(task.id);
            setSessionTasks((items) => items.filter((item) => item.id !== task.id));
            setHistory((items) => items.filter((item) => item.id !== task.id));
        } catch (deleteError) {
            setError((deleteError as Error).message);
        }
    };

    const downloadTask = async (task: MusicTask) => {
        const url = musicTaskAudioUrl(task);
        if (!url) return;
        try {
            const blob = await downloadRemoteMedia(url);
            saveAs(blob, `${task.title || "music"}.${(task.mime_type || "audio/mpeg").includes("wav") ? "wav" : "mp3"}`);
        } catch (downloadError) {
            setError((downloadError as Error).message);
        }
    };

    const saveToAssets = (task: MusicTask) => {
        const url = musicTaskAudioUrl(task);
        if (!url) return;
        addAsset({
            kind: "audio",
            title: task.title || "音乐作品",
            coverUrl: "",
            tags: task.styleTags || [],
            source: "音乐创作台",
            data: { url, storageKey: task.storageKey, bytes: task.bytes, mimeType: task.mime_type || "audio/mpeg", durationMs: task.duration_ms },
            metadata: { source: "music-page", prompt: task.prompt || "", model: task.model || "" },
        });
    };

    /** 同一次提交里完成的多首可以 A/B 对比试听。 */
    const candidateGroups = useMemo(() => {
        const groups = new Map<string, MusicTask[]>();
        sessionTasks.forEach((task) => {
            const batchId = batchRef.current[task.id];
            if (!batchId) return;
            groups.set(batchId, [...(groups.get(batchId) || []), task]);
        });
        return new Map([...groups].filter(([, group]) => group.length > 1));
    }, [sessionTasks]);

    /** 每个批次只在第一张卡片内展示候选，避免重复渲染。 */
    const candidatesFor = (task: MusicTask) => {
        const group = candidateGroups.get(batchRef.current[task.id] || "");
        if (!group?.length) return [];
        return group[0].id === task.id ? group : [];
    };

    const running = sessionTasks.filter((task) => isMusicTaskRunning(task.status));

    /** 本次会话结果 + 历史合并为同一个列表：按 task.id 去重，本次会话的任务排在前面。 */
    const mergedTasks = useMemo(() => {
        const seen = new Set(sessionTasks.map((task) => task.id));
        return [...sessionTasks, ...history.filter((task) => !seen.has(task.id))];
    }, [history, sessionTasks]);

    /** 与视频创作台一致：有输入才可用。纯音乐模式只看描述，歌曲模式描述或歌词任一非空即可。 */
    const canGenerate = Boolean(prompt.trim() || (mode === "song" && lyrics.trim()));

    /** 底部布局与侧边布局共用的输入区动作按钮。 */
    const inputActions = (onPaste: () => void, onClear: () => void, clearDisabled: boolean, extra?: ReactNode) => (
        <>
            <Button title="读取剪贴板" size="small" icon={<ClipboardPaste className="size-3.5" />} onClick={onPaste}>
                读取剪贴板
            </Button>
            <Button title="清空" size="small" icon={<Trash2 className="size-3.5" />} disabled={clearDisabled} onClick={onClear}>
                清空
            </Button>
            {extra}
        </>
    );

    const lyricsSection = (
        <MusicSection title="歌词">
            <div className="flex flex-wrap gap-1">
                {inputActions(
                    () => void pasteInto(setLyrics),
                    () => setLyrics(""),
                    !lyrics.trim(),
                    <Button size="small" icon={<Sparkles className="size-3.5" />} onClick={() => void writeLyrics()}>
                        AI 写词
                    </Button>,
                )}
            </div>
            <textarea
                rows={6}
                value={lyrics}
                onChange={(event) => setLyrics(event.target.value)}
                placeholder={"[Verse]\n窗外的雨敲着旧站台\n[Chorus]\n我把那年夏天折成纸船"}
                className="w-full rounded-lg border border-stone-200 bg-transparent px-3 py-2 text-[13px] outline-none focus:border-stone-400 dark:border-stone-700"
            />
            <p className="text-[11px] text-stone-400">支持 [Verse] / [Chorus] 结构标签</p>
        </MusicSection>
    );

    const promptSection = (
        <MusicSection title={instrumental ? "音乐描述" : "风格描述"}>
            <div className="flex flex-wrap gap-1">{inputActions(() => void pasteInto(setPrompt), () => setPrompt(""), !prompt.trim())}</div>
            <input
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder={instrumental ? "深夜城市电子氛围，缓慢铺底" : "国风流行，古筝与电子鼓，清亮女声"}
                className="w-full rounded-lg border border-stone-200 bg-transparent px-3 py-2 text-[13px] outline-none focus:border-stone-400 dark:border-stone-700"
            />
            <input
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder="作品标题（可不填）"
                className="w-full rounded-lg border border-stone-200 bg-transparent px-3 py-2 text-[13px] outline-none focus:border-stone-400 dark:border-stone-700"
            />
        </MusicSection>
    );

    const tagChips = (tags: readonly string[], selected: string[], onChange: (next: string[]) => void) => (
        <div className="flex flex-wrap gap-1.5">
            {tags.map((tag) => (
                <Tag.CheckableTag key={tag} checked={selected.includes(tag)} className={cn("prompt-filter-tag", selected.includes(tag) && "is-active")} onChange={() => toggleTag(tag, selected, onChange)}>
                    {tag}
                </Tag.CheckableTag>
            ))}
        </div>
    );

    const styleSection = <MusicSection title="风格标签">{tagChips(musicStyleTags, styleTags, setStyleTags)}</MusicSection>;
    const moodSection = <MusicSection title="情绪 / 场景">{tagChips(musicMoodTags, moodTags, setMoodTags)}</MusicSection>;

    const durationSection = (
        <MusicSection title="时长">
            <div className="flex items-center justify-between">
                <span className="text-xs text-stone-500 dark:text-stone-400">目标时长</span>
                <span className="text-xs font-medium tabular-nums">{duration} 秒</span>
            </div>
            <input
                type="range"
                min={range.min}
                max={range.max}
                step={5}
                value={duration}
                onChange={(event) => setDuration(Number(event.target.value))}
                aria-label="时长"
                className="h-1 w-full cursor-pointer appearance-none rounded-full bg-stone-200 accent-sky-500 dark:bg-stone-700"
            />
            <div className="flex justify-between text-[11px] text-stone-400">
                <span>{range.min} 秒</span>
                <span>{range.max} 秒</span>
            </div>
            {instrumental ? <p className="rounded-md border border-amber-200 bg-amber-50 px-2 py-1.5 text-[11px] text-amber-700 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300">当前模型的纯音乐时长上限为 {range.max} 秒</p> : null}
        </MusicSection>
    );

    const paramSection = (
        <MusicSection title="参数">
            <div className="grid grid-cols-2 gap-3">
                <div>
                    <label className="mb-1.5 block text-xs text-stone-500 dark:text-stone-400">语言</label>
                    <select value={instrumental ? "Instrumental/Non-vocal" : language} disabled={instrumental} onChange={(event) => setLanguage(event.target.value)} className="h-11 w-full rounded-xl border border-stone-200 bg-background px-3 text-sm text-stone-900 outline-none disabled:opacity-50 dark:border-stone-800 dark:text-stone-100">
                        {musicLanguages.map((item) => (
                            <option key={item.value} value={item.value}>
                                {item.label}
                            </option>
                        ))}
                    </select>
                </div>
                <div>
                    <label className="mb-1.5 block text-xs text-stone-500 dark:text-stone-400">人声</label>
                    <select value={vocalGender} disabled={instrumental} onChange={(event) => setVocalGender(event.target.value)} className="h-11 w-full rounded-xl border border-stone-200 bg-background px-3 text-sm text-stone-900 outline-none disabled:opacity-50 dark:border-stone-800 dark:text-stone-100">
                        {musicVocalGenders.map((item) => (
                            <option key={item.value} value={item.value}>
                                {item.label}
                            </option>
                        ))}
                    </select>
                </div>
                <div>
                    <label className="mb-1.5 block text-xs text-stone-500 dark:text-stone-400">调式</label>
                    <select value={keyMode} onChange={(event) => setKeyMode(event.target.value)} className="h-11 w-full rounded-xl border border-stone-200 bg-background px-3 text-sm text-stone-900 outline-none disabled:opacity-50 dark:border-stone-800 dark:text-stone-100">
                        {musicKeyModes.map((item) => (
                            <option key={item.value} value={item.value}>
                                {item.label}
                            </option>
                        ))}
                    </select>
                </div>
                <div>
                    <label className="mb-1.5 block text-xs text-stone-500 dark:text-stone-400">速度 BPM</label>
                    <input value={tempo} onChange={(event) => setTempo(event.target.value.replace(/[^\d]/g, ""))} className="h-11 w-full rounded-xl border border-stone-200 bg-background px-3 text-sm text-stone-900 outline-none disabled:opacity-50 dark:border-stone-800 dark:text-stone-100" />
                </div>
            </div>
        </MusicSection>
    );

    const modelSection = (
        <MusicSection title="模型">
            {musicModels.length ? (
                <div className="flex flex-col gap-2">
                    {musicModels.map((item) => (
                        <MusicModelCard key={item} name={item} channelName={channelNameOfModel(config, item)} license={musicLicenseOf(protocolOfModel(config, item))} selected={model === item} onSelect={() => setModel(item)} />
                    ))}
                </div>
            ) : (
                <button type="button" onClick={() => openConfigDialog(false)} className="w-full rounded-lg border border-dashed border-stone-300 px-3 py-3 text-xs text-stone-500 transition hover:border-stone-400 dark:border-stone-700 dark:text-stone-400">
                    还没有可用的音乐模型，去设置里添加渠道与模型
                </button>
            )}
        </MusicSection>
    );

    const countSection = (
        <MusicSection title="生成数量">
            <div className="inline-flex shrink-0 rounded-lg border border-stone-200 bg-stone-50 p-1 dark:border-stone-800 dark:bg-stone-900">
                {[1, 2].map((value) => (
                    <Button key={value} size="small" type={batchCount === value ? "primary" : "text"} onClick={() => setBatchCount(value)}>
                        {value} 首
                    </Button>
                ))}
            </div>
            <p className="text-[11px] text-stone-400">选择 2 首会连续提交两次，完成后可在同一张卡片里 A/B 试听。</p>
        </MusicSection>
    );

    const modeSection = (
        <MusicSection title="创作模式">
            <div className="flex shrink-0 rounded-lg border border-stone-200 bg-stone-50 p-1 dark:border-stone-800 dark:bg-stone-900">
                {(["song", "instrumental"] as MusicMode[]).map((value) => (
                    <Button key={value} size="small" type={mode === value ? "primary" : "text"} onClick={() => setMode(value)}>
                        {value === "song" ? "歌曲" : "纯音乐"}
                    </Button>
                ))}
            </div>
        </MusicSection>
    );

    const errorText = error ? <p className="rounded-md border border-red-200 bg-red-50 px-2 py-1.5 text-xs text-red-600 dark:border-red-900 dark:bg-red-950 dark:text-red-300">{error}</p> : null;

    const generateButton = (
        <Button type="primary" size="large" block icon={<Sparkles className="size-4" />} loading={submitting} disabled={!canGenerate} onClick={() => void submit()}>
            {running.length ? `生成中（${running.length}）` : "生成音乐"}
        </Button>
    );

    const resultPanelHeader = (
        <div className="mb-4 flex items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-2">
                <Music2 className="size-4 shrink-0 text-stone-400" />
                <h2 className="truncate text-xl font-semibold">我的音乐</h2>
                <Tag className="m-0">{mergedTasks.length}</Tag>
                {running.length ? <Tag className="m-0 px-2 py-1">{running.length} 个生成中</Tag> : null}
            </div>
            <div className="flex shrink-0 items-center gap-2">
                <Button size="small" onClick={() => void refreshHistory()}>
                    刷新
                </Button>
            </div>
        </div>
    );

    /** 本次会话结果 + 历史合并渲染在同一个网格；执行中的任务保留进度条。 */
    const resultPanelBody = mergedTasks.length ? (
        <div className="grid gap-3 md:grid-cols-2 2xl:grid-cols-3">
            {mergedTasks.map((task) => (
                <MusicResultCard
                    key={task.id}
                    task={task}
                    focused={focusTaskId === task.id}
                    candidates={candidatesFor(task)}
                    onDownload={(item) => void downloadTask(item)}
                    onSaveAsset={saveToAssets}
                    onCopyLink={(item) => copyText(musicTaskAudioUrl(item), "音频地址已复制")}
                    onRemove={(item) => void removeTask(item)}
                    onRegenerate={() => void submit()}
                />
            ))}
        </div>
    ) : loading ? (
        <p className="py-10 text-center text-xs text-stone-400">正在加载…</p>
    ) : (
        <div className="flex min-h-[320px] flex-col items-center justify-center rounded-lg border border-dashed border-stone-300 text-center dark:border-stone-700 lg:min-h-[560px]">
            <Music2 className="mb-4 size-11 text-stone-400" />
            <p className="text-sm text-stone-400">填写描述或歌词，生成你的第一首音乐</p>
        </div>
    );

    /** 合规声明与是否有结果无关，始终显示。 */
    const complianceNote = <p className="mt-4 rounded-lg border border-stone-200 px-4 py-3 text-xs text-stone-500 dark:border-stone-800 dark:text-stone-400">AI 生成内容已按法规添加标识与音频水印，不可去除。生成物仅供创作参考，商用前请确认所选模型的授权条款。</p>;

    return (
        <div className="flex h-full flex-col overflow-hidden bg-stone-50 text-stone-900 dark:bg-stone-950 dark:text-stone-100">
            <main className={`${workbenchLayout === "side" ? "grid grid-cols-1 lg:grid-cols-[420px_minmax(0,1fr)]" : "relative flex flex-col"} min-h-0 flex-1 gap-3 overflow-y-auto p-3 lg:overflow-hidden`}>
                {workbenchLayout === "side" ? (
                    <>
                        {/* 左：仅创作参数（历史与本次结果统一在右侧面板） */}
                        <div className="flex min-h-[420px] flex-col overflow-hidden rounded-lg border border-stone-200 bg-card shadow-sm dark:border-stone-800 lg:min-h-0">
                            <MusicWorkbenchHeader currentLayout={workbenchLayout} onLayoutChange={setWorkbenchLayout} />
                            <div className="thin-scrollbar min-h-0 flex-1 space-y-3 overflow-y-auto px-4 pb-3">
                                {modeSection}
                                {instrumental ? null : lyricsSection}
                                {promptSection}
                                {styleSection}
                                {moodSection}
                                {durationSection}
                                {paramSection}
                                {modelSection}
                                {countSection}
                            </div>
                            <div className="shrink-0 border-t border-stone-200 p-4 dark:border-stone-800">
                                {generateButton}
                                <p className="mt-2 text-center text-xs text-stone-400">
                                    预计消耗 <span className="font-medium text-stone-600 dark:text-stone-300">{cost || "—"}</span> 算力点
                                </p>
                                {errorText ? <div className="mt-2">{errorText}</div> : null}
                            </div>
                        </div>

                        {/* 右：我的音乐（本次结果 + 历史合并列表） */}
                        <section className="thin-scrollbar rounded-lg border border-stone-200 bg-card p-4 shadow-sm dark:border-stone-800 lg:min-h-0 lg:overflow-y-auto lg:p-5">
                            {resultPanelHeader}
                            {resultPanelBody}
                            {complianceNote}
                        </section>
                    </>
                ) : (
                    <>
                        {/* 底部布局：结果占满，创作区浮层固定在底部 */}
                        <section className="thin-scrollbar min-h-[360px] flex-1 rounded-lg border border-stone-200 bg-card p-4 pb-40 shadow-sm dark:border-stone-800 lg:min-h-0 lg:overflow-y-auto lg:p-5 lg:pb-44">
                            {resultPanelHeader}
                            {resultPanelBody}
                            {complianceNote}
                        </section>

                        <div className="pointer-events-none fixed inset-x-0 bottom-5 z-40 flex justify-center px-5 sm:bottom-7 sm:px-10 lg:px-16">
                            <div className="pointer-events-auto w-full max-w-5xl rounded-[24px] bg-white/65 p-4 shadow-[0_32px_100px_rgba(15,23,42,.22),0_10px_34px_rgba(15,23,42,.10)] ring-1 ring-white/50 backdrop-blur-2xl dark:bg-stone-950/60 dark:ring-white/10 dark:shadow-[0_34px_110px_rgba(0,0,0,.58)]">
                                <div className="flex flex-col gap-3">
                                    <div className="flex flex-col gap-2">
                                        <div className="flex flex-wrap items-center justify-end gap-2">
                                            <Button
                                                title="读取剪贴板"
                                                icon={<ClipboardPaste className="size-4" />}
                                                onClick={() =>
                                                    void pasteInto((text) => {
                                                        if (instrumental) setPrompt(text);
                                                        else setLyrics(text);
                                                    })
                                                }
                                            />
                                            <Button
                                                title="清空输入"
                                                icon={<Trash2 className="size-4" />}
                                                disabled={!prompt.trim() && !lyrics.trim()}
                                                onClick={() => {
                                                    if (instrumental) setPrompt("");
                                                    else {
                                                        setLyrics("");
                                                        setPrompt("");
                                                    }
                                                }}
                                            />
                                            <Button title="参数配置" className={`lg:hidden ${!bottomSettingsCollapsed ? "!border-sky-500/30 !bg-sky-500/10 !text-sky-500" : ""}`} icon={<SlidersHorizontal className="size-4" />} onClick={() => setBottomSettingsCollapsed((value) => !value)} />
                                            <Button title="切换到侧边工作台" icon={<PanelLeft className="size-4" />} onClick={() => setWorkbenchLayout("side")} />
                                            <Button type="primary" className="h-9 rounded-xl px-4 font-medium lg:!hidden" icon={<Sparkles className="size-4" />} loading={submitting} disabled={!canGenerate} onClick={() => void submit()}>
                                                {running.length ? `${running.length} 生成中` : "开始创作"}
                                            </Button>
                                        </div>
                                        {instrumental ? (
                                            <Input.TextArea
                                                value={prompt}
                                                onChange={(event) => setPrompt(event.target.value)}
                                                placeholder="描述想要的纯音乐：风格、情绪、乐器、场景"
                                                autoSize={{ minRows: 2, maxRows: 4 }}
                                                className="rounded-2xl"
                                            />
                                        ) : (
                                            <div className="grid gap-2 lg:grid-cols-2">
                                                <Input.TextArea
                                                    value={lyrics}
                                                    onChange={(event) => setLyrics(event.target.value)}
                                                    placeholder="粘贴或输入歌词，支持 [Verse] / [Chorus] 结构标签"
                                                    autoSize={{ minRows: 2, maxRows: 4 }}
                                                    className="rounded-2xl"
                                                />
                                                <Input
                                                    value={prompt}
                                                    onChange={(event) => setPrompt(event.target.value)}
                                                    placeholder="风格描述，例如：国风流行，古筝与电子鼓，清亮女声"
                                                    className="rounded-2xl"
                                                />
                                            </div>
                                        )}
                                    </div>
                                    <div className={`grid grid-cols-2 gap-2 sm:grid-cols-4 ${bottomSettingsCollapsed ? "hidden lg:grid" : "grid"}`}>
                                        <BottomField label="模式">
                                            <select value={mode} onChange={(event) => setMode(event.target.value as MusicMode)} className="h-11 min-w-0 rounded-xl border border-stone-200 bg-background px-3 text-sm text-stone-900 outline-none dark:border-stone-800 dark:text-stone-100">
                                                <option value="song">歌曲</option>
                                                <option value="instrumental">纯音乐</option>
                                            </select>
                                        </BottomField>
                                        <BottomField label="模型">
                                            <select value={model} onChange={(event) => setModel(event.target.value)} className="h-11 min-w-0 rounded-xl border border-stone-200 bg-background px-3 text-sm text-stone-900 outline-none dark:border-stone-800 dark:text-stone-100">
                                                {musicModels.length ? (
                                                    musicModels.map((item) => (
                                                        <option key={item} value={item}>
                                                            {item}
                                                        </option>
                                                    ))
                                                ) : (
                                                    <option value="">未配置音乐模型</option>
                                                )}
                                            </select>
                                        </BottomField>
                                        <BottomField label="时长（秒）">
                                            <input
                                                type="number"
                                                min={range.min}
                                                max={range.max}
                                                step={5}
                                                value={duration}
                                                onChange={(event) => setDuration(Math.min(range.max, Math.max(range.min, Number(event.target.value) || range.min)))}
                                                className="h-11 min-w-0 rounded-xl border border-stone-200 bg-background px-3 text-sm text-stone-900 outline-none dark:border-stone-800 dark:text-stone-100"
                                            />
                                        </BottomField>
                                        <BottomField label="生成数量">
                                            <select value={String(batchCount)} onChange={(event) => setBatchCount(Number(event.target.value))} className="h-11 min-w-0 rounded-xl border border-stone-200 bg-background px-3 text-sm text-stone-900 outline-none dark:border-stone-800 dark:text-stone-100">
                                                <option value="1">1 首</option>
                                                <option value="2">2 首</option>
                                            </select>
                                        </BottomField>
                                    </div>
                                    <div className={`flex flex-wrap items-center gap-1.5 ${bottomSettingsCollapsed ? "hidden lg:flex" : "flex"}`}>
                                        {musicStyleTags.slice(0, 8).map((tag) => (
                                            <Tag.CheckableTag key={tag} checked={styleTags.includes(tag)} className={cn("prompt-filter-tag", styleTags.includes(tag) && "is-active")} onChange={() => toggleTag(tag, styleTags, setStyleTags)}>
                                                {tag}
                                            </Tag.CheckableTag>
                                        ))}
                                    </div>
                                    <div className="flex items-center justify-between gap-3">
                                        {errorText ?? <span className="text-xs text-stone-400">预计消耗 {cost || "—"} 算力点</span>}
                                        <Button type="primary" className="hidden h-11 min-w-28 items-center justify-center gap-1.5 rounded-xl lg:flex" icon={<Sparkles className="size-4" />} loading={submitting} disabled={!canGenerate} onClick={() => void submit()}>
                                            {running.length ? `${running.length} 生成中` : "生成音乐"}
                                        </Button>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </>
                )}
            </main>
        </div>
    );
}

/** 左栏标题 + 布局切换，对齐视频创作台的 WorkbenchHeader。 */
function MusicWorkbenchHeader({ currentLayout, onLayoutChange }: { currentLayout: WorkbenchLayout; onLayoutChange: (layout: WorkbenchLayout) => void }) {
    return (
        <div className="flex shrink-0 items-center justify-between gap-3 p-4 pb-3">
            <h1 className="min-w-0 truncate text-2xl font-semibold text-stone-950 dark:text-stone-100">音乐创作台</h1>
            <div className="flex shrink-0 rounded-lg border border-stone-200 bg-stone-50 p-1 dark:border-stone-800 dark:bg-stone-900">
                <Button size="small" type={currentLayout === "side" ? "primary" : "text"} icon={<PanelLeft className="size-3.5" />} onClick={() => onLayoutChange("side")}>
                    侧边
                </Button>
                <Button size="small" type={currentLayout === "bottom" ? "primary" : "text"} icon={<PanelBottom className="size-3.5" />} onClick={() => onLayoutChange("bottom")}>
                    底部
                </Button>
            </div>
        </div>
    );
}

/** 参数分组，样式对齐视频创作台的 WorkbenchSection。 */
function MusicSection({ title, count, children }: { title: string; count?: number; children: ReactNode }) {
    return (
        <section className="overflow-hidden rounded-lg border border-stone-200 bg-background dark:border-stone-800">
            <div className="flex items-center gap-2 px-3 py-2">
                <span className="text-sm font-medium">{title}</span>
                {typeof count === "number" ? <Tag className="m-0 text-xs">{count}</Tag> : null}
            </div>
            <div className="space-y-2 border-t border-stone-200 p-3 dark:border-stone-800">{children}</div>
        </section>
    );
}

/** 底部浮层里的带标题字段。 */
function BottomField({ label, children }: { label: string; children: ReactNode }) {
    return (
        <label className="grid gap-1 text-xs text-stone-500 dark:text-stone-400">
            {label}
            {children}
        </label>
    );
}

/** 模型卡片：保留名称/单价/授权徽标，选中态用 antd token 颜色。 */
function MusicModelCard({ name, channelName, license, selected, onSelect }: { name: string; channelName: string; license: (typeof musicProtocolLicense)[string] | null; selected: boolean; onSelect: () => void }) {
    const { token } = theme.useToken();
    return (
        <button
            type="button"
            onClick={onSelect}
            style={selected ? { borderColor: token.colorPrimary, background: token.colorPrimaryBg } : undefined}
            className={cn("flex items-center justify-between gap-3 rounded-lg border px-3 py-2 text-left transition", selected ? "" : "border-stone-200 hover:border-stone-400 dark:border-stone-700")}
        >
            <span className="min-w-0">
                <span className="block truncate text-[13px] font-medium" style={selected ? { color: token.colorPrimary } : undefined}>
                    {name}
                </span>
                <span className="block truncate text-[11px] text-stone-400">{channelName || "未指定渠道"}</span>
            </span>
            {license ? (
                <span className={cn("shrink-0 rounded border px-1.5 py-0.5 text-[11px]", license.tone === "official" ? "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-300" : "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300")}>{license.label}</span>
            ) : null}
        </button>
    );
}

/** 合并列表里的结果卡片：执行中 / 失败 / 完成三态。 */
function MusicResultCard({
    task,
    focused,
    candidates,
    onDownload,
    onSaveAsset,
    onCopyLink,
    onRemove,
    onRegenerate,
}: {
    task: MusicTask;
    focused: boolean;
    candidates: MusicTask[];
    onDownload: (task: MusicTask) => void;
    onSaveAsset: (task: MusicTask) => void;
    onCopyLink: (task: MusicTask) => void;
    onRemove: (task: MusicTask) => void;
    onRegenerate: () => void;
}) {
    const { token } = theme.useToken();
    const buttons = [
        { label: "下载", icon: <Download className="size-3" />, onClick: () => onDownload(task) },
        { label: "存入素材", onClick: () => onSaveAsset(task) },
        { label: "复制链接", onClick: () => onCopyLink(task) },
        { label: "再来一版", onClick: onRegenerate },
    ];
    return (
        <section
            className={cn("flex flex-col rounded-lg border p-4", focused ? "" : isMusicTaskRunning(task.status) ? "border-sky-300 bg-sky-50/40 dark:border-sky-800 dark:bg-sky-950/20" : "border-stone-200 bg-background dark:border-stone-800")}
            style={focused ? { borderColor: token.colorPrimary, background: token.colorPrimaryBg } : undefined}
        >
            <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                    <p className="truncate text-[15px] font-semibold">{task.title || task.prompt || "未命名作品"}</p>
                    <p className="truncate text-xs text-stone-400">
                        {[task.mode === "instrumental" ? "纯音乐" : "歌曲", (task.styleTags || []).join("/"), task.upstream_model || task.model, task.duration_ms ? formatMusicDuration(task.duration_ms) : task.duration ? `${task.duration}s` : ""].filter(Boolean).join(" · ")}
                    </p>
                </div>
                <span className={cn("shrink-0 rounded border px-1.5 py-0.5 text-[11px]", isMusicTaskFailed(task.status) ? "border-red-200 bg-red-50 text-red-600 dark:border-red-900 dark:bg-red-950 dark:text-red-300" : isMusicTaskRunning(task.status) ? "border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-950 dark:text-sky-300" : "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-300")}>{musicTaskStatusText(task.status)}</span>
            </div>

            {isMusicTaskRunning(task.status) ? (
                <div className="mt-3">
                    <div className="h-1.5 w-full overflow-hidden rounded-full bg-sky-100 dark:bg-sky-900">
                        <div className="h-full rounded-full bg-sky-500 transition-all" style={{ width: `${task.progress || 0}%` }} />
                    </div>
                    <p className="mt-2 inline-flex items-center gap-1.5 text-xs text-sky-600 dark:text-sky-400">
                        <LoaderCircle className="size-3 animate-spin" />
                        {task.progress || 0}%
                    </p>
                </div>
            ) : isMusicTaskFailed(task.status) ? (
                <>
                    <p className="mt-3 rounded-md border border-red-200 bg-red-50 px-2 py-1.5 text-xs text-red-600 dark:border-red-900 dark:bg-red-950 dark:text-red-300">{musicTaskErrorMessage(task)}</p>
                    <div className="mt-3 flex justify-end">
                        <Button size="small" danger icon={<Trash2 className="size-3" />} onClick={() => onRemove(task)}>
                            删除
                        </Button>
                    </div>
                </>
            ) : (
                <>
                    {musicTaskAudioUrl(task) ? <AudioPlayer src={musicTaskAudioUrl(task)} className="mt-3" /> : null}
                    {candidates.length ? (
                        <div className="mt-3 rounded-lg border border-stone-200 p-3 dark:border-stone-800">
                            <p className="mb-2 text-xs text-stone-400">同一批候选（A/B 试听）</p>
                            <div className="flex flex-col divide-y divide-stone-100 dark:divide-stone-800">
                                {candidates.map((item) => (
                                    <div key={item.id} className="flex flex-col gap-2 py-2 first:pt-0 last:pb-0">
                                        <p className="truncate text-xs font-medium">{item.title || item.prompt || "候选"}</p>
                                        {musicTaskAudioUrl(item) ? <AudioPlayer src={musicTaskAudioUrl(item)} compact /> : null}
                                    </div>
                                ))}
                            </div>
                        </div>
                    ) : null}
                    <div className="mt-3 flex flex-wrap gap-2">
                        {buttons.map((item) => (
                            <Button key={item.label} size="small" icon={item.icon} onClick={item.onClick}>
                                {item.label}
                            </Button>
                        ))}
                    </div>
                    {task.lyrics ? <pre className="mt-3 max-h-40 overflow-auto whitespace-pre-wrap rounded-lg bg-stone-50 p-3 text-xs text-stone-600 dark:bg-stone-950 dark:text-stone-300">{task.lyrics}</pre> : null}
                    <div className="mt-3 flex justify-end">
                        <Button size="small" danger icon={<Trash2 className="size-3" />} onClick={() => onRemove(task)}>
                            删除
                        </Button>
                    </div>
                </>
            )}
        </section>
    );
}

function protocolOfModel(config: ReturnType<typeof useEffectiveConfig>, model: string) {
    const channels = config.channelMode === "remote" ? config.publicChannels : config.localChannels;
    return channels.find((channel) => (channel.models || []).includes(model))?.protocol || "";
}

function channelNameOfModel(config: ReturnType<typeof useEffectiveConfig>, model: string) {
    const channels = config.channelMode === "remote" ? config.publicChannels : config.localChannels;
    const matched = channels.find((channel) => (channel.models || []).includes(model));
    return matched?.name || matched?.baseUrl || "";
}
