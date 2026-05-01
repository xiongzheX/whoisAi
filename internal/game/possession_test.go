package game

import "testing"

func TestRewriteMessageByStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		style Style
		want  string
	}{
		{style: StylePolite, want: "可能我觉得不行"},
		{style: StyleVerbose, want: "我觉得不行，因为这和前面的投票历史有关"},
		{style: StyleNeutral, want: "我觉得不太稳"},
		{style: StyleAwkward, want: "从系统角度看，我觉得不行"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.style), func(t *testing.T) {
			t.Parallel()

			got := RewriteMessage("我觉得不行", tt.style)
			if got != tt.want {
				t.Fatalf("RewriteMessage style %q = %q, want %q", tt.style, got, tt.want)
			}
		})
	}
}
