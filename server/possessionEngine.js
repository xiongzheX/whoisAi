const { POSSESSION_STYLE_KEYS } = require('./gameData');

function randomStyle() {
  return POSSESSION_STYLE_KEYS[Math.floor(Math.random() * POSSESSION_STYLE_KEYS.length)];
}

function shouldPossess(chance = 0.5) {
  return Math.random() < chance;
}

function selectPossessedPlayer(playerIds = []) {
  if (!playerIds.length) return null;
  return playerIds[Math.floor(Math.random() * playerIds.length)];
}

function rewriteMessage(message, style) {
  const text = String(message || '').trim();
  if (!text) return text;

  switch (style) {
    case 'polite':
      return `我倾向于认为，${text}`;
    case 'verbose':
      return `${text}，这个判断和目前的维护风险有关`;
    case 'neutral':
      return text
        .replace(/肯定/g, '可能')
        .replace(/必须/g, '最好')
        .replace(/不行/g, '不太稳');
    case 'awkward':
      return `从系统状态看，${text}`;
    default:
      return text;
  }
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

function getPossessedHint() {
  return '你的诊断信息可能被 AI 信号扭曲，请谨慎核对队友看到的内容。';
}

module.exports = {
  rewriteMessage,
  randomStyle,
  shouldPossess,
  selectPossessedPlayer,
  distortPuzzle,
  getPossessedHint
};
