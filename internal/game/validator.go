package game

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var roomIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func ValidateRoomID(input string) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", errors.New("房间ID不能为空")
	}
	if len(value) > 20 {
		return "", errors.New("房间ID长度必须在1-20之间")
	}
	if !roomIDPattern.MatchString(value) {
		return "", errors.New("房间ID只能包含字母、数字、下划线和连字符")
	}
	return value, nil
}

func ValidatePlayerName(input string) (string, error) {
	value := sanitizeText(strings.TrimSpace(input))
	if value == "" {
		return "", errors.New("昵称不能为空")
	}
	if utf8.RuneCountInString(value) > 10 {
		return "", errors.New("昵称长度必须在1-10之间")
	}
	if strings.EqualFold(value, "admin") {
		return "", errors.New("昵称包含不允许的词汇")
	}
	return value, nil
}

func ValidateChatMessage(input string) (string, error) {
	value := sanitizeText(strings.TrimSpace(input))
	if value == "" {
		return "", errors.New("消息不能为空")
	}
	if utf8.RuneCountInString(value) > 100 {
		return "", errors.New("消息长度必须在1-100之间")
	}
	return value, nil
}

func sanitizeText(input string) string {
	replacer := strings.NewReplacer("<", "", ">", "")
	return replacer.Replace(input)
}
