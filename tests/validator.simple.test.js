/**
 * 输入验证简单测试
 */

const {
  validateRoomId,
  validatePlayerName,
  validateChatMessage,
  validateMemberIds,
  validateVote
} = require('../server/validator');

// 测试函数
function test(name, fn) {
  try {
    fn();
    console.log(`✅ ${name}`);
  } catch (err) {
    console.log(`❌ ${name}: ${err.message}`);
  }
}

// 断言函数
function assert(condition, message) {
  if (!condition) {
    throw new Error(message || '断言失败');
  }
}

// 运行测试
console.log('开始测试...\n');

// 测试房间ID验证
test('房间ID验证 - 空值', () => {
  const result = validateRoomId('');
  assert(!result.valid, '应该拒绝空房间ID');
  assert(result.error === '房间ID不能为空', '错误消息不正确');
});

test('房间ID验证 - 过长', () => {
  const result = validateRoomId('a'.repeat(21));
  assert(!result.valid, '应该拒绝过长的房间ID');
  assert(result.error === '房间ID长度必须在1-20之间', '错误消息不正确');
});

test('房间ID验证 - 特殊字符', () => {
  const result = validateRoomId('room@123');
  assert(!result.valid, '应该拒绝包含特殊字符的房间ID');
  assert(result.error === '房间ID只能包含字母、数字、下划线和连字符', '错误消息不正确');
});

test('房间ID验证 - 有效值', () => {
  const result = validateRoomId('room-123');
  assert(result.valid, '应该接受有效的房间ID');
  assert(result.value === 'room-123', '值不正确');
});

test('房间ID验证 - 修剪空格', () => {
  const result = validateRoomId('  room1  ');
  assert(result.valid, '应该修剪空格');
  assert(result.value === 'room1', '值不正确');
});

// 测试玩家昵称验证
test('玩家昵称验证 - 空值', () => {
  const result = validatePlayerName('');
  assert(!result.valid, '应该拒绝空昵称');
  assert(result.error === '昵称不能为空', '错误消息不正确');
});

test('玩家昵称验证 - 过长', () => {
  const result = validatePlayerName('a'.repeat(11));
  assert(!result.valid, '应该拒绝过长的昵称');
  assert(result.error === '昵称长度必须在1-10之间', '错误消息不正确');
});

test('玩家昵称验证 - 敏感词', () => {
  const result = validatePlayerName('admin');
  assert(!result.valid, '应该拒绝敏感词');
  assert(result.error === '昵称包含不允许的词汇', '错误消息不正确');
});

test('玩家昵称验证 - 有效值', () => {
  const result = validatePlayerName('玩家1');
  assert(result.valid, '应该接受有效的昵称');
  assert(result.value === '玩家1', '值不正确');
});

// 测试聊天消息验证
test('聊天消息验证 - 空值', () => {
  const result = validateChatMessage('');
  assert(!result.valid, '应该拒绝空消息');
  assert(result.error === '消息不能为空', '错误消息不正确');
});

test('聊天消息验证 - 过长', () => {
  const result = validateChatMessage('a'.repeat(101));
  assert(!result.valid, '应该拒绝过长的消息');
  assert(result.error === '消息长度必须在1-100之间', '错误消息不正确');
});

test('聊天消息验证 - 有效值', () => {
  const result = validateChatMessage('大家好！');
  assert(result.valid, '应该接受有效的消息');
  assert(result.value === '大家好！', '值不正确');
});

// 测试成员ID验证
test('成员ID验证 - 非数组', () => {
  const result = validateMemberIds('not array');
  assert(!result.valid, '应该拒绝非数组');
  assert(result.error === '成员ID必须是数组', '错误消息不正确');
});

test('成员ID验证 - 空数组', () => {
  const result = validateMemberIds([]);
  assert(!result.valid, '应该拒绝空数组');
  assert(result.error === '成员数量必须在1-6之间', '错误消息不正确');
});

test('成员ID验证 - 有效值', () => {
  const result = validateMemberIds(['player1', 'player2']);
  assert(result.valid, '应该接受有效的成员ID');
  assert(result.value.length === 2, '值不正确');
});

// 测试投票验证
test('投票验证 - 非布尔值', () => {
  const result = validateVote('true');
  assert(!result.valid, '应该拒绝非布尔值');
  assert(result.error === '投票必须是布尔值', '错误消息不正确');
});

test('投票验证 - true', () => {
  const result = validateVote(true);
  assert(result.valid, '应该接受true');
  assert(result.value === true, '值不正确');
});

test('投票验证 - false', () => {
  const result = validateVote(false);
  assert(result.valid, '应该接受false');
  assert(result.value === false, '值不正确');
});

console.log('\n测试完成！');
