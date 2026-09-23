// Package poker implements server-authoritative play-money Texas Hold'em.
package poker

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sort"
)

type Card struct {
	Rank int `json:"rank"`
	Suit int `json:"suit"`
}
type Player struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Bot     bool   `json:"bot"`
	Chips   int    `json:"chips"`
	Cards   []Card `json:"-"`
	Bet     int    `json:"bet"`
	Total   int    `json:"total"`
	Folded  bool   `json:"folded"`
	Action  string `json:"action"`
	lastBet int
	recent  []string
}
type Options struct {
	Call     int  `json:"call"`
	Min      int  `json:"min"`
	Max      int  `json:"max"`
	CanRaise bool `json:"canRaise"`
}
type Action struct {
	Seat     int    `json:"seat"`
	Kind     string `json:"kind"`
	Text     string `json:"text"`
	Paid     int    `json:"paid"`
	Chips    int    `json:"chips"`
	Pot      int    `json:"pot"`
	Sequence uint64 `json:"sequence"`
	TimedOut bool   `json:"timedOut"`
}
type Table struct {
	Players                             []Player
	Dealer, Small, Big, Actor, Hand     int
	Phase                               string
	Board                               []Card
	Pot, LastPot, CurrentBet, LastRaise int
	Payouts                             []int
	Revealed                            bool
	Result                              string
	Log                                 []string
	LastAction                          *Action
	deck                                []Card
	pending                             map[int]bool
	random                              func(int) int
}

var handNames = []string{"高牌", "一对", "两对", "三条", "顺子", "同花", "葫芦", "四条", "同花顺"}

func randomInt(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic("crypto random unavailable")
	}
	return int(v.Int64())
}
func newTable(players []Player) *Table {
	return &Table{Players: players, Dealer: -1, Actor: -1, Phase: "idle", random: randomInt}
}
func (t *Table) next(from int, allowed func(int) bool) int {
	for n := 1; n <= len(t.Players); n++ {
		i := (from + n) % len(t.Players)
		if allowed(i) {
			return i
		}
	}
	return -1
}
func (t *Table) record(text string) {
	t.Log = append([]string{text}, t.Log...)
	if len(t.Log) > 35 {
		t.Log = t.Log[:35]
	}
}
func (t *Table) draw() Card { c := t.deck[len(t.deck)-1]; t.deck = t.deck[:len(t.deck)-1]; return c }
func (t *Table) pay(i, amount int) {
	p := &t.Players[i]
	paid := min(p.Chips, amount)
	p.Chips -= paid
	p.Bet += paid
	p.Total += paid
	t.Pot += paid
}
func (t *Table) start() error {
	if t.Phase != "idle" && t.Phase != "done" {
		return errors.New("本局尚未结束")
	}
	alive := 0
	for _, p := range t.Players {
		if p.Chips > 0 {
			alive++
		}
	}
	if alive < 2 {
		return errors.New("剩余玩家不足，请回到等待房重新开桌")
	}
	t.Hand++
	t.Board = []Card{}
	t.Log = nil
	t.Pot = 0
	t.Payouts = nil
	t.Revealed = false
	t.LastAction = nil
	t.Result = ""
	t.deck = make([]Card, 52)
	for i := range t.deck {
		t.deck[i] = Card{i%13 + 2, i / 13}
	}
	for i := 51; i > 0; i-- {
		j := t.random(i + 1)
		t.deck[i], t.deck[j] = t.deck[j], t.deck[i]
	}
	for i := range t.Players {
		p := &t.Players[i]
		p.Cards = nil
		p.Bet = 0
		p.Total = 0
		p.Folded = p.Chips == 0
		p.lastBet = -1
		p.Action = "等待行动"
		if p.Folded {
			p.Action = "休息中"
		}
	}
	t.Dealer = t.next(t.Dealer, func(i int) bool { return !t.Players[i].Folded })
	for round := 0; round < 2; round++ {
		for i := range t.Players {
			if !t.Players[i].Folded {
				t.Players[i].Cards = append(t.Players[i].Cards, t.draw())
			}
		}
	}
	t.Small = t.Dealer
	if alive > 2 {
		t.Small = t.next(t.Dealer, func(i int) bool { return !t.Players[i].Folded })
	}
	t.Big = t.next(t.Small, func(i int) bool { return !t.Players[i].Folded })
	t.pay(t.Small, 10)
	t.pay(t.Big, 20)
	t.Players[t.Small].Action = fmt.Sprintf("小盲 %d", t.Players[t.Small].Bet)
	t.Players[t.Big].Action = fmt.Sprintf("大盲 %d", t.Players[t.Big].Bet)
	t.CurrentBet = 20
	t.LastRaise = 20
	t.Phase = "preflop"
	t.pending = map[int]bool{}
	for i, p := range t.Players {
		if !p.Folded && p.Chips > 0 {
			t.pending[i] = true
		}
	}
	t.record(fmt.Sprintf("第 %d 局开始 · 小盲 10 / 大盲 20", t.Hand))
	t.advance(t.Big)
	return nil
}
func (t *Table) options() *Options {
	if t.Actor < 0 || t.Phase == "done" {
		return nil
	}
	p := t.Players[t.Actor]
	opponent := false
	for i, q := range t.Players {
		if i != t.Actor && !q.Folded && q.Chips > 0 {
			opponent = true
		}
	}
	maxBet := p.Bet + p.Chips
	return &Options{Call: min(max(0, t.CurrentBet-p.Bet), p.Chips), Min: t.CurrentBet + t.LastRaise, Max: maxBet, CanRaise: opponent && maxBet > t.CurrentBet && (p.lastBet < 0 || t.CurrentBet-p.lastBet >= t.LastRaise)}
}
func (t *Table) act(kind string, amount int) error {
	o := t.options()
	if o == nil {
		return errors.New("当前不能操作")
	}
	i := t.Actor
	p := &t.Players[i]
	before := p.Chips
	switch kind {
	case "raise":
		if !o.CanRaise || amount > o.Max || amount <= t.CurrentBet || (amount < o.Min && amount != o.Max) {
			return errors.New("加注金额无效")
		}
		increase := amount - t.CurrentBet
		t.pay(i, amount-p.Bet)
		if increase >= t.LastRaise {
			t.LastRaise = increase
		}
		t.CurrentBet = amount
		for j, q := range t.Players {
			if j != i && !q.Folded && q.Chips > 0 && q.Bet < amount {
				t.pending[j] = true
			}
		}
		p.Action = fmt.Sprintf("加注至 %d", amount)
		if p.Chips == 0 {
			p.Action = fmt.Sprintf("全下 %d", amount)
		}
	case "call":
		t.pay(i, o.Call)
		p.Action = "过牌"
		if o.Call > 0 {
			p.Action = fmt.Sprintf("跟注 %d", o.Call)
			if p.Chips == 0 {
				p.Action = fmt.Sprintf("全下 %d", o.Call)
			}
		}
	case "fold":
		p.Folded = true
		p.Action = "弃牌"
	default:
		return errors.New("未知操作")
	}
	p.recent = append(p.recent, kind)
	if len(p.recent) > 24 {
		p.recent = p.recent[1:]
	}
	t.LastAction = &Action{Seat: i, Kind: kind, Text: p.Action, Paid: before - p.Chips, Chips: p.Chips, Pot: t.Pot}
	p.lastBet = t.CurrentBet
	t.record(p.Name + " · " + p.Action)
	delete(t.pending, i)
	t.advance(i)
	return nil
}
func (t *Table) advance(from int) {
	alive := 0
	canAct := []int{}
	for i, p := range t.Players {
		if !p.Folded {
			alive++
			if p.Chips > 0 {
				canAct = append(canAct, i)
			}
		}
	}
	if alive == 1 {
		t.finish(false)
		return
	}
	if len(canAct) == 0 || (len(canAct) == 1 && t.Players[canAct[0]].Bet >= t.CurrentBet) {
		t.pending = map[int]bool{}
	}
	for i := range t.pending {
		if t.Players[i].Folded || t.Players[i].Chips == 0 {
			delete(t.pending, i)
		}
	}
	if len(t.pending) > 0 {
		t.Actor = t.next(from, func(i int) bool { return t.pending[i] })
		return
	}
	if t.Phase == "river" {
		t.finish(true)
		return
	}
	t.draw()
	count := 1
	if t.Phase == "preflop" {
		count = 3
	}
	for n := 0; n < count; n++ {
		t.Board = append(t.Board, t.draw())
	}
	t.Phase = map[string]string{"preflop": "flop", "flop": "turn", "turn": "river"}[t.Phase]
	t.CurrentBet = 0
	t.LastRaise = 20
	for i := range t.Players {
		p := &t.Players[i]
		p.Bet = 0
		p.lastBet = -1
		if !p.Folded && p.Chips > 0 {
			p.Action = "等待行动"
		}
	}
	t.record(map[string]string{"flop": "翻牌 · 三张公共牌登场", "turn": "转牌 · 再看一张", "river": "河牌 · 最后一张牌"}[t.Phase])
	t.pending = map[int]bool{}
	for _, i := range canAct {
		t.pending[i] = true
	}
	t.advance(t.Dealer)
}
func (t *Table) finish(showdown bool) {
	t.Revealed = showdown
	t.Payouts = make([]int, len(t.Players))
	scores := make([][]int, len(t.Players))
	levels := []int{}
	for i, p := range t.Players {
		if p.Total > 0 {
			levels = append(levels, p.Total)
		}
		if !p.Folded && showdown {
			scores[i] = evaluate(append(append([]Card{}, p.Cards...), t.Board...))
		}
	}
	sort.Ints(levels)
	previous := 0
	for _, level := range levels {
		if level == previous {
			continue
		}
		contributors, eligible := []int{}, []int{}
		for i, p := range t.Players {
			if p.Total >= level {
				contributors = append(contributors, i)
				if !p.Folded {
					eligible = append(eligible, i)
				}
			}
		}
		amount := (level - previous) * len(contributors)
		previous = level
		if len(eligible) == 0 {
			for _, i := range contributors {
				t.Payouts[i] += amount / len(contributors)
			}
			continue
		}
		winners := []int{eligible[0]}
		for _, i := range eligible[1:] {
			diff := 0
			if showdown {
				diff = compare(scores[i], scores[winners[0]])
			}
			if diff > 0 {
				winners = []int{i}
			} else if diff == 0 {
				winners = append(winners, i)
			}
		}
		size := len(t.Players)
		sort.Slice(winners, func(a, b int) bool { return (winners[a]-t.Dealer-1+size)%size < (winners[b]-t.Dealer-1+size)%size })
		for n, i := range winners {
			t.Payouts[i] += amount / len(winners)
			if n < amount%len(winners) {
				t.Payouts[i]++
			}
		}
	}
	t.LastPot = t.Pot
	t.Result = ""
	for i := range t.Players {
		p := &t.Players[i]
		p.Chips += t.Payouts[i]
		if !p.Folded {
			p.Action = "大家都弃牌啦"
			if showdown {
				p.Action = handNames[scores[i][0]]
			}
		}
		if t.Payouts[i] > 0 {
			if t.Result != "" {
				t.Result += " · "
			}
			t.Result += fmt.Sprintf("%s收获 %d", p.Name, t.Payouts[i])
		}
	}
	t.record(t.Result)
	t.Pot = 0
	t.Actor = -1
	t.Phase = "done"
}
func compare(a, b []int) int {
	for i := 0; i < max(len(a), len(b)); i++ {
		x, y := 0, 0
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x - y
		}
	}
	return 0
}
func evaluate(cards []Card) []int {
	var best []int
	for a := 0; a < len(cards)-4; a++ {
		for b := a + 1; b < len(cards)-3; b++ {
			for c := b + 1; c < len(cards)-2; c++ {
				for d := c + 1; d < len(cards)-1; d++ {
					for e := d + 1; e < len(cards); e++ {
						score := five([]Card{cards[a], cards[b], cards[c], cards[d], cards[e]})
						if best == nil || compare(score, best) > 0 {
							best = score
						}
					}
				}
			}
		}
	}
	return best
}
func five(cards []Card) []int {
	ranks := []int{}
	counts := map[int]int{}
	flush := true
	for _, c := range cards {
		ranks = append(ranks, c.Rank)
		counts[c.Rank]++
		flush = flush && c.Suit == cards[0].Suit
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ranks)))
	groups := []int{}
	for r := range counts {
		groups = append(groups, r)
	}
	sort.Slice(groups, func(i, j int) bool {
		if counts[groups[i]] != counts[groups[j]] {
			return counts[groups[i]] > counts[groups[j]]
		}
		return groups[i] > groups[j]
	})
	straight := 0
	if len(counts) == 5 {
		if ranks[0]-ranks[4] == 4 {
			straight = ranks[0]
		} else if ranks[0] == 14 && ranks[1] == 5 && ranks[4] == 2 {
			straight = 5
		}
	}
	if flush && straight > 0 {
		return []int{8, straight}
	}
	if counts[groups[0]] == 4 {
		return append([]int{7}, groups...)
	}
	if counts[groups[0]] == 3 && counts[groups[1]] == 2 {
		return append([]int{6}, groups...)
	}
	if flush {
		return append([]int{5}, ranks...)
	}
	if straight > 0 {
		return []int{4, straight}
	}
	if counts[groups[0]] == 3 {
		return append([]int{3}, groups...)
	}
	if counts[groups[0]] == 2 && counts[groups[1]] == 2 {
		return append([]int{2}, groups...)
	}
	if counts[groups[0]] == 2 {
		return append([]int{1}, groups...)
	}
	return append([]int{0}, ranks...)
}
