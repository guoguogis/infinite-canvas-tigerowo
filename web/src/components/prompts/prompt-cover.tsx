"use client";

import { Image as ImageIcon, Play } from "lucide-react";
import { useEffect, useRef, useState } from "react";

// 按标题哈希出一个稳定色相，让同一条提示词的渐变封面每次都一致。
function coverHue(title: string) {
    let hash = 0;
    for (let index = 0; index < title.length; index += 1) hash = (hash * 31 + title.charCodeAt(index)) % 360;
    return hash;
}

// 封面缺失或加载失败时显示渐变占位块，避免出现浏览器裂图或空白卡片。
export function PromptCover({ url, title, className, hasVideo = false }: { url: string; title: string; className: string; hasVideo?: boolean }) {
    const [failed, setFailed] = useState(false);
    const badge = hasVideo ? (
        <span className="pointer-events-none absolute inset-0 grid place-items-center bg-black/15 transition group-hover:bg-black/25">
            <span className="grid size-9 place-items-center rounded-full bg-black/55 text-white backdrop-blur-sm">
                <Play className="size-4 translate-x-[1px] fill-current" />
            </span>
        </span>
    ) : null;

    if (!url || failed) {
        const hue = coverHue(title);
        return (
            <span
                className={`${className} relative grid place-items-center`}
                style={{ background: `radial-gradient(circle at 30% 22%, rgb(255 255 255 / 0.3), transparent 62%), linear-gradient(135deg, hsl(${hue} 58% 56%), hsl(${(hue + 48) % 360} 55% 37%))` }}
            >
                {hasVideo ? null : <ImageIcon className="size-6 text-white/70" />}
                {badge}
            </span>
        );
    }
    return (
        <span className="relative block">
            <img src={url} alt={title} loading="lazy" className={className} onError={() => setFailed(true)} />
            {badge}
        </span>
    );
}

// 详情里的预览：有视频就直接播放，否则退化为封面图。
export function PromptPreview({ videoUrl, coverUrl, title, className }: { videoUrl: string; coverUrl: string; title: string; className: string }) {
    const [videoFailed, setVideoFailed] = useState(false);
    const videoRef = useRef<HTMLVideoElement | null>(null);
    // 卸载或切换视频时显式停掉播放：关闭弹框只是把内容隐藏，浏览器不保证停止已挂载的媒体。
    useEffect(() => {
        const video = videoRef.current;
        return () => {
            if (!video) return;
            video.pause();
            video.removeAttribute("src");
            video.load();
        };
    }, [videoUrl, videoFailed]);
    if (videoUrl && !videoFailed) {
        return <video ref={videoRef} src={videoUrl} poster={coverUrl || undefined} controls autoPlay loop muted playsInline preload="metadata" className={className} onError={() => setVideoFailed(true)} />;
    }
    return <PromptCover url={coverUrl} title={title} className={className} />;
}
