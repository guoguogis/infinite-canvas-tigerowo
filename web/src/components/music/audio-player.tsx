"use client";

import { Pause, Play, Repeat, Volume2, VolumeX } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { formatMusicDuration } from "@/lib/music-styles";
import { cn } from "@/lib/utils";

const playRates = [1, 1.25, 1.5, 2, 0.75];

type AudioPlayerProps = {
    src: string;
    className?: string;
    compact?: boolean;
};

/** 可复用的扁平风格音频播放器：结果区与「我的音乐」列表共用。 */
export function AudioPlayer({ src, className, compact = false }: AudioPlayerProps) {
    const audioRef = useRef<HTMLAudioElement | null>(null);
    const [playing, setPlaying] = useState(false);
    const [current, setCurrent] = useState(0);
    const [total, setTotal] = useState(0);
    const [volume, setVolume] = useState(1);
    const [muted, setMuted] = useState(false);
    const [rate, setRate] = useState(1);
    const [loop, setLoop] = useState(false);

    useEffect(() => {
        setPlaying(false);
        setCurrent(0);
        setTotal(0);
    }, [src]);

    const toggle = () => {
        const audio = audioRef.current;
        if (!audio) return;
        if (audio.paused) void audio.play().catch(() => setPlaying(false));
        else audio.pause();
    };

    const seek = (seconds: number) => {
        const audio = audioRef.current;
        if (!audio) return;
        audio.currentTime = seconds;
        setCurrent(seconds);
    };

    const changeVolume = (next: number) => {
        const audio = audioRef.current;
        setVolume(next);
        setMuted(next === 0);
        if (audio) {
            audio.volume = next;
            audio.muted = next === 0;
        }
    };

    const cycleRate = () => {
        const next = playRates[(playRates.indexOf(rate) + 1) % playRates.length];
        setRate(next);
        if (audioRef.current) audioRef.current.playbackRate = next;
    };

    const toggleLoop = () => {
        const next = !loop;
        setLoop(next);
        if (audioRef.current) audioRef.current.loop = next;
    };

    const percent = total > 0 ? (current / total) * 100 : 0;

    return (
        <div className={cn("flex w-full flex-col gap-2", className)}>
            <audio
                ref={audioRef}
                src={src}
                preload="metadata"
                onPlay={() => setPlaying(true)}
                onPause={() => setPlaying(false)}
                onEnded={() => setPlaying(false)}
                onTimeUpdate={(event) => setCurrent(event.currentTarget.currentTime)}
                onLoadedMetadata={(event) => {
                    setTotal(Number.isFinite(event.currentTarget.duration) ? event.currentTarget.duration : 0);
                    event.currentTarget.volume = volume;
                    event.currentTarget.playbackRate = rate;
                }}
            />

            <div className="flex items-center gap-3">
                <button
                    type="button"
                    onClick={toggle}
                    aria-label={playing ? "暂停" : "播放"}
                    className="inline-flex size-8 shrink-0 items-center justify-center rounded-full border border-stone-300 text-stone-700 transition hover:border-stone-500 hover:text-stone-950 dark:border-stone-700 dark:text-stone-200 dark:hover:border-stone-500"
                >
                    {playing ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
                </button>

                <span className="shrink-0 text-xs tabular-nums text-stone-500 dark:text-stone-400">{formatMusicDuration(current * 1000)}</span>

                <input
                    type="range"
                    min={0}
                    max={Math.max(total, 1)}
                    step={0.1}
                    value={current}
                    onChange={(event) => seek(Number(event.target.value))}
                    aria-label="播放进度"
                    className="h-1 min-w-0 flex-1 cursor-pointer appearance-none rounded-full bg-stone-200 accent-sky-500 dark:bg-stone-800"
                    style={{ backgroundImage: `linear-gradient(to right, rgb(14 165 233) ${percent}%, transparent ${percent}%)` }}
                />

                <span className="shrink-0 text-xs tabular-nums text-stone-500 dark:text-stone-400">{formatMusicDuration(total * 1000)}</span>
            </div>

            {compact ? null : (
                <div className="flex items-center gap-3 text-xs text-stone-500 dark:text-stone-400">
                    <button type="button" onClick={() => changeVolume(muted ? 1 : 0)} aria-label={muted ? "取消静音" : "静音"} className="inline-flex items-center transition hover:text-stone-900 dark:hover:text-stone-100">
                        {muted || volume === 0 ? <VolumeX className="size-3.5" /> : <Volume2 className="size-3.5" />}
                    </button>
                    <input
                        type="range"
                        min={0}
                        max={1}
                        step={0.05}
                        value={muted ? 0 : volume}
                        onChange={(event) => changeVolume(Number(event.target.value))}
                        aria-label="音量"
                        className="h-1 w-20 cursor-pointer appearance-none rounded-full bg-stone-200 accent-sky-500 dark:bg-stone-800"
                    />
                    <button type="button" onClick={cycleRate} className="tabular-nums transition hover:text-stone-900 dark:hover:text-stone-100" aria-label="播放倍速">
                        {rate}x
                    </button>
                    <button
                        type="button"
                        onClick={toggleLoop}
                        aria-label="循环播放"
                        className={cn("inline-flex items-center transition hover:text-stone-900 dark:hover:text-stone-100", loop && "text-sky-600 dark:text-sky-400")}
                    >
                        <Repeat className="size-3.5" />
                    </button>
                </div>
            )}
        </div>
    );
}
