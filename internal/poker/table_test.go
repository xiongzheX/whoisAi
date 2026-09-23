package poker

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func makeTable(count int, seed int64) *Table {
	players := make([]Player, count)
	for i := range players {
		players[i] = Player{ID: fmt.Sprint(i), Name: fmt.Sprint(i), Chips: 1000, Bot: i > 0}
	}
	table := newTable(players)
	table.random = rand.New(rand.NewSource(seed)).Intn
	return table
}
func parseCards(text string) []Card {
	out := []Card{}
	for i := 0; i < len(text); i += 3 {
		r, s := 0, 0
		for j, c := range "23456789TJQKA" {
			if byte(c) == text[i] {
				r = j + 2
			}
		}
		for j, c := range "shcd" {
			if byte(c) == text[i+1] {
				s = j
			}
		}
		out = append(out, Card{r, s})
	}
	return out
}
func TestHandEvaluation(t *testing.T) {
	hands := []string{"As Jh 9c 7d 2s", "As Ah 9c 7d 2s", "As Ah 9c 9d 2s", "As Ah Ac 7d 2s", "As 2h 3c 4d 5s", "As Js 9s 7s 2s", "As Ah Ac 7d 7s", "As Ah Ac Ad 2s", "As Ks Qs Js Ts"}
	for rank, hand := range hands {
		if got := evaluate(parseCards(hand)); got[0] != rank {
			t.Fatalf("%s: %v", hand, got)
		}
	}
	if got := evaluate(parseCards("As Ah Ac Ks Kh Kc 2s")); !reflect.DeepEqual(got, []int{6, 14, 13}) {
		t.Fatal(got)
	}
}
func TestAllSeatCountsAndConservation(t *testing.T) {
	for count := 2; count <= 9; count++ {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			for run := 0; run < 20; run++ {
				table := makeTable(count, int64(run))
				for hand := 0; hand < 10; hand++ {
					if err := table.start(); err != nil {
						break
					}
					steps := 0
					for table.Phase != "done" {
						steps++
						if steps > 1500 {
							t.Fatal("stuck")
						}
						kind, amount := table.botAction()
						if err := table.act(kind, amount); err != nil {
							t.Fatal(err)
						}
						total := table.Pot
						for _, p := range table.Players {
							if p.Chips < 0 {
								t.Fatal("negative chips")
							}
							total += p.Chips
						}
						if total != count*1000 {
							t.Fatalf("chips=%d", total)
						}
					}
					seen := map[Card]bool{}
					cards := append([]Card{}, table.Board...)
					for _, p := range table.Players {
						cards = append(cards, p.Cards...)
					}
					for _, c := range cards {
						if seen[c] {
							t.Fatal("duplicate card")
						}
						seen[c] = true
					}
				}
			}
		})
	}
}
func TestNineSidePots(t *testing.T) {
	table := makeTable(9, 1)
	table.Board = parseCards("Ts Jh Qd Ks 2h")
	holes := []string{"As Ah", "9c 9h", "Kc Kh", "Qc Qh", "Jc Jd", "Tc Th", "8c 8h", "7c 7h", "6c 6h"}
	for i := range table.Players {
		p := &table.Players[i]
		p.Chips = 0
		p.Total = (i + 1) * 100
		p.Cards = parseCards(holes[i])
	}
	table.Pot = 4500
	table.finish(true)
	want := []int{900, 800, 700, 600, 500, 400, 300, 200, 100}
	if !reflect.DeepEqual(table.Payouts, want) {
		t.Fatal(table.Payouts)
	}
}
func TestShortAllInAndHeadsUp(t *testing.T) {
	table := makeTable(9, 1)
	if err := table.start(); err != nil {
		t.Fatal(err)
	}
	_ = table.act("raise", 100)
	table.Players[4].Chips = 130
	_ = table.act("raise", 130)
	table.Players[5].Chips = 180
	_ = table.act("raise", 180)
	for table.Actor != 3 {
		_ = table.act("call", 0)
	}
	if !table.options().CanRaise || table.options().Min != 260 {
		t.Fatal(table.options())
	}
	if err := table.act("raise", 259); err == nil {
		t.Fatal("invalid raise accepted")
	}
	table = makeTable(9, 1)
	for i := 1; i < 8; i++ {
		table.Players[i].Chips = 0
	}
	_ = table.start()
	if table.Small != 0 || table.Big != 8 || table.Actor != 0 {
		t.Fatal("heads-up blinds")
	}
	_ = table.act("call", 0)
	_ = table.act("call", 0)
	if table.Actor != 8 {
		t.Fatal("heads-up postflop")
	}
}
