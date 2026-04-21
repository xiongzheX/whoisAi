/**
 * 输入验证单元测试
 */

const {
  validateRoomId,
  validatePlayerName,
  validateChatMessage,
  validateMemberIds,
  validateVote
} = require('../server/validator');

// 测试房间ID验证
describe('validateRoomId', () => {
  test('应该拒绝空房间ID', () => {
    const result = validateRoomId('');
    expect(result.valid).toBe(false);
    expect(result.error).toBe('房间ID不能为空');
  });

  test('应该拒绝过长的房间ID', () => {
    const result = validateRoomId('a'.repeat(21));
    expect(result.valid).toBe(false);
    expect(result.error).toBe('房间ID长度必须在1-20之间');
  });

  test('应该拒绝包含特殊字符的房间ID', () => {
    const result = validateRoomId('room@123');
    expect(result.valid).toBe(false);
    expect(result.error).toBe('房间ID只能包含字母、数字、下划线和连字符');
  });

  test('应该接受有效的房间ID', () => {
    const result = validateRoomId('room-123');
    expect(result.valid).toBe(true);
    expect(result.value).toBe('room-123');
  });

  test('应该修剪房间ID的空格', () => {
    const result = validateRoomId('  room1  ');
    expect(result.valid).toBe(true);
    expect(result.value).toBe('room1');
  });
});

// 测试玩家昵称验证
describe('validatePlayerName', () => {
  test('应该拒绝空昵称', () => {
    const result = validatePlayerName('');
    expect(result.valid).toBe(false);
    expect(result.error).toBe('昵称不能为空');
  });

  test('应该拒绝过长的昵称', () => {
    const result = validatePlayerName('a'.repeat(11));
    expect(result.valid).toBe(false);
    expect(result.error).toBe('昵称长度必须在1-10之间');
  });

  test('应该过滤HTML标签', () => {
    const result = validatePlayerName('<script>alert(1)</script>');
    expect(result.valid).toBe(true);
    expect(result.value).toBe('scriptalert(1)/script');
  });

  test('应该拒绝敏感词', () => {
    const result = validatePlayerName('admin');
    expect(result.valid).toBe(false);
    expect(result.error).toBe('昵称包含不允许的词汇');
  });

  test('应该接受有效的昵称', () => {
    const result = validatePlayerName('玩家1');
    expect(result.valid).toBe(true);
    expect(result.value).toBe('玩家1');
  });
});

// 测试聊天消息验证
describe('validateChatMessage', () => {
  test('应该拒绝空消息', () => {
    const result = validateChatMessage('');
    expect(result.valid).toBe(false);
    expect(result.error).toBe('消息不能为空');
  });

  test('应该拒绝过长的消息', () => {
    const result = validateChatMessage('a'.repeat(101));
    expect(result.valid).toBe(false);
    expect(result.error).toBe('消息长度必须在1-100之间');
  });

  test('应该过滤HTML标签', () => {
    const result = validateChatMessage('<b>消息</b>');
    expect(result.valid).toBe(true);
    expect(result.value).toBe('b消息/b');
  });

  test('应该接受有效的消息', () => {
    const result = validateChatMessage('大家好！');
    expect(result.valid).toBe(true);
    expect(result.value).toBe('大家好！');
  });
});

// 测试成员ID验证
describe('validateMemberIds', () => {
  test('应该拒绝非数组', () => {
    const result = validateMemberIds('not array');
    expect(result.valid).toBe(false);
    expect(result.error).toBe('成员ID必须是数组');
  });

  test('应该拒绝空数组', () => {
    const result = validateMemberIds([]);
    expect(result.valid).toBe(false);
    expect(result.error).toBe('成员数量必须在1-6之间');
  });

  test('应该拒绝过长的数组', () => {
    const result = validateMemberIds(['a', 'b', 'c', 'd', 'e', 'f', 'g']);
    expect(result.valid).toBe(false);
    expect(result.error).toBe('成员数量必须在1-6之间');
  });

  test('应该拒绝无效的ID', () => {
    const result = validateMemberIds(['valid', '']);
    expect(result.valid).toBe(false);
    expect(result.error).toBe('成员ID格式无效');
  });

  test('应该接受有效的成员ID', () => {
    const result = validateMemberIds(['player1', 'player2']);
    expect(result.valid).toBe(true);
    expect(result.value).toEqual(['player1', 'player2']);
  });
});

// 测试投票验证
describe('validateVote', () => {
  test('应该拒绝非布尔值', () => {
    const result = validateVote('true');
    expect(result.valid).toBe(false);
    expect(result.error).toBe('投票必须是布尔值');
  });

  test('应该接受true', () => {
    const result = validateVote(true);
    expect(result.valid).toBe(true);
    expect(result.value).toBe(true);
  });

  test('应该接受false', () => {
    const result = validateVote(false);
    expect(result.valid).toBe(true);
    expect(result.value).toBe(false);
  });
});

console.log('所有测试通过！');
