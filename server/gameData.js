const GAME_MODE_CONFIG = {
  normal: {
    minPlayers: 6,
    autoFillAI: false,
    autoStart: false,
    introDelay: 1200,
    roundDelay: 1200,
    aiActionDelayMin: 800,
    aiActionDelayMax: 2600,
    aiAnswerDelay: 800
  },
  test: {
    minPlayers: 6,
    autoFillAI: true,
    autoStart: true,
    introDelay: 1200,
    roundDelay: 1200,
    aiActionDelayMin: 800,
    aiActionDelayMax: 2600,
    aiAnswerDelay: 800
  },
  solo: {
    minPlayers: 1,
    autoFillAI: true,
    autoStart: true,
    introDelay: 350,
    roundDelay: 300,
    aiActionDelayMin: 150,
    aiActionDelayMax: 450,
    aiAnswerDelay: 250
  }
};

const WORD_DIFFICULTY = {
  low: [
    ['手机', '电话'],
    ['苹果', '香蕉'],
    ['杯子', '碗'],
    ['铅笔', '钢笔'],
    ['桌子', '椅子']
  ],
  medium: [
    ['干洗机', '甩干机'],
    ['相机', '摄影机'],
    ['打火机', '点烟器'],
    ['福尔摩斯', '华生'],
    ['薛定谔', '海森堡']
  ],
  high: [
    ['量子力学', '相对论'],
    ['多巴胺', '内啡肽'],
    ['形而上学', '认识论'],
    ['贝叶斯', '傅里叶'],
    ['哥德尔', '图灵']
  ]
};

const AI_LEVELS = {
  normal: { name: 'normal', chance: 0.25, description: '普通AI' },
  advanced: { name: 'advanced', chance: 0.15, description: '高级AI' },
  spy: { name: 'spy', chance: 0.10, description: '间谍AI' }
};

const AI_CONFIG = {
  normal: {
    delayMin: 1500,
    delayMax: 3000,
    descriptionStyle: 'simple',
    cooperationRate: 0.1
  },
  advanced: {
    delayMin: 2000,
    delayMax: 4000,
    descriptionStyle: 'mixed',
    cooperationRate: 0.3
  },
  spy: {
    delayMin: 2500,
    delayMax: 5000,
    descriptionStyle: 'complex',
    cooperationRate: 0.5
  }
};

const AI_PERSONAS = {
  analytical: {
    name: 'analytical',
    label: '分析型',
    descriptionTone: '更像在拆解信息',
    defenseTone: '会先解释逻辑，再回避关键点',
    questionTone: '喜欢用追问逼近结论',
    speechPattern: 'short',
    riskBias: 0.2
  },
  cautious: {
    name: 'cautious',
    label: '谨慎型',
    descriptionTone: '说话会留余地',
    defenseTone: '先自保，再慢慢解释',
    questionTone: '不太愿意直接给答案',
    speechPattern: 'hedged',
    riskBias: -0.15
  },
  confrontational: {
    name: 'confrontational',
    label: '对抗型',
    descriptionTone: '会主动反问别人',
    defenseTone: '火力更强，容易反咬一口',
    questionTone: '更容易直接顶回去',
    speechPattern: 'sharp',
    riskBias: 0.35
  },
  quirky: {
    name: 'quirky',
    label: '跳脱型',
    descriptionTone: '表达偶尔会偏一点',
    defenseTone: '带一点插科打诨和转移话题',
    questionTone: '回复更像临场发挥',
    speechPattern: 'playful',
    riskBias: 0.05
  }
};

const ROUND_EVENTS = {
  glitch: {
    id: 'glitch',
    label: '语义噪声',
    message: '系统噪声干扰了某位 AI 的措辞，细节更容易露馅。',
    pressureBoost: 28,
    tempoMultiplier: 1,
    focusBias: 'observe',
    personaHint: 'slip'
  },
  tempo_shift: {
    id: 'tempo_shift',
    label: '节奏变化',
    message: '房间节奏被打乱，本轮 AI 行动会更快，发言更容易抢拍。',
    pressureBoost: 0,
    tempoMultiplier: 0.55,
    focusBias: 'question',
    personaHint: 'rush'
  },
  echo: {
    id: 'echo',
    label: '回声效应',
    message: '有一位 AI 的说法开始重复回响，容易被盯上。',
    pressureBoost: 14,
    tempoMultiplier: 0.9,
    focusBias: 'listen',
    personaHint: 'repeat'
  }
};

const FLAW_TYPES = {
  logic: { name: 'logic', description: '逻辑破绽' },
  tone: { name: 'tone', description: '语气破绽' },
  knowledge: { name: 'knowledge', description: '知识破绽' }
};

const DETECTIVE_TASKS = [
  { id: 'find_flaws', description: '找出描述中最多的AI破绽', points: 50 },
  { id: 'first_vote_ai', description: '第一次就猜中AI身份', points: 100 },
  { id: 'accurate_guidance', description: '成功引导他人投票给AI', points: 30 },
  { id: 'quick_detective', description: '在前两轮内找出AI', points: 40 }
];

function getRoomMode(room) {
  return room?.mode || 'normal';
}

function getModeConfig(room) {
  return GAME_MODE_CONFIG[getRoomMode(room)] || GAME_MODE_CONFIG.normal;
}

function getRandomWord(difficulty = 'medium') {
  const wordPairs = WORD_DIFFICULTY[difficulty] || WORD_DIFFICULTY.medium;
  return wordPairs[Math.floor(Math.random() * wordPairs.length)];
}

function getRandomAILevel() {
  const rand = Math.random();
  if (rand < AI_LEVELS.normal.chance) return 'normal';
  if (rand < AI_LEVELS.normal.chance + AI_LEVELS.advanced.chance) return 'advanced';
  return 'spy';
}

function getRandomAIPersona(level = 'normal') {
  const pools = {
    normal: ['cautious', 'analytical', 'quirky'],
    advanced: ['analytical', 'confrontational', 'cautious'],
    spy: ['confrontational', 'quirky', 'analytical']
  };
  const pool = pools[level] || pools.normal;
  return pool[Math.floor(Math.random() * pool.length)];
}

function getRandomRoundEvent() {
  const events = Object.values(ROUND_EVENTS);
  return events[Math.floor(Math.random() * events.length)];
}

function checkShouldShowFlaw(aiLevel, pressure) {
  const baseChance = {
    normal: 0.25,
    advanced: 0.15,
    spy: 0.10
  };
  const pressureMultiplier = pressure / 100;
  return Math.random() < (baseChance[aiLevel] + pressureMultiplier * 0.3);
}

module.exports = {
  GAME_MODE_CONFIG,
  WORD_DIFFICULTY,
  AI_LEVELS,
  AI_CONFIG,
  FLAW_TYPES,
  DETECTIVE_TASKS,
  getRoomMode,
  getModeConfig,
  getRandomWord,
  getRandomAILevel,
  getRandomAIPersona,
  getRandomRoundEvent,
  checkShouldShowFlaw
};
