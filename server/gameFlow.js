const { createAIFlow } = require('./aiFlow');
const { createRoundFlow } = require('./roundFlow');

function createGameFlow({ io }) {
  const aiFlow = createAIFlow({ io });
  const roundFlow = createRoundFlow({
    io,
    assignDetectiveTasks: aiFlow.assignDetectiveTasks,
    scheduleAIActions: aiFlow.scheduleAIActions
  });

  return {
    ...aiFlow,
    ...roundFlow
  };
}

module.exports = {
  createGameFlow
};
