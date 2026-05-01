const TOPICS = [
  '谁更适合进入本轮维护小队？',
  '刚才的投票里谁最像在隐藏真实意图？',
  '本轮如果出现 AI 附身，最可能影响哪类发言？',
  '小队需要优先解释哪些维护风险？',
  '哪些玩家的判断前后一致，哪些玩家在摇摆？'
];

function getTopics(count) {
  const result = [];
  for (let i = 0; i < count; i++) {
    result.push(TOPICS[i % TOPICS.length]);
  }
  return result;
}

module.exports = { getTopics };
