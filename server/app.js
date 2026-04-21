const express = require('express');
const http = require('http');
const socketIO = require('socket.io');
const cors = require('cors');
const path = require('path');

const { createGameFlow } = require('./gameFlow');
const { registerRoomHandlers } = require('./handlers/roomHandlers');

// ─── 全局异常安全网 ───
process.on('uncaughtException', (err) => {
  console.error('[FATAL] 未捕获异常:', err.message);
  console.error(err.stack);
  // 不退出进程，继续运行（生产环境可改为 graceful shutdown）
});

process.on('unhandledRejection', (reason) => {
  console.error('[FATAL] 未处理的 Promise 拒绝:', reason);
});

// CORS 配置 - 限制允许的来源
const allowedOrigins = process.env.ALLOWED_ORIGINS 
  ? process.env.ALLOWED_ORIGINS.split(',') 
  : ['http://localhost:3000', 'http://127.0.0.1:3000'];

const app = express();
const server = http.createServer(app);
const io = socketIO(server, {
  cors: {
    origin: allowedOrigins,
    methods: ['GET', 'POST'],
    credentials: true
  }
});

const gameFlow = createGameFlow({ io });

// Express CORS 中间件
app.use(cors({
  origin: allowedOrigins,
  credentials: true
}));
app.use(express.static(path.join(__dirname, '../client')));
app.get('/favicon.ico', (_req, res) => res.status(204).end());

// ─── Express 错误处理中间件 ───
app.use((err, _req, res, _next) => {
  console.error('[Express Error]', err.message);
  res.status(500).json({ error: '服务器内部错误' });
});

// ─── Socket.IO 连接 ───
io.on('connection', (socket) => {
  console.log('玩家连接:', socket.id);

  // 增强 socket.on 以自动包装错误处理
  const originalOn = socket.on.bind(socket);
  socket.on = function (event, handler) {
    const wrappedHandler = function (...args) {
      try {
        const result = handler.apply(this, args);
        if (result && typeof result.catch === 'function') {
          result.catch(err => {
            console.error(`[Socket Async Error] ${event}:`, err.message);
            socket.emit('error', '服务器处理出错，请重试');
          });
        }
      } catch (err) {
        console.error(`[Socket Error] ${event}:`, err.message);
        socket.emit('error', '服务器处理出错，请重试');
      }
    };
    return originalOn(event, wrappedHandler);
  };

  registerRoomHandlers({
    socket,
    io,
    gameFlow
  });
});

const PORT = process.env.PORT || 3000;
server.listen(PORT, () => {
  console.log(`服务器运行在端口 ${PORT}`);
});
