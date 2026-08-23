import type { DimensionKey } from './types'

export type PresetCardTemplate = {
  id: string
  name: string
  tagline: string
  description: string
  icon: string
  dimensions: Record<DimensionKey, { level: number; summary: string; techniques: string[] }>
  prohibitions: string[]
}

export const PRESET_CARDS: PresetCardTemplate[] = [
  {
    id: 'preset-fanren',
    name: '凡人仙途 · 冷冽白描',
    tagline: '低情绪宣泄 · 重因果算计 · 克制冷峻',
    description: '继承凡人修仙流精髓。注重环境细节与心理暗涌，主角心智成熟戒备，极少有情绪化的大喊大叫与冲动之举，一切行事以利害因果为准则。',
    icon: '⚔️',
    dimensions: {
      sentence_rhythm: {
        level: 48,
        summary: '以中长句铺陈环境与细节，动作交锋时转为冷峻短句，节奏沉稳克制。',
        techniques: ['冷峻短句收尾', '客观白描动作', '环境细节渗透'],
      },
      narrative_perspective: {
        level: 35,
        summary: '第三人称有限视角，高度聚焦主角的机警观察与内心戒备。',
        techniques: ['限制性视点', '戒备心理白描', '不轻信外人视角'],
      },
      dialogue_density: {
        level: 38,
        summary: '对白密度中等偏低，说话留三分余地，极少废话，多有虚与委蛇与试探。',
        techniques: ['言语试探', '虚与委蛇', '表面恭敬暗藏机锋'],
      },
      sensory_description: {
        level: 72,
        summary: '重光影、气味、灵气波动与周遭危机感官捕捉，烘托真实质感。',
        techniques: ['法力波动感知', '阴暗环境触感', '血腥与杀气捕捉'],
      },
      scene_pacing: {
        level: 55,
        summary: '场景推进稳扎稳打，准备工作与杀伐决断分明，无拖泥带水。',
        techniques: ['杀伐果断不留后患', '事后迅速打扫战场', '遁光撤离'],
      },
      emotional_expression: {
        level: 25,
        summary: '极低的情绪外放，哪怕生死关头也面色如常，只在心底迅速权衡退路。',
        techniques: ['喜怒不形于色', '内心冷笑计算', '绝不无能狂怒'],
      },
      rhetoric_preference: {
        level: 42,
        summary: '克制朴素的文白相间比喻，注重精准表意，不堆砌辞藻。',
        techniques: ['精准名物名词', '精炼文白互参', '拒绝繁琐修饰'],
      },
      lexical_texture: {
        level: 68,
        summary: '大量使用修真名物、古朴法宝器物与天地灵草准确术语。',
        techniques: ['古拙典雅动词', '法宝术数名词', '炼气修心术语'],
      },
      tension_hook: {
        level: 78,
        summary: '每章末尾常留有未知杀机、灵物现世或第三方势力窥伺之暗钩。',
        techniques: ['暗处窥伺尾声', '神识警戒触动', '传送阵灵光异变'],
      },
    },
    prohibitions: [
      '严禁主角无脑圣母或心慈手软',
      '严禁反派在动手前长篇大论废话',
      '严禁无由来的狂妄大笑或无能咆哮',
      '严禁出现现代网络口癖与出戏词汇',
    ],
  },
  {
    id: 'preset-golden3',
    name: '黄金三章 · 爆款快节奏',
    tagline: '短句快推 · 极强冲突张力 · 密集钩子',
    description: '商业网文爆款节奏。短句密集，每三百字一处微冲突，每一千字一次情绪反转与高潮打脸，章末绝命断章钩子拉满。',
    icon: '⚡',
    dimensions: {
      sentence_rhythm: {
        level: 82,
        summary: '极高比例的短句与单字段，呼吸短促，冲击力强，扫读体验极佳。',
        techniques: ['短句轰炸', '单句成段强调', '极速动作切分'],
      },
      narrative_perspective: {
        level: 22,
        summary: '贴身第一/近身第三视角，读者代入感极强，情绪实时同步。',
        techniques: ['实时情绪共鸣', '爽点即时反馈', '主观强代入'],
      },
      dialogue_density: {
        level: 65,
        summary: '对白密度高，言语挑衅与反击针锋相对，情绪烈度高。',
        techniques: ['强冲突对白', '轻蔑与打脸回击', '短促有力交锋'],
      },
      sensory_description: {
        level: 40,
        summary: '快速给到核心视觉与爆裂听觉，不展开冗长景物描写。',
        techniques: ['炸裂声光呈现', '直观痛感反馈', '气场压迫感'],
      },
      scene_pacing: {
        level: 88,
        summary: '极速场景推进，直奔矛盾核心，省略一切无关过渡。',
        techniques: ['跳过无效日常', '开门见山切入冲突', '节奏绝不拖泥带水'],
      },
      emotional_expression: {
        level: 85,
        summary: '强烈的情绪释放与扬眉吐气感，憋屈必迅速反转，爽感直击人心。',
        techniques: ['憋屈迅速反击', '震撼全场反差', '扬眉吐气情绪'],
      },
      rhetoric_preference: {
        level: 30,
        summary: '通俗易懂，多用强力动词与具象比喻，拒绝晦涩隐喻。',
        techniques: ['白话通透表达', '强力动词驱动', '直给爽点'],
      },
      lexical_texture: {
        level: 35,
        summary: '现代通俗网文语汇，流畅轻快，阅读零阻力。',
        techniques: ['高频流行表达', '轻量化词汇', '低认知负荷'],
      },
      tension_hook: {
        level: 95,
        summary: '章末必留断章钩子，巨大悬念或强援降临，让人欲罢不能。',
        techniques: ['生死一线断章', '神秘强敌降临', '惊天底牌亮出一角'],
      },
    },
    prohibitions: [
      '严禁长篇大论的世界观说明文背景堆砌',
      '严禁主角持续憋屈超过一千字不反击',
      '严禁出现大段冗长复杂的学术化设定解析',
    ],
  },
  {
    id: 'preset-strange',
    name: '古典志怪 · 诡谲奇谭',
    tagline: '浓郁语汇 · 中式民俗恐怖 · 氛围烘托',
    description: '聊斋与中式怪谈风格。文辞古朴雅致，重在以夜雾、孤烛、荒祠、皮影等民俗意象烘托诡谲阴冷之感，神秘莫测。',
    icon: '🏮',
    dimensions: {
      sentence_rhythm: {
        level: 60,
        summary: '文白相间，四六句错落，语流如古琴泛音，幽深回环。',
        techniques: ['四字成语点睛', '虚词顿挫穿插', '回环往复韵味'],
      },
      narrative_perspective: {
        level: 68,
        summary: '全知与旁观者视角转换，如说书人娓娓道来荒诞异事。',
        techniques: ['说书客视角', '旁观冷眼摹写', '真假莫辨传闻'],
      },
      dialogue_density: {
        level: 45,
        summary: '对话半文半白，辞意隐晦，常带谶语与机锋，耐人寻味。',
        techniques: ['半文半白口吻', '谶语暗示', '鬼怪客套笑语藏刀'],
      },
      sensory_description: {
        level: 92,
        summary: '极度细腻的阴冷视觉、霉腐纸钱气味、凄厉风声与冰凉触觉描写。',
        techniques: ['烛火青碧摇曳', '纸灰与冷香弥漫', '指尖刺骨阴寒'],
      },
      scene_pacing: {
        level: 42,
        summary: '层层铺陈阴森氛围，水到渠成揭开恐怖真相，余味悠长。',
        techniques: ['循序渐进压迫', '细思极恐细节', '荒诞留白结尾'],
      },
      emotional_expression: {
        level: 50,
        summary: '惊悚与苍凉并存，对人妖鬼魅既有畏惧亦有宿命叹息。',
        techniques: ['毛骨悚然战栗', '宿命叹惋', '荒冢白骨之悲'],
      },
      rhetoric_preference: {
        level: 85,
        summary: '极富诗意与古典意象的拟人与移情，字字有画面。',
        techniques: ['古典民俗意象', '借景抒怪异情', '拟物生灵'],
      },
      lexical_texture: {
        level: 88,
        summary: '典雅文言实词、阴阳八卦术语与古代民俗典章词汇丰富。',
        techniques: ['文言实词点缀', '民俗鬼神名号', '古色古香字眼'],
      },
      tension_hook: {
        level: 70,
        summary: '章末往往留下一处不合常理的细微破绽，让人背脊发凉。',
        techniques: ['镜中倒影异样', '多出一双绣花鞋', '身后传来熟悉笑声'],
      },
    },
    prohibitions: [
      '严禁出现任何现代科技词汇或现代俗语',
      '严禁鬼怪出场即直白无脑大吼大叫',
      '严禁破坏古典志怪的朦胧与敬畏氛围',
    ],
  },
  {
    id: 'preset-hardboiled',
    name: '硬派硬核 · 视听镜头',
    tagline: '电影分镜 · 冷硬动作 · 高对白机锋',
    description: '兼具冷硬派推理与动作大片质感。如摄影机般冷峻记录肢体动作、弹道轨迹与微表情，对白简短如子弹般精准致命。',
    icon: '🎬',
    dimensions: {
      sentence_rhythm: {
        level: 75,
        summary: '快切短句如连发点射，省略主语与连词，极具打击节奏感。',
        techniques: ['电影分镜切分', '省略连词快推', '击打感短动词'],
      },
      narrative_perspective: {
        level: 40,
        summary: '极度克制客观的摄影机视角，只记录所见所闻，不猜测内心。',
        techniques: ['客观摄影机机位', '冷眼旁观行为', '通过动作显露心理'],
      },
      dialogue_density: {
        level: 58,
        summary: '对白硬朗短促，字字机锋，充满黑色幽默与致命威胁。',
        techniques: ['短句交火', '黑色幽默潜台词', '冷酷最后通牒'],
      },
      sensory_description: {
        level: 65,
        summary: '强调金属冰冷触感、硝烟与雨水气味、机械上膛清脆回响。',
        techniques: ['机械咬合声', '雨水与血腥气味', '后坐力骨骼震颤'],
      },
      scene_pacing: {
        level: 80,
        summary: '紧凑高效，从追踪到交火无缝衔接，高潮迭起。',
        techniques: ['战术推演切入', '瞬息生死搏杀', '干净利落收尾'],
      },
      emotional_expression: {
        level: 20,
        summary: '冷酷硬汉，极少流露软弱，痛楚与压力全被压抑在骨髓深处。',
        techniques: ['嚼碎血水咽下', '眼神如死水不波', '沉默抽完最后一口烟'],
      },
      rhetoric_preference: {
        level: 35,
        summary: '极简白描，偶尔出现的比喻如刀刻般冰冷锋利。',
        techniques: ['冷金属隐喻', '直观物理动作', '毫无花哨修饰'],
      },
      lexical_texture: {
        level: 55,
        summary: '精准的机械、武器、人体解剖与战术术语。',
        techniques: ['战术标准名词', '精准肌肉解剖', '机械型号数据'],
      },
      tension_hook: {
        level: 82,
        summary: '章末往往定格在撞针击发前一瞬或红外线红点锁定的瞬间。',
        techniques: ['红外瞄准点亮起', '电话那头传来盲音', '电梯门缓缓开启'],
      },
    },
    prohibitions: [
      '严禁多愁善感的无谓伤感独白',
      '严禁反科学的夸张动作描写',
      '严禁长篇大论的心路历程自我辩解',
    ],
  },
  {
    id: 'preset-epic',
    name: '群像史诗 · 沉郁顿挫',
    tagline: '多视点交织 · 宏大场景 · 厚重历史宿命',
    description: '大型严肃历史与奇幻史诗流派。全景展现列国博弈、王权更迭与众生相，行文如大江大河，波澜壮阔，带有深沉的宿命感。',
    icon: '👑',
    dimensions: {
      sentence_rhythm: {
        level: 50,
        summary: '长句大段交织，如长卷画轴徐徐铺展，语调庄严典重。',
        techniques: ['排比长卷铺陈', '庄严叙事语调', '史官实录口吻'],
      },
      narrative_perspective: {
        level: 80,
        summary: '宏观全知与多人物视角交替切入，展现立体群像博弈。',
        techniques: ['多线并行切入', '全景阵营俯瞰', '时代巨浪中的小人物视角'],
      },
      dialogue_density: {
        level: 50,
        summary: '庙堂策论与营帐密谋，对白极具政治智慧与历史分量。',
        techniques: ['庙堂争锋论道', '君臣利益权衡', '悲壮临别誓言'],
      },
      sensory_description: {
        level: 70,
        summary: '千军万马的铁蹄震颤、旗帜猎猎作响与天地苍茫景象。',
        techniques: ['铁骑踏破冰河', '烽火连天苍茫', '城池倾覆烽烟'],
      },
      scene_pacing: {
        level: 45,
        summary: '厚积薄发，先布大局再引爆决战，每场冲突均有巨大战略影响。',
        techniques: ['草蛇灰线暗局', '天下棋局博弈', '决战势不可挡'],
      },
      emotional_expression: {
        level: 70,
        summary: '深沉厚重，英雄迟暮与时代挽歌，极具感染力与回味。',
        techniques: ['时代挽歌之叹', '义无反顾之死', '天下苍生之悯'],
      },
      rhetoric_preference: {
        level: 75,
        summary: '气势磅礴的历史隐喻与天地意象，文辞博大。',
        techniques: ['星宿移位借喻', '山河破碎移情', '史诗颂歌修辞'],
      },
      lexical_texture: {
        level: 80,
        summary: '丰富严谨的古制官职、地理舆图、军阵兵器名词。',
        techniques: ['官阶仪轨规范', '山川关隘全称', '史书体裁实词'],
      },
      tension_hook: {
        level: 75,
        summary: '章末常定格在八百里加急军报送达或天下大势剧变转折点。',
        techniques: ['八百里加急战报', '老臣自缢相府', '边关狼烟四起'],
      },
    },
    prohibitions: [
      '严禁单主角独角戏而忽略群像逻辑',
      '严禁幼稚过家家式的政治权谋儿戏',
      '严禁主角一己之力无视客观规律逆天',
    ],
  },
  {
    id: 'preset-light',
    name: '轻快日常 · 机锋反差',
    tagline: '高对白密度 · 吐槽潜台词 · 反差萌幽默',
    description: '现代都市与轻松欢脱流派。节奏欢快，对话充满段子与心理反差吐槽，在生活烟火气中推进主线，解压治愈。',
    icon: '☕',
    dimensions: {
      sentence_rhythm: {
        level: 68,
        summary: '短小精悍，口语化切分，段落轻巧灵动，易读不费脑。',
        techniques: ['轻巧短段落', '逗趣断句节奏', '生活化语感'],
      },
      narrative_perspective: {
        level: 30,
        summary: '第一人称或近身第三视角，内置丰富的内心独白弹幕吐槽。',
        techniques: ['心理弹幕吐槽', '表面装傻内心明镜', '反差萌内心戏'],
      },
      dialogue_density: {
        level: 85,
        summary: '极高对白占比，角色间接梗接梗不断，言语机智生动。',
        techniques: ['连环接梗抛梗', '阴阳怪气调侃', '真情流露反差'],
      },
      sensory_description: {
        level: 50,
        summary: '重在生活细节与美食香气、晨光微风等温馨感官烘托。',
        techniques: ['烟火气细节', '街角咖啡香气', '懒洋洋午后阳光'],
      },
      scene_pacing: {
        level: 60,
        summary: '小事件高频推进，轻松无压力，主线隐匿在日常互动中。',
        techniques: ['日常小剧场连缀', '意外事件破冰', '温馨搞笑收尾'],
      },
      emotional_expression: {
        level: 65,
        summary: '积极明亮，笑中带温情，幽默中见真情实感。',
        techniques: ['啼笑皆非的误会', '温暖治愈感动', '猝不及防的甜度'],
      },
      rhetoric_preference: {
        level: 45,
        summary: '生动现代比喻，网梗与生动口语妙用。',
        techniques: ['当代生动比喻', '反套路调侃', '生活化修辞'],
      },
      lexical_texture: {
        level: 40,
        summary: '年轻态现代汉语，通俗鲜活，口语化极强。',
        techniques: ['鲜活口语名词', '流行语境词汇', '轻量化表达'],
      },
      tension_hook: {
        level: 50,
        summary: '章末常留尴尬或令人捧腹的修罗场现场，让人会心一笑。',
        techniques: ['修罗场撞破尴尬', '一记搞笑反转', '意外发现的小秘密'],
      },
    },
    prohibitions: [
      '严禁沉重压抑的虐心致郁情节',
      '严禁低俗恶俗的烂俗段子',
      '严禁破坏轻松温馨基调的突兀血腥',
    ],
  },
]
