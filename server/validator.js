/**
 * 输入验证工具
 */

// 房间ID验证
function validateRoomId(roomId) {
  if (!roomId || typeof roomId !== 'string') {
    return { valid: false, error: '房间ID不能为空' };
  }
  
  // 移除首尾空格
  roomId = roomId.trim();
  
  // 长度限制
  if (roomId.length < 1 || roomId.length > 20) {
    return { valid: false, error: '房间ID长度必须在1-20之间' };
  }
  
  // 只允许字母、数字、下划线、连字符
  if (!/^[a-zA-Z0-9_-]+$/.test(roomId)) {
    return { valid: false, error: '房间ID只能包含字母、数字、下划线和连字符' };
  }
  
  return { valid: true, value: roomId };
}

// 玩家昵称验证
function validatePlayerName(name) {
  if (!name || typeof name !== 'string') {
    return { valid: false, error: '昵称不能为空' };
  }
  
  // 移除首尾空格
  name = name.trim();
  
  // 长度限制
  if (name.length < 1 || name.length > 10) {
    return { valid: false, error: '昵称长度必须在1-10之间' };
  }
  
  // 过滤HTML标签和特殊字符（增强版）
  // 移除HTML标签
  name = name.replace(/<[^>]*>/g, '');
  // 移除HTML实体
  name = name.replace(/&[a-zA-Z0-9#]+;/g, '');
  // 移除JavaScript代码相关字符
  name = name.replace(/[<>\"'&]/g, '');
  // 移除控制字符
  name = name.replace(/[\x00-\x1F\x7F-\x9F]/g, '');
  
  // 检查是否包含敏感词（简单示例）
  const sensitiveWords = ['admin', 'system', 'bot', 'ai'];
  const lowerName = name.toLowerCase();
  for (const word of sensitiveWords) {
    if (lowerName.includes(word)) {
      return { valid: false, error: '昵称包含不允许的词汇' };
    }
  }
  
  return { valid: true, value: name };
}

// 聊天消息验证
function validateChatMessage(message) {
  if (!message || typeof message !== 'string') {
    return { valid: false, error: '消息不能为空' };
  }
  
  // 移除首尾空格
  message = message.trim();
  
  // 长度限制
  if (message.length < 1 || message.length > 100) {
    return { valid: false, error: '消息长度必须在1-100之间' };
  }
  
  // 过滤HTML标签和特殊字符（增强版）
  // 移除HTML标签
  message = message.replace(/<[^>]*>/g, '');
  // 移除HTML实体
  message = message.replace(/&[a-zA-Z0-9#]+;/g, '');
  // 移除JavaScript代码相关字符
  message = message.replace(/[<>\"'&]/g, '');
  // 移除控制字符
  message = message.replace(/[\x00-\x1F\x7F-\x9F]/g, '');
  
  return { valid: true, value: message };
}

// 数组验证（用于小队提名）
function validateMemberIds(memberIds) {
  if (!Array.isArray(memberIds)) {
    return { valid: false, error: '成员ID必须是数组' };
  }
  
  // 长度限制
  if (memberIds.length < 1 || memberIds.length > 6) {
    return { valid: false, error: '成员数量必须在1-6之间' };
  }
  
  // 验证每个ID
  for (const id of memberIds) {
    if (typeof id !== 'string' || id.length < 1 || id.length > 50) {
      return { valid: false, error: '成员ID格式无效' };
    }
  }
  
  return { valid: true, value: memberIds };
}

// 投票验证
function validateVote(vote) {
  if (typeof vote !== 'boolean') {
    return { valid: false, error: '投票必须是布尔值' };
  }
  
  return { valid: true, value: vote };
}

module.exports = {
  validateRoomId,
  validatePlayerName,
  validateChatMessage,
  validateMemberIds,
  validateVote
};
