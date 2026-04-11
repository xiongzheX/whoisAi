const { rooms } = require('./state');
const { getRandomAILevel, getRandomAIPersona, DETECTIVE_TASKS, AI_CONFIG } = require('./gameData');
const { getUniqueAIName } = require('./roomService');

function createAIFlow({ io }) {
  function chooseAIAction(ai, roundEvent = null) {
    const actionPools = {
      normal: ['observe', 'observe', 'listen'],
      advanced: ['question', 'observe', 'question'],
      spy: ['listen', 'question', 'observe']
    };

    const personaPools = {
      analytical: ['observe', 'question', 'observe'],
      cautious: ['listen', 'observe', 'observe'],
      confrontational: ['question', 'question', 'listen'],
      quirky: ['observe', 'listen', 'question']
    };

    const basePool = actionPools[ai.aiLevel] || actionPools.normal;
    const personaPool = personaPools[ai.persona] || basePool;
    const eventPools = {
      glitch: ['observe', 'observe', 'question'],
      tempo_shift: ['question', 'question', 'listen'],
      echo: ['listen', 'observe', 'listen']
    };
    const eventPool = roundEvent ? (eventPools[roundEvent.type] || []) : [];
    const combinedPool = [...basePool, ...personaPool, ...eventPool];
    return combinedPool[Math.floor(Math.random() * combinedPool.length)];
  }

  function assignDetectiveTasks(roomId) {
    const room = rooms[roomId];
    room.playerTasks = {};
    room.detectiveSkills = {};

    room.players.forEach(player => {
      if (!player.isAI) {
        const task = DETECTIVE_TASKS[Math.floor(Math.random() * DETECTIVE_TASKS.length)];
        room.playerTasks[player.id] = {
          ...task,
          completed: false,
          progress: 0
        };
        room.detectiveSkills[player.id] = {
          observe: 3,
          question: 3,
          listen: 1
        };
      }
    });

    console.log(`已分配侦探任务给 ${Object.keys(room.playerTasks).length} 位玩家`);
  }

  function addAIToRoom(roomId, level = null) {
    const room = rooms[roomId];
    const aiLevel = level || getRandomAILevel();
    const persona = getRandomAIPersona(aiLevel);
    const ai = {
      id: `ai_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`,
      name: getUniqueAIName(room),
      isAI: true,
      aiLevel,
      persona,
      description: '',
      position: room.players.length,
      isCooperating: false,
      cooperationTarget: null,
      currentFlaw: null
    };

    room.players.push(ai);
    room.questionCount[ai.id] = 3;
    room.aiPressure[ai.id] = 0;

    io.to(roomId).emit('playerJoined', {
      players: room.players
    });

    console.log(`添加AI: ${ai.name}, 等级: ${aiLevel}`);
  }

  function scheduleAIActions(roomId, recordRoundAction) {
    const room = rooms[roomId];
    if (!room) return;

    const tempoMultiplier = room.roundTempoMultiplier || 1;
    const roundEvent = room.roundEvent;

    room.players.filter(p => p.isAI).forEach(ai => {
      const config = AI_CONFIG[ai.aiLevel] || AI_CONFIG.normal;
      const baseDelay = config.delayMin + Math.random() * (config.delayMax - config.delayMin);
      const eventTempo = roundEvent?.type === 'tempo_shift' ? 0.75 : 1;
      const delay = Math.max(150, baseDelay * tempoMultiplier * eventTempo);
      setTimeout(() => {
        if (!rooms[roomId] || rooms[roomId].matchResolved || rooms[roomId].status !== 'action') return;
        if (room.roundActions[ai.id]) return;

        const targets = room.players.filter(p => p.id !== ai.id);
        const target = targets[Math.floor(Math.random() * targets.length)];
        if (!target) {
          recordRoundAction(roomId, ai.id, 'skip', null, 'AI 跳过');
          return;
        }

        const actionType = chooseAIAction(ai, roundEvent);

        if (actionType === 'observe') {
          io.to(roomId).emit('skillUsed', {
            playerId: ai.id,
            skillType: 'observe',
            targetId: target.id,
            description: target.description
          });
          recordRoundAction(roomId, ai.id, 'observe', target.id, `观察了 ${target.name}`);
        } else if (actionType === 'question') {
          const questionPool = {
            analytical: [
              `你刚才为什么会提到 ${room.currentWord}?`,
              `你能把刚才那句再解释清楚一点吗？`,
              '你这句和前面的逻辑有点接不上。'
            ],
            cautious: [
              '你为什么会这样描述？',
              '你刚才是不是漏说了什么？',
              '我想再确认一下你的意思。'
            ],
            confrontational: [
              '你先回答我，你为什么会这么说？',
              '你是不是在故意转移话题？',
              '别绕了，直接说重点。'
            ],
            quirky: [
              '你刚才那句听起来有点怪，能再说一遍吗？',
              '你是不是把别的词带进来了？',
              '我总觉得你那句像拐了个弯。'
            ]
          };
          const question = questionPool[ai.persona]?.[Math.floor(Math.random() * questionPool[ai.persona].length)]
            || `你刚才为什么会提到 ${room.currentWord}?`;
          io.to(roomId).emit('questionAsked', {
            questionerId: ai.id,
            targetId: target.id,
            question,
            remainingQuestions: room.questionCount
          });
          recordRoundAction(roomId, ai.id, 'question', target.id, question);
        } else {
          io.to(roomId).emit('skillUsed', {
            playerId: ai.id,
            skillType: 'listen',
            targetId: target.id,
            targetName: target.name,
            isAI: target.isAI,
            hint: roundEvent?.type === 'echo'
              ? (target.isAI ? '这段说法有点在重复回响...' : '这段说法挺稳定的。')
              : target.isAI
                ? '这个玩家的行为有点可疑...'
                : '这个玩家表现很自然'
          });
          recordRoundAction(roomId, ai.id, 'listen', target.id, `偷听了 ${target.name}`);
        }
      }, delay);
    });
  }

  return {
    assignDetectiveTasks,
    addAIToRoom,
    scheduleAIActions
  };
}

module.exports = {
  createAIFlow
};
