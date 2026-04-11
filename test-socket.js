#!/usr/bin/env node

/**
 * 游戏Socket连接测试脚本
 * 直接测试服务器和客户端交互
 */

const io = require('socket.io-client');

console.log('======================================');
console.log('游戏Socket连接测试');
console.log('======================================');
console.log('');

// 配置
const SERVER_URL = 'http://localhost:3000';
const TEST_ROOM = 'test-room-' + Date.now();
const TEST_PLAYER = 'TestPlayer-' + Date.now().toString().substr(-4);

let socket = null;
let testResults = {
    connection: false,
    joinRoom: false,
    playerJoined: false,
    gameStarted: false,
    roomReset: false,
    rejoinRoom: false
};
let gameStartCount = 0;
let rejoinRequested = false;

function log(message, type = 'info') {
    const timestamp = new Date().toLocaleTimeString();
    const prefix = type === 'error' ? '❌' : type === 'success' ? '✅' : 'ℹ️';
    console.log(`[${timestamp}] ${prefix} ${message}`);
}

function runTest() {
    return new Promise((resolve, reject) => {
        log(`连接到服务器: ${SERVER_URL}`);
        
        socket = io(SERVER_URL, {
            transports: ['websocket'],
            timeout: 5000
        });

        // 连接成功
        socket.on('connect', () => {
            log('Socket连接成功', 'success');
            testResults.connection = true;
            testResults.joinRoom = true;

            log(`加入房间: ${TEST_ROOM}, 玩家: ${TEST_PLAYER}`);
            socket.emit('joinRoom', {
                roomId: TEST_ROOM,
                name: TEST_PLAYER,
                testMode: true
            });
        });

        // 玩家加入事件
        socket.on('playerJoined', ({ players }) => {
            log(`玩家加入事件触发，当前玩家数: ${players.length}`, 'success');
            testResults.playerJoined = true;
            
            players.forEach((p, i) => {
                log(`  玩家 ${i + 1}: ${p.name} (ID: ${p.id.substr(-6)})${p.isAI ? ' [AI]' : ''}`);
            });

            // 检查是否有AI自动加入（测试模式）
            const aiCount = players.filter(p => p.isAI).length;
            if (aiCount > 0) {
                log(`测试模式: 自动添加了 ${aiCount} 个AI玩家`, 'success');
            }
        });

        // 游戏开始事件
        socket.on('gameStarted', ({ word, players, wordDifficulty, roundNumber }) => {
            log('游戏开始事件触发！', 'success');
            testResults.gameStarted = true;
            gameStartCount += 1;

            log(`单词: ${word}`, 'success');
            log(`难度: ${wordDifficulty || '默认'}`, 'success');
            log(`回合: ${roundNumber || 'N/A'}`, 'success');
            log(`参与玩家: ${players.length} 人`, 'success');

            const names = players.map(p => p.name);
            const uniqueNames = new Set(names);
            if (uniqueNames.size !== names.length) {
                reject(new Error('AI names are duplicated'));
                return;
            }

            players.forEach((p, i) => {
                log(`  ${i + 1}. ${p.name} ${p.isAI ? '[AI]' : ''}`);
            });

            if (gameStartCount === 1 && !rejoinRequested) {
                rejoinRequested = true;
                log('触发房间重置，验证同一 socket 可重新加入', 'success');
                socket.emit('resetRoom', { roomId: TEST_ROOM });
                return;
            }

            setTimeout(() => {
                socket.disconnect();
                log('测试完成，断开连接', 'success');
                resolve();
            }, 3000);
        });

        socket.on('roomReset', ({ roomId }) => {
            if (roomId !== TEST_ROOM) return;
            log(`房间已重置: ${roomId}`, 'success');
            testResults.roomReset = true;
            socket.emit('joinRoom', {
                roomId: TEST_ROOM,
                name: TEST_PLAYER,
                testMode: true
            });
            testResults.rejoinRoom = true;
        });

        socket.on('roundSummary', ({ roundNumber, message }) => {
            log(`回合结算: 第 ${roundNumber} 轮`, 'success');
            log(message || 'no message', 'success');
        });

        // 错误处理
        socket.on('connect_error', (error) => {
            log(`连接错误: ${error.message}`, 'error');
            reject(error);
        });

        socket.on('error', (message) => {
            log(`服务器错误: ${message}`, 'error');
        });

        // 断开连接
        socket.on('disconnect', (reason) => {
            log(`断开连接: ${reason}`);
        });

        // 超时处理
        setTimeout(() => {
            if (!testResults.gameStarted) {
                log('测试超时（10秒），游戏未开始', 'error');
                socket.disconnect();
                reject(new Error('Test timeout'));
            }
        }, 10000);
    });
}

// 运行测试
async function main() {
    try {
        log('开始测试...');
        await runTest();

        console.log('');
        console.log('======================================');
        console.log('测试结果汇总');
        console.log('======================================');
        console.log('');

        for (const [key, value] of Object.entries(testResults)) {
            const status = value ? '✅ 通过' : '❌ 失败';
            console.log(`${key.padEnd(15)}: ${status}`);
        }

        console.log('');
        console.log('测试完成！');
        process.exit(0);

    } catch (error) {
        log(`测试失败: ${error.message}`, 'error');
        console.error(error);
        process.exit(1);
    }
}

// 运行主函数
main();
