package game

import "strings"

type Style string

const (
	StylePolite  Style = "polite"
	StyleVerbose Style = "verbose"
	StyleNeutral Style = "neutral"
	StyleAwkward Style = "awkward"
)

func RewriteMessage(message string, style Style) string {
	text := strings.TrimSpace(message)
	switch style {
	case StylePolite:
		return "可能" + text
	case StyleVerbose:
		return text + "，因为这和前面的投票历史有关"
	case StyleNeutral:
		return strings.ReplaceAll(text, "不行", "不太稳")
	case StyleAwkward:
		return "从系统角度看，" + text
	default:
		return text
	}
}
