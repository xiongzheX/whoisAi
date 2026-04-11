const io = require('socket.io-client');

const SERVER_URL = 'http://localhost:3000';
const ROOM_ID = 'room1';
const PLAYER_NAMES = ['小明', '小红', '小李', '小王', '小张'];

// 模拟5个玩家
async function simulateGame() {
  const players = [];
  
  console.log('开始模拟5个真人玩家...\n');

  // 连接5个玩家
  for (let i = 0; i < PLAYER_NAMES.length; i++) {
    const socket = io(SERVER_URL, {
      transports: ['websocket'],
      reconnection: false
    });
    
    await new Promise((resolve, reject) => {
      socket.on('connect', () => {
        console.log(`✓ ${PLAYER_NAMES[i]} 已连接`);
        
        socket.emit('joinRoom', { roomId: ROOM_ID, name: PLAYER_NAMES[i] });
        
        const player = {
          id: socket.id,
          name: PLAYER_NAMES[i],
          socket,
          description: ''
        };
        players.push(player);
        
        resolve();
      });
      
      socket.on('connect_error', (err) => {
        console.error(`❌ ${PLAYER_NAMES[i]} 连接失败:`, err.message);
        reject(err);
      });
      
      socket.on('disconnect', () => {
        console.log(`❌ ${PLAYER_NAMES[i]} 断开连接`);
      });
      
      // 监听游戏事件
      socket.on('gameStarted', ({ word, players: gamePlayers }) => {
        console.log(`\n🎮 游戏开始！描述词: ${word}`);
        console.log(`玩家列表:`, gamePlayers.map(p => p.name));
        
        // 提交描述
        setTimeout(() => {
          const descriptions = ['常见的水果', '很好吃的东西', '甜甜的', '红色的', '夏天爱吃'];
          const currentDesc = descriptions[i % descriptions.length];
          const currentP = players.find(p => p.name === PLAYER_NAMES[i]);
          if (currentP) {
            currentP.description = currentDesc;
            socket.emit('submitDescription', { roomId: ROOM_ID, description: currentDesc });
            console.log(`${currentP.name} 提交描述: ${currentDesc}`);
          }
        }, 1000 + Math.random() * 2000);
      });

      socket.on('descriptionSubmitted', ({ playerId, description }) => {
        const p = players.find(pl => pl.id === playerId);
        if (p) {
          console.log(`${p.name} 提交了描述: ${description}`);
        }
      });

      socket.on('phaseChange', ({ phase }) => {
        console.log(`\n📢 进入阶段: ${phase}`);
        
        if (phase === 'questioning1' || phase === 'questioning2') {
          // AI开始提问
          setTimeout(() => aiAskQuestion(players, phase), 2000);
        }
      });

      socket.on('questionAsked', ({ questionerId, targetId, question }) => {
        const q = players.find(p => p.id === questionerId);
        const t = players.find(p => p.id === targetId);
        if (q && t) {
          console.log(`\n❓ ${q.name} 问 ${t.name}: ${question}`);
          
          // 如果是被问的玩家，自动回答
          if (t.id === player.id) {
            setTimeout(() => {
              const answers = ['我觉得还好', '大概是那个意思', '不太清楚', '应该差不多'];
              const answer = answers[Math.floor(Math.random() * answers.length)];
              console.log(`${player.name} 回答: ${answer}`);
              // 注意：这里需要实现回答接口，目前先打印
            }, 1000);
          }
        }
      });

      socket.on('voteResult', ({ voteResults, defendingPlayers }) => {
        console.log(`\n🗳️ 投票结果:`);
        players.forEach(p => {
          console.log(`  ${p.name}: ${voteResults[p.id] || 0}票`);
        });
        
        const defending = players.filter(p => defendingPlayers.includes(p.id));
        console.log(`辩解玩家: ${defending.map(p => p.name).join(', ')}`);
      });

      socket.on('playerDefended', ({ playerId, statement }) => {
        const p = players.find(pl => pl.id === playerId);
        if (p) {
          console.log(`\n🎤 ${p.name} 辩解: ${statement}`);
        }
      });

      socket.on('gameFinished', ({ winner, voteResults }) => {
        console.log(`\n🎉 游戏结束！`);
        console.log(`胜利者: ${winner === 'humans' ? '真人' : 'AI'}`);
        
        process.exit(0);
      });
    });
    
    // 等待连接
    await new Promise(resolve => setTimeout(resolve, 500));
  }

  console.log('\n所有玩家已连接，等待游戏开始...\n');
}

// 模拟AI提问
function aiAskQuestion(players, phase) {
  // 这里模拟真人提问，实际游戏中真人会自己提问
  const askers = players.filter(p => !p.name.includes('AI_'));
  if (askers.length > 0) {
    const asker = askers[Math.floor(Math.random() * askers.length)];
    const targets = players.filter(p => p.id !== asker.id);
    const target = targets[Math.floor(Math.random() * targets.length)];
    const questions = ['你的描述是什么意思？', '能具体说说吗？', '你觉得这个词有什么特点？', '你是怎么想的？', '能不能详细点？'];
    const question = questions[Math.floor(Math.random() * questions.length)];
    
    console.log(`模拟 ${asker.name} 提问 ${target.name}: ${question}`);
    // 注意：这里需要实现提问接口
  }
}

simulateGame();
