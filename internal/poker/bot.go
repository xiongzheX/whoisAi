package poker

import "math"

type botStyle struct{ loose, aggression, bluff, sizing, pace float64 }

var botStyles = []botStyle{
	{-.07, .28, .04, .6, 1.2}, {.10, .32, .06, .45, .9}, {-.09, .62, .05, .85, 1.25}, {-.04, .23, .03, .55, 1.5},
	{0, .48, .10, .65, 1}, {.06, .78, .19, .9, .8}, {.13, .18, .03, .45, 1.1}, {.02, .59, .16, .55, 1.05}, {.07, .51, .12, .75, .85},
}

func clamp(v, low, high float64) float64 { return math.Max(low, math.Min(high, v)) }
func (t *Table) roll() float64           { return float64(t.random(1000000)) / 1000000 }

// handEstimate reads only the acting player's own cards and the public board.
func handEstimate(cards, board []Card, opponents int) (equity, draw, danger float64) {
	high, low := max(cards[0].Rank, cards[1].Rank), min(cards[0].Rank, cards[1].Rank)
	if len(board) == 0 {
		strength := .17 + float64(high)*.023 + float64(low)*.013
		if high == low {
			strength = .51 + float64(high)*.023
		} else {
			if cards[0].Suit == cards[1].Suit {
				strength += .045
			}
			if high-low <= 2 {
				strength += .035
			}
		}
		return math.Pow(clamp(strength, .18, .86), 1+float64(opponents-1)*.28), 0, 0
	}
	all := append(append([]Card{}, cards...), board...)
	score := evaluate(all)
	suits, boardSuits := [4]int{}, [4]int{}
	ranks := map[int]bool{}
	boardRanks := map[int]bool{}
	for _, c := range all {
		suits[c.Suit]++
		ranks[c.Rank] = true
	}
	if ranks[14] {
		ranks[1] = true
	}
	for _, c := range board {
		boardSuits[c.Suit]++
		boardRanks[c.Rank] = true
	}
	if len(board) < 5 {
		for suit, count := range suits {
			if count == 4 && (cards[0].Suit == suit || cards[1].Suit == suit) {
				draw += .13
				break
			}
		}
		for start := 1; start <= 10; start++ {
			count := 0
			for r := start; r < start+5; r++ {
				if ranks[r] {
					count++
				}
			}
			if count == 4 && score[0] < 4 {
				draw += .08
				break
			}
		}
	}
	for _, count := range boardSuits {
		if count >= 3 {
			danger += .10
			break
		}
	}
	for start := 2; start <= 10; start++ {
		count := 0
		for r := start; r < start+5; r++ {
			if boardRanks[r] {
				count++
			}
		}
		if count >= 3 {
			danger += .07
			break
		}
	}
	equity = []float64{.15, .46, .64, .76, .84, .89, .94, .98, .995}[score[0]]
	if score[0] == 0 {
		equity += float64(high-8) * .016
	}
	if score[0] == 1 {
		personal := cards[0].Rank == score[1] || cards[1].Rank == score[1]
		top := 0
		for _, c := range board {
			top = max(top, c.Rank)
		}
		if !personal {
			equity -= .21
		} else if score[1] >= top {
			equity += .17
		} else {
			equity += .02
		}
	}
	if len(board) == 5 && compare(score, evaluate(board)) == 0 {
		equity = .45 / float64(opponents+1)
	} else {
		if score[0] < 5 {
			equity -= danger
		}
		equity = math.Pow(clamp(equity, .05, .995), 1+float64(opponents-1)*.22)
	}
	return clamp(equity+draw, .03, .995), draw, danger
}
func (t *Table) botAction() (string, int) {
	p := t.Players[t.Actor]
	o := t.options()
	style := botStyles[t.Actor%len(botStyles)]
	opponents, observed, folds, raises := 0, 0, 0, 0
	order := []int{}
	for n := 1; n <= len(t.Players); n++ {
		i := (t.Dealer + n) % len(t.Players)
		q := t.Players[i]
		if !q.Folded {
			order = append(order, i)
			if i != t.Actor {
				opponents++
				for _, a := range q.recent {
					observed++
					if a == "fold" {
						folds++
					}
					if a == "raise" {
						raises++
					}
				}
			}
		}
	}
	equity, draw, danger := handEstimate(p.Cards, t.Board, opponents)
	position := -.025
	for _, i := range order[max(0, len(order)-2):] {
		if i == t.Actor {
			position = .035
		}
	}
	read, foldRead := 0.0, 0.0
	if observed >= 12 {
		read = clamp((float64(raises)/float64(observed)-.3)*.15, -.035, .06)
		foldRead = clamp((float64(folds)/float64(observed)-.2)*.16, -.025, .06)
	}
	odds := float64(o.Call) / float64(max(1, t.Pot+o.Call))
	pressure := float64(o.Call) / float64(max(1, p.Chips))
	edge := equity + style.loose + position + read - odds - pressure*.055
	if o.Call > 0 && t.roll() < clamp(.42-edge*2.1, .015, .96) {
		return "fold", 0
	}
	value := equity > .56 && edge > .14
	steal := 0.0
	if o.Call == 0 && position > 0 {
		steal = .06
	}
	chance := math.Max(0, style.bluff+steal+draw*.5+foldRead) / math.Max(1, float64(opponents)*.9)
	if danger > 0 {
		chance *= .7
	}
	if value {
		chance = style.aggression * (.55 + equity*.45)
	}
	if o.CanRaise && t.roll() < chance {
		fraction := clamp(style.sizing+(t.roll()-.5)*.5, .3, 1.2)
		if value {
			fraction += danger
		}
		target := t.CurrentBet + int(math.Round(float64(t.Pot+o.Call)*fraction/10))*10
		if len(t.Board) == 0 {
			target = max(target, 40+t.random(3)*20)
		}
		if value && float64(p.Chips) <= float64(t.Pot+o.Call)*1.25 && t.roll() < .65 {
			target = o.Max
		}
		return "raise", min(o.Max, max(o.Min, target))
	}
	return "call", 0
}
