const { AI_CONFIG, AI_PERSONAS, FLAW_TYPES, checkShouldShowFlaw } = require('./gameData');

function pickRandom(items) {
  return items[Math.floor(Math.random() * items.length)];
}

function getPersonaProfile(persona) {
  return AI_PERSONAS[persona] || AI_PERSONAS.cautious;
}

function compactText(text) {
  return String(text || '').replace(/\s+/g, ' ').trim();
}

function applyPersonaVoice(text, persona, category = 'statement') {
  const profile = getPersonaProfile(persona);
  let output = compactText(text);

  const prefixes = {
    analytical: {
      description: ['先从结构看，', '如果按逻辑拆开看，', '我先把结论说清楚：'],
      defense: ['先别急着下结论，', '把逻辑捋一下，', '我先回应最关键的点：'],
      question: ['我想追问一下，', '我先确认一个点，', '从你的说法看，']
    },
    cautious: {
      description: ['我先保留一点判断，', '我现在只能先这么说，', '我倾向于认为：'],
      defense: ['我不想把话说太满，', '我先解释到这里，', '我先把态度摆明：'],
      question: ['我有个小问题，', '我先轻轻问一句，', '我想确认一下，']
    },
    confrontational: {
      description: ['我直接说，', '别绕了，', '先把话挑明：'],
      defense: ['先别急着盯我，', '我看你们更像在带节奏，', '这票有点离谱，'],
      question: ['你先回答我，', '我倒要问一句，', '别急着反问，']
    },
    quirky: {
      description: ['我有点像在打盹时想到的，', '这个感觉有点怪，但我先说，', '像一颗弹跳的石子，'],
      defense: ['这个局面有点像反着走的钟，', '我先把想法抖出来，', '听我把这团线理一下，'],
      question: ['我突然想到一个问题，', '这个问题有点像拐了个弯，', '我先随手问一句，']
    }
  };

  const suffixes = {
    analytical: {
      description: ['这更像是结构上的判断。', '结论大概就是这样。'],
      defense: ['所以我认为这个判断还不够稳。', '我觉得现在票我太早了。'],
      question: ['你可以把上下文也一起说清楚。', '我更想看你的逻辑链。']
    },
    cautious: {
      description: ['大概就是这种感觉。', '我先说到这里。'],
      defense: ['我还是建议你们再看一轮。', '现在下结论真的太早。'],
      question: ['如果你愿意，可以补充一点。', '我想先听你的解释。']
    },
    confrontational: {
      description: ['你们应该能听懂吧。', '我已经说得很直白了。'],
      defense: ['先把你们自己的判断说清楚。', '别急着把锅扣到我头上。'],
      question: ['你最好直接回答。', '别把话题往别人身上带。']
    },
    quirky: {
      description: ['差不多就是这样，像一段半醒的梦。', '我觉得大概是这个味道。'],
      defense: ['这局面有点滑稽，但我真不是。', '我得先把脑子里的线头捋顺。'],
      question: ['你这个回答让我有点在意。', '你刚才那句有点像绕了个圈。']
    }
  };

  const voicePrefix = pickRandom(prefixes[profile.name]?.[category] || prefixes.cautious[category] || ['']);
  const voiceSuffix = pickRandom(suffixes[profile.name]?.[category] || suffixes.cautious[category] || ['']);

  output = `${voicePrefix}${output}${voiceSuffix ? ` ${voiceSuffix}` : ''}`.replace(/\s+/g, ' ').trim();

  if (profile.name === 'analytical' && output.length > 42) {
    output = output.replace(/。+/g, '。');
  }
  if (profile.name === 'confrontational') {
    output = output.replace(/\?$/, '？');
  }
  if (profile.name === 'quirky' && !/[，。！？]$/.test(output)) {
    output = `${output}。`;
  }

  return output;
}

function applyEventFlavor(text, persona, actorId, roundEvent) {
  if (!roundEvent || (actorId && roundEvent.targetId && roundEvent.targetId !== actorId)) {
    return text;
  }

  const profile = getPersonaProfile(persona);
  if (roundEvent.type === 'glitch') {
    const glitchText = profile.name === 'analytical'
      ? '我再把这个点拆细一点，免得说偏。'
      : profile.name === 'confrontational'
        ? '别抓我这个细节，我刚才已经说得够清楚了。'
        : profile.name === 'quirky'
          ? '这个地方有点打结了，但方向还是这个方向。'
          : '这句话我先留一点余地。';
    return `${text} ${glitchText}`;
  }

  if (roundEvent.type === 'echo') {
    const echoText = profile.name === 'confrontational'
      ? '我还是这个判断，你们别被表面带跑。'
      : profile.name === 'analytical'
        ? '我还是坚持这个推理链。'
        : profile.name === 'quirky'
          ? '我还是这么觉得，像回声一样。'
          : '我目前还是这个看法。';
    return `${text} ${echoText}`;
  }

  if (roundEvent.type === 'tempo_shift') {
    const tempoText = profile.name === 'analytical'
      ? '重点先放这里。'
      : profile.name === 'confrontational'
        ? '我直接说重点。'
        : profile.name === 'quirky'
          ? '先抛个结论。'
          : '我先说重点。';
    return `${text} ${tempoText}`;
  }

  return text;
}

function personaFlawBias(persona) {
  const profile = getPersonaProfile(persona);
  return profile.riskBias || 0;
}

function aiAnswerToSkillQuestion(level, question, persona = 'cautious', actorId = null, roundEvent = null) {
  const profile = getPersonaProfile(persona);
  let answers;

  if (level === 'normal') {
    answers = [
      '我刚才说的意思你应该懂吧？',
      '这个挺简单的，没那么多说法',
      '我觉得没问题啊',
      '你想太多了'
    ];
  } else if (level === 'advanced') {
    answers = [
      '这个问题挺有意思的，我想想...',
      '其实我的意思没那么复杂',
      '你从我的描述里能看到什么破绽吗？',
      '我觉得这问题问得好，不过我确实没什么好隐瞒的'
    ];
  } else {
    answers = [
      '你为什么这么问？是对我有怀疑吗？',
      '这个问题太直接了吧，我想知道你为什么这么问',
      '我觉得你的提问方式有点问题',
      '我倒要看看你能问出什么'
    ];
  }

  const personaAnswers = {
    analytical: [
      '我先拆一下这个问题，结论其实不复杂。',
      '你的提问方向有意思，但我觉得证据还不够。',
      '我理解你的怀疑，不过逻辑上还不能直接下结论。',
      '如果你要追问，可以先看前面的描述。'
    ],
    cautious: [
      '我先保留一点判断，再看后面的情况。',
      '这个问题我不想答得太死。',
      '我觉得现在下结论还太早了。',
      '你可以继续问，但我不想暴露太多。'
    ],
    confrontational: [
      '你先别急着问我，先解释你自己的判断。',
      '你为什么一直盯着我？',
      '这个问题反而说明你自己有点心虚。',
      '我觉得你现在更可疑。'
    ],
    quirky: [
      '这个问题有点像把饼干放进茶里，怪但不是不能理解。',
      '我一时想到了别的方向，先别急着判我。',
      '你问得挺突然，我得先理一理。',
      '我觉得这问题像雾里看花，不过也许你有你的道理。'
    ]
  };

  const personaPool = personaAnswers[profile.name] || answers;
  let reply = pickRandom(personaPool);
  reply = applyPersonaVoice(reply, persona, 'question');

  if (question && /为什么|怎么|哪里|谁/.test(question) && profile.name === 'analytical') {
    reply = `${reply} 这类问题更适合从上下文里看。`;
  }

  if (roundEvent) {
    reply = applyEventFlavor(reply, persona, actorId, roundEvent);
  }

  return reply;
}

function generateAIDescription(word, level = 'normal', difficulty = 'medium', pressure = 0, persona = 'cautious', roundEvent = null, actorId = null) {
  const config = AI_CONFIG[level];
  const style = config.descriptionStyle;
  const profile = getPersonaProfile(persona);
  const showFlaw = checkShouldShowFlaw(level, pressure);

  const simpleTemplates = [
    `是一种${word.length > 2 ? '常见' : '美味'}的东西`,
    `我觉得${word}挺不错的`,
    `${word}在生活中很常见`,
    `这个跟${word}有关`
  ];

  const mixedTemplates = [
    `是一种${word.length > 2 ? '常见' : '美味'}的东西`,
    `${word}在生活中很常见`,
    `这个东西很有特点`,
    `我觉得${word}挺好的`,
    `应该是大家都知道的${word.length > 2 ? '物品' : '东西'}`
  ];

  const complexTemplates = [
    `这个嘛，我觉得${word}确实很有意思`,
    `${word}在日常生活中还是比较常见的`,
    `我想表达的是，${word}这个概念挺重要的`,
    `关于${word}，我想说它确实不错`,
    `应该是那种${word.length > 2 ? '很有特色的' : '比较常见'}的${word.length > 2 ? '概念' : '东西'}`
  ];

  let templates = simpleTemplates;
  if (style === 'mixed') {
    templates = mixedTemplates;
  } else if (style === 'complex') {
    templates = complexTemplates;
  }

  const personaTweaks = {
    analytical: [
      `它的核心特征比较明显，我先这么理解。`,
      `如果按结构看，大概就是这个方向。`
    ],
    cautious: [
      `我先说到这里，免得说太满。`,
      `大概就是这个感觉。`
    ],
    confrontational: [
      `你们应该能听懂我这个意思吧？`,
      `我说得已经很直白了。`
    ],
    quirky: [
      `感觉像是那种会突然出现在桌上的东西。`,
      `有点像你一眼能认出来的那类。`
    ]
  };

  let description = pickRandom(templates);
  description = `${description} ${pickRandom(personaTweaks[profile.name] || personaTweaks.cautious)}`;
  description = applyPersonaVoice(description, persona, 'description');

  if (showFlaw) {
    const flawType = Object.keys(FLAW_TYPES)[Math.floor(Math.random() * 3)];

    if (flawType === 'logic') {
      if (difficulty === 'high' && word.length <= 2) {
        description = `这是一个非常复杂的${word}，有很多方面的讨论`;
      } else if (difficulty === 'low' && word.length > 2) {
        description = `简单来说就是个${word}，没什么复杂的`;
      }
    } else if (flawType === 'knowledge') {
      if (difficulty === 'high') {
        description = `这个${word}很有历史背景，很多人都不知道它的深层含义`;
      }
    } else if (flawType === 'tone') {
      description = `我觉得${word}挺好的，真的挺好的，挺不错的`;
    }
  }

  description = applyEventFlavor(description, persona, actorId, roundEvent);

  if (roundEvent && roundEvent.type === 'glitch' && roundEvent.targetId === actorId) {
    description = `${description} 这个细节我可能有点说偏了。`;
  }

  return description;
}

function generateAIDefense(aiLevel, persona = 'cautious', actorId = null, roundEvent = null) {
  const profile = getPersonaProfile(persona);
  let statements;

  if (aiLevel === 'normal') {
    statements = [
      '我真的不是AI，你们误会了',
      '我只是表达比较简单而已',
      '你们凭什么觉得我是AI？',
      '我觉得投票有问题',
      '我是真玩家，别冤枉我'
    ];
  } else if (aiLevel === 'advanced') {
    statements = [
      '你们是不是对我有偏见？我说的每句话都很正常啊',
      '可惜了，本来还指望他能带我赢的...',
      '我真的不是AI，只是表达方式不同',
      '你们应该重新考虑一下，这个投票结果不太合理',
      '我想证明我是人类，但你们好像已经认定了'
    ];
  } else {
    statements = [
      '我看谁投我了？这不公平吧',
      '你们是不是串通好了？',
      '我觉得某人反而像AI，你们为什么不怀疑他？',
      '我被冤枉了，真没想到会这样',
      '我倒要看看最后是谁赢，别高兴太早'
    ];
  }

  const personaStatements = {
    analytical: [
      '先别急着投我，咱们把逻辑捋清楚。',
      '如果你们看上下文，我的说法其实挺一致。',
      '你们的判断依据还不够稳。',
      '我觉得这个票型有点草率。'
    ],
    cautious: [
      '我先不把话说太满，但我真不是AI。',
      '我觉得你们需要再看一轮。',
      '现在下结论还太早了。',
      '别急着把我当目标。'
    ],
    confrontational: [
      '你们先解释一下为什么盯着我。',
      '投我之前，先看看自己是不是更可疑。',
      '这票投得也太快了吧。',
      '我觉得你们在带节奏。'
    ],
    quirky: [
      '我感觉这局像在看一场倒着放的电影。',
      '你们投我之前，先确认一下是不是被表象骗了。',
      '我真不是AI，但这个局面有点荒诞。',
      '我现在很像被一锅端的那个。'
    ]
  };

  let statement = pickRandom(personaStatements[profile.name] || statements);
  statement = applyPersonaVoice(statement, persona, 'defense');
  statement = applyEventFlavor(statement, persona, actorId, roundEvent);

  if (roundEvent && roundEvent.type === 'echo' && roundEvent.targetId === actorId) {
    statement = `${statement} 我还是这个意思。`;
  }

  return statement;
}

module.exports = {
  aiAnswerToSkillQuestion,
  generateAIDescription,
  generateAIDefense,
  applyPersonaVoice,
  applyEventFlavor,
  personaFlawBias
};
