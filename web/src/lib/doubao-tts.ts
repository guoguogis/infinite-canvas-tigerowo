import { channelProtocolForConfig, type AiConfig } from "@/stores/use-config-store";

// 豆包语音合成（独立渠道，TokenPlan 与后付费只差 baseUrl）的 2.0 音色表。
// 表中音色全部以 _uranus_bigtts 结尾，对应 X-Api-Resource-Id: seed-tts-2.0。
export const DOUBAO_TTS_CHANNEL_PROTOCOL = "doubao-tts";
export const DOUBAO_TTS_DEFAULT_VOICE = "zh_female_shaoergushi_uranus_bigtts";

export const doubaoTtsVoiceOptions = [
    { value: "zh_female_tvbnv_uranus_bigtts", label: "TVB女声" },
    { value: "zh_female_yingyujiaoxue_uranus_bigtts", label: "Tina老师" },
    { value: "zh_female_vv_uranus_bigtts", label: "Vivi" },
    { value: "zh_male_dongfanghaoran_uranus_bigtts", label: "东方浩然" },
    { value: "zh_male_m191_uranus_bigtts", label: "云舟" },
    { value: "zh_male_liangsangmengzai_uranus_bigtts", label: "亮嗓萌仔" },
    { value: "zh_female_qinqienv_uranus_bigtts", label: "亲切女声" },
    { value: "zh_female_peiqi_uranus_bigtts", label: "佩奇猪" },
    { value: "zh_female_qiaopinv_uranus_bigtts", label: "俏皮女声" },
    { value: "zh_male_aojiaobazong_uranus_bigtts", label: "傲娇霸总" },
    { value: "zh_male_ruyayichen_uranus_bigtts", label: "儒雅逸辰" },
    { value: "zh_male_ruyaqingnian_uranus_bigtts", label: "儒雅青年" },
    { value: "zh_female_xiaoxue_uranus_bigtts", label: "儿童绘本" },
    { value: "zh_male_liufei_uranus_bigtts", label: "刘飞" },
    { value: "zh_male_fanjuanqingnian_uranus_bigtts", label: "反卷青年" },
    { value: "zh_female_gufengshaoyu_uranus_bigtts", label: "古风少御" },
    { value: "zh_male_tangseng_uranus_bigtts", label: "唐僧" },
    { value: "zh_male_silang_uranus_bigtts", label: "四郎" },
    { value: "zh_male_dayi_uranus_bigtts", label: "大壹" },
    { value: "zh_male_tiancaitongsheng_uranus_bigtts", label: "天才童声" },
    { value: "zh_female_nvleishen_uranus_bigtts", label: "女雷神" },
    { value: "zh_male_naiqimengwa_uranus_bigtts", label: "奶气萌娃" },
    { value: "zh_female_jiaochuannv_uranus_bigtts", label: "娇喘女声" },
    { value: "zh_female_popo_uranus_bigtts", label: "婆婆" },
    { value: "zh_female_xiaohe_uranus_bigtts", label: "小何" },
    { value: "zh_male_taocheng_uranus_bigtts", label: "小天" },
    { value: "zh_female_shaoergushi_uranus_bigtts", label: "少儿故事" },
    { value: "zh_male_shaonianzixin_uranus_bigtts", label: "少年梓辛/Brayan" },
    { value: "zh_male_guanggaojieshuo_uranus_bigtts", label: "广告解说" },
    { value: "zh_male_zhuangzhou_uranus_bigtts", label: "庄周" },
    { value: "zh_female_kailangjiejie_uranus_bigtts", label: "开朗姐姐" },
    { value: "zh_male_kailangxuezhang_uranus_bigtts", label: "开朗学长" },
    { value: "zh_male_kailangdidi_uranus_bigtts", label: "开朗弟弟" },
    { value: "zh_female_xinlingjitang_uranus_bigtts", label: "心灵鸡汤" },
    { value: "zh_male_kuailexiaodong_uranus_bigtts", label: "快乐小东" },
    { value: "zh_male_youyoujunzi_uranus_bigtts", label: "悠悠君子" },
    { value: "zh_male_xuanyijieshuo_uranus_bigtts", label: "悬疑解说" },
    { value: "zh_female_ganmaodianyin_uranus_bigtts", label: "感冒电音姐姐" },
    { value: "zh_male_lanyinmianbao_uranus_bigtts", label: "懒音绵宝" },
    { value: "zh_female_sajiaoxuemei_uranus_bigtts", label: "撒娇学妹" },
    { value: "zh_male_qingcang_uranus_bigtts", label: "擎苍" },
    { value: "zh_female_wenjingmaomao_uranus_bigtts", label: "文静毛毛" },
    { value: "zh_female_chunribu_uranus_bigtts", label: "春日部姐姐" },
    { value: "zh_female_kefunvsheng_uranus_bigtts", label: "暖阳女声" },
    { value: "zh_female_linxiao_uranus_bigtts", label: "林潇" },
    { value: "zh_female_roumeinvyou_uranus_bigtts", label: "柔美女友" },
    { value: "zh_female_yingtaowanzi_uranus_bigtts", label: "樱桃丸子" },
    { value: "zh_female_wuzetian_uranus_bigtts", label: "武则天" },
    { value: "zh_male_huolixiaoge_uranus_bigtts", label: "活力小哥" },
    { value: "zh_female_liuchangnv_uranus_bigtts", label: "流畅女声" },
    { value: "zh_male_shenyeboke_uranus_bigtts", label: "深夜播客" },
    { value: "zh_female_qingxinnvsheng_uranus_bigtts", label: "清新女声" },
    { value: "zh_female_qingchezizi_uranus_bigtts", label: "清澈梓梓" },
    { value: "zh_male_qingshuangnanda_uranus_bigtts", label: "清爽男大" },
    { value: "zh_male_yuanboxiaoshu_uranus_bigtts", label: "渊博小叔" },
    { value: "zh_male_wennuanahu_uranus_bigtts", label: "温暖阿虎/Alvin" },
    { value: "zh_female_wenroumama_uranus_bigtts", label: "温柔妈妈" },
    { value: "zh_male_wenrouxiaoge_uranus_bigtts", label: "温柔小哥" },
    { value: "zh_female_wenrouxiaoya_uranus_bigtts", label: "温柔小雅" },
    { value: "zh_female_wenroushunv_uranus_bigtts", label: "温柔淑女" },
    { value: "zh_male_xionger_uranus_bigtts", label: "熊二" },
    { value: "zh_female_shuangkuaisisi_uranus_bigtts", label: "爽快思思" },
    { value: "zh_male_zhubajie_uranus_bigtts", label: "猪八戒" },
    { value: "zh_male_sunwukong_uranus_bigtts", label: "猴哥" },
    { value: "zh_female_lingling_uranus_bigtts", label: "玲玲姐姐" },
    { value: "zh_female_tianmeixiaoyuan_uranus_bigtts", label: "甜美小源" },
    { value: "zh_female_tianmeiyueyue_uranus_bigtts", label: "甜美悦悦" },
    { value: "zh_female_tianmeitaozi_uranus_bigtts", label: "甜美桃子" },
    { value: "zh_female_zhishuaiyingzi_uranus_bigtts", label: "直率英子" },
    { value: "zh_female_zhixingnv_uranus_bigtts", label: "知性女声" },
    { value: "zh_female_cancan_uranus_bigtts", label: "知性灿灿" },
    { value: "zh_male_cixingjieshuonan_uranus_bigtts", label: "磁性解说男声/Morgan" },
    { value: "zh_female_mengyatou_uranus_bigtts", label: "萌丫头/Cutey" },
    { value: "zh_male_jieshuoxiaoming_uranus_bigtts", label: "解说小明" },
    { value: "zh_male_yizhipiannan_uranus_bigtts", label: "译制片男" },
    { value: "zh_female_chanmeinv_uranus_bigtts", label: "谄媚女声" },
    { value: "zh_female_tiexinnvsheng_uranus_bigtts", label: "贴心女声/Candy" },
    { value: "zh_female_linjianvhai_uranus_bigtts", label: "邻家女孩" },
    { value: "zh_male_linjiananhai_uranus_bigtts", label: "邻家男孩" },
    { value: "zh_male_yangguangqingnian_uranus_bigtts", label: "阳光青年" },
    { value: "zh_male_baqiqingshu_uranus_bigtts", label: "霸气青叔" },
    { value: "zh_female_gujie_uranus_bigtts", label: "顾姐" },
    { value: "zh_female_gaolengyujie_uranus_bigtts", label: "高冷御姐" },
    { value: "zh_male_gaolengchenwen_uranus_bigtts", label: "高冷沉稳" },
    { value: "zh_female_meilinvyou_uranus_bigtts", label: "魅力女友" },
    { value: "zh_female_sophie_uranus_bigtts", label: "魅力苏菲" },
    { value: "zh_male_lubanqihao_uranus_bigtts", label: "鲁班七号" },
    { value: "zh_female_jitangnv_uranus_bigtts", label: "鸡汤女" },
    { value: "zh_female_jitangmei_uranus_bigtts", label: "鸡汤妹妹/Hope" },
    { value: "zh_female_mizai_uranus_bigtts", label: "黑猫侦探社咪仔" },
];

export function isDoubaoTTsConfig(config: AiConfig, modelName: string) {
    const model = modelName.trim();
    return channelProtocolForConfig({ ...config, model, audioModel: model }) === DOUBAO_TTS_CHANNEL_PROTOCOL;
}

export function normalizeDoubaoTTsVoice(value: string) {
    return doubaoTtsVoiceOptions.some((item) => item.value === value) ? value : DOUBAO_TTS_DEFAULT_VOICE;
}

export function doubaoTtsVoiceLabel(value: string) {
    const voice = normalizeDoubaoTTsVoice(value);
    return doubaoTtsVoiceOptions.find((item) => item.value === voice)?.label || voice;
}
