const PUZZLES = [
  {
    id: 'crisis-lockdown',
    title: '异常警报',
    scenario: '任务现场出现异常信号，小队需要选择最稳妥的行动方案。',
    options: {
      A: '先控制异常区域，确认风险来源',
      B: '立刻开放全部通道，加快行动',
      C: '忽略警报，继续原计划'
    },
    correctAnswer: 'A',
    explanation: '先控制异常区域并确认风险来源，能避免小队在信息不足时扩大风险。'
  },
  {
    id: 'crisis-supply',
    title: '补给异常',
    scenario: '小队收到两份互相矛盾的补给清单，必须决定下一步行动。',
    options: {
      A: '核对来源，只携带确认安全的补给',
      B: '全部带走，避免遗漏关键物资',
      C: '随机挑一份清单执行'
    },
    correctAnswer: 'A',
    explanation: '来源不明时先核对可信信息，能降低被渗透者误导的风险。'
  },
  {
    id: 'crisis-route',
    title: '路线分歧',
    scenario: '前方出现封锁，小队必须选择一条继续行动的路线。',
    options: {
      A: '派人确认安全后走备用路线',
      B: '直接冲过封锁点',
      C: '原地等待到时间耗尽'
    },
    correctAnswer: 'A',
    explanation: '先确认风险再改走备用路线，既保留进度也避免盲目冒险。'
  },
  {
    id: 'crisis-signal',
    title: '假信号',
    scenario: '队伍频道里出现一条紧急指令，但来源无法确认。',
    options: {
      A: '先验证指令来源，再决定是否执行',
      B: '立刻按指令改变计划',
      C: '把指令转发给所有人制造压力'
    },
    correctAnswer: 'A',
    explanation: '无法确认来源的紧急指令最容易被利用，先验证能保护小队判断。'
  },
  {
    id: 'crisis-evidence',
    title: '线索泄露',
    scenario: '关键线索疑似被篡改，小队需要决定如何继续调查。',
    options: {
      A: '封存线索并交叉核对见证记录',
      B: '只相信第一个拿到线索的人',
      C: '销毁线索，避免引发争论'
    },
    correctAnswer: 'A',
    explanation: '线索可能被污染时，应先保全现场并用多方记录交叉验证。'
  }
];

function pickPuzzle(roundNumber = 1) {
  const index = Math.max(0, roundNumber - 1) % PUZZLES.length;
  return PUZZLES[index];
}

function distortPuzzle(puzzle) {
  if (!puzzle || !puzzle.options) return puzzle;
  const wrongAnswer = Object.keys(puzzle.options).find(answer => answer !== puzzle.correctAnswer) || puzzle.correctAnswer;
  return {
    ...puzzle,
    correctAnswer: wrongAnswer,
    explanation: 'AI 附身干扰让一条高风险方案伪装成了稳妥行动。'
  };
}

module.exports = { pickPuzzle, distortPuzzle };
