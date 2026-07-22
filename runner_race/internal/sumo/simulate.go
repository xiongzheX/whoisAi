package sumo

import (
	"fmt"
	"math"
)

func Simulate(a, b Wrestler, seed int64) (MatchResult, error) {
	if err := ValidateWrestler(a); err != nil {
		return MatchResult{}, fmt.Errorf("wrestler A: %w", err)
	}
	if err := ValidateWrestler(b); err != nil {
		return MatchResult{}, fmt.Errorf("wrestler B: %w", err)
	}

	rng := NewRNG(seed)
	environment := randomEnvironment(rng)
	archetypeA := Archetype(a)
	archetypeB := Archetype(b)
	stateA := initState(a, -2.2, 0)
	stateB := initState(b, 2.2, 0)
	result := MatchResult{
		MatchID:     fmt.Sprintf("sumo_%d", seed),
		Seed:        seed,
		Environment: environment,
		ArchetypeA:  archetypeA,
		ArchetypeB:  archetypeB,
		Frames:      make([]ReplayFrame, 0, MaxTicks),
		Events:      make([]ReplayEvent, 0, 16),
	}

	const finishTicks = 40
	regularTicks := MaxTicks - finishTicks
	for tick := 0; tick < regularTicks; tick++ {
		t := float64(tick) * DT
		stepPair(&stateA, &stateB, a, b, archetypeA, archetypeB, environment, tick, t, rng, &result)
		appendFrame(&result, tick, t, stateA, stateB)
		if outOfRing(stateA) || outOfRing(stateB) {
			if tick < regularTicks-8 && tryEarlySave(&stateA, &stateB, a, b, tick, t, rng, &result) {
				continue
			}
			if outOfRing(stateA) && outOfRing(stateB) {
				result.Winner = tieBreak(stateA, stateB, a, b, rng)
				result.Reason = "edge_tiebreak"
			} else if outOfRing(stateA) {
				result.Winner = b.ID
				result.Reason = "ring_out"
			} else {
				result.Winner = a.ID
				result.Reason = "ring_out"
			}
			return result, nil
		}
	}

	result.Winner = tieBreak(stateA, stateB, a, b, rng)
	result.Reason = "ring_out"
	appendFinish(&result, stateA, stateB, a, b, regularTicks, finishTicks, rng)
	return result, nil
}

func appendFrame(result *MatchResult, tick int, t float64, a, b wrestlerState) {
	result.Frames = append(result.Frames, ReplayFrame{
		Tick: tick,
		Time: round3(t),
		A:    snapshot(a),
		B:    snapshot(b),
	})
}

func appendFinish(result *MatchResult, startA, startB wrestlerState, wa, wb Wrestler, startTick, ticks int, rng *RNG) {
	roll := rng.Float64()
	switch {
	case roll < 0.30:
		appendPressureFinish(result, startA, startB, wa, wb, startTick, ticks)
	case roll < 0.60:
		appendEdgeTurnFinish(result, startA, startB, wa, wb, startTick, ticks)
	default:
		appendDirectFinish(result, startA, startB, wa, wb, startTick, ticks)
	}
}

func appendPressureFinish(result *MatchResult, startA, startB wrestlerState, wa, wb Wrestler, startTick, ticks int) {
	const pressureTicks = 14
	finishTicks := ticks - pressureTicks
	winnerA := result.Winner == wa.ID
	winnerStart := startA
	loserStart := startB
	if !winnerA {
		winnerStart = startB
		loserStart = startA
	}

	dx, dy := unit(winnerStart.X, winnerStart.Y)
	if math.Hypot(dx, dy) == 0 {
		dx, dy = unit(winnerStart.X-loserStart.X, winnerStart.Y-loserStart.Y)
	}
	winnerDangerX := dx * (RingRadius * 0.46)
	winnerDangerY := dy * (RingRadius * 0.46)
	loserPressX := winnerDangerX - dx*1.62
	loserPressY := winnerDangerY - dy*1.62

	loserID := wb.ID
	if !winnerA {
		loserID = wa.ID
	}
	addEvent(result, startTick, float64(startTick)*DT, loserID, "near_throw", "压住重心")
	for i := 1; i <= pressureTicks; i++ {
		p := easeInOut(float64(i) / float64(pressureTicks))
		winner := winnerStart
		loser := loserStart
		winner.X = lerp(winnerStart.X, winnerDangerX, p)
		winner.Y = lerp(winnerStart.Y, winnerDangerY, p)
		loser.X = lerp(loserStart.X, loserPressX, p)
		loser.Y = lerp(loserStart.Y, loserPressY, p)
		winner.StumbleTicks = pressureTicks - i + 1
		sepX, sepY := unit(loser.X-winner.X, loser.Y-winner.Y)
		keepBodiesApart(&winner, &loser, sepX, sepY, math.Hypot(loser.X-winner.X, loser.Y-winner.Y), 1.45)

		tick := startTick + i - 1
		if winnerA {
			appendFrame(result, tick, float64(tick)*DT, winner, loser)
		} else {
			appendFrame(result, tick, float64(tick)*DT, loser, winner)
		}
	}

	loserEndX := -dx * (RingRadius + 1.25)
	loserEndY := -dy * (RingRadius + 1.25)

	addEvent(result, startTick+pressureTicks, float64(startTick+pressureTicks)*DT, result.Winner, "finish_push", "回身推出")
	for i := 1; i <= finishTicks; i++ {
		p := easeOut(float64(i) / float64(finishTicks))
		winner := winnerStart
		loser := loserStart
		loser.X = lerp(loserPressX, loserEndX, p)
		loser.Y = lerp(loserPressY, loserEndY, p)
		contact := lerp(1.62, 1.46, p)
		winner.X, winner.Y = pusherPosition(loser.X, loser.Y, -dx, -dy, contact)
		loser.StumbleTicks = finishTicks - i + 1

		tick := startTick + pressureTicks + i - 1
		if winnerA {
			appendFrame(result, tick, float64(tick)*DT, winner, loser)
		} else {
			appendFrame(result, tick, float64(tick)*DT, loser, winner)
		}
	}
}

func appendEdgeTurnFinish(result *MatchResult, startA, startB wrestlerState, wa, wb Wrestler, startTick, ticks int) {
	const braceTicks = 12
	finishTicks := ticks - braceTicks
	winnerA := result.Winner == wa.ID
	winnerStart := startA
	loserStart := startB
	if !winnerA {
		winnerStart = startB
		loserStart = startA
	}

	dx, dy := unit(winnerStart.X, winnerStart.Y)
	if math.Hypot(dx, dy) == 0 {
		dx, dy = unit(winnerStart.X-loserStart.X, winnerStart.Y-loserStart.Y)
	}
	winnerBraceX := dx * (RingRadius * 0.58)
	winnerBraceY := dy * (RingRadius * 0.58)
	loserLeanX := winnerBraceX - dx*1.55
	loserLeanY := winnerBraceY - dy*1.55

	loserID := wb.ID
	if !winnerA {
		loserID = wa.ID
	}
	addEvent(result, startTick, float64(startTick)*DT, loserID, "near_throw", "逼到外侧")
	for i := 1; i <= braceTicks; i++ {
		p := easeInOut(float64(i) / float64(braceTicks))
		winner := winnerStart
		loser := loserStart
		winner.X = lerp(winnerStart.X, winnerBraceX, p)
		winner.Y = lerp(winnerStart.Y, winnerBraceY, p)
		loser.X = lerp(loserStart.X, loserLeanX, p)
		loser.Y = lerp(loserStart.Y, loserLeanY, p)
		winner.StumbleTicks = braceTicks - i + 1
		sepX, sepY := unit(loser.X-winner.X, loser.Y-winner.Y)
		keepBodiesApart(&winner, &loser, sepX, sepY, math.Hypot(loser.X-winner.X, loser.Y-winner.Y), 1.45)

		tick := startTick + i - 1
		if winnerA {
			appendFrame(result, tick, float64(tick)*DT, winner, loser)
		} else {
			appendFrame(result, tick, float64(tick)*DT, loser, winner)
		}
	}

	loserEndX := -dx * (RingRadius + 1.05)
	loserEndY := -dy * (RingRadius + 1.05)

	addEvent(result, startTick+braceTicks, float64(startTick+braceTicks)*DT, result.Winner, "edge_turn", "回身借力")
	for i := 1; i <= finishTicks; i++ {
		p := easeOut(float64(i) / float64(finishTicks))
		winner := winnerStart
		loser := loserStart
		loser.X = lerp(loserLeanX, loserEndX, p)
		loser.Y = lerp(loserLeanY, loserEndY, p)
		contact := lerp(1.55, 1.46, p)
		winner.X, winner.Y = pusherPosition(loser.X, loser.Y, -dx, -dy, contact)
		loser.StumbleTicks = finishTicks - i + 1

		tick := startTick + braceTicks + i - 1
		if winnerA {
			appendFrame(result, tick, float64(tick)*DT, winner, loser)
		} else {
			appendFrame(result, tick, float64(tick)*DT, loser, winner)
		}
	}
}

func appendDirectFinish(result *MatchResult, startA, startB wrestlerState, wa, wb Wrestler, startTick, ticks int) {
	winnerA := result.Winner == wa.ID
	winnerStart := startA
	loserStart := startB
	if !winnerA {
		winnerStart = startB
		loserStart = startA
	}

	dx, dy := unit(loserStart.X-winnerStart.X, loserStart.Y-winnerStart.Y)
	if math.Hypot(dx, dy) == 0 {
		dx, dy = unit(loserStart.X, loserStart.Y)
	}
	loserEndX := dx * (RingRadius + 1.05)
	loserEndY := dy * (RingRadius + 1.05)

	addEvent(result, startTick, float64(startTick)*DT, result.Winner, "finish_push", "顺势推出")
	for i := 1; i <= ticks; i++ {
		p := easeOut(float64(i) / float64(ticks))
		winner := winnerStart
		loser := loserStart
		loser.X = lerp(loserStart.X, loserEndX, p)
		loser.Y = lerp(loserStart.Y, loserEndY, p)
		startGap := math.Hypot(loserStart.X-winnerStart.X, loserStart.Y-winnerStart.Y)
		if startGap < 1.46 {
			startGap = 1.46
		}
		contact := lerp(startGap, 1.46, p)
		winner.X, winner.Y = pusherPosition(loser.X, loser.Y, dx, dy, contact)
		loser.StumbleTicks = ticks - i + 1

		tick := startTick + i - 1
		if winnerA {
			appendFrame(result, tick, float64(tick)*DT, winner, loser)
		} else {
			appendFrame(result, tick, float64(tick)*DT, loser, winner)
		}
	}
}

func pusherPosition(targetX, targetY, pushX, pushY, contact float64) (float64, float64) {
	return targetX - pushX*contact, targetY - pushY*contact
}

func easeOut(v float64) float64 {
	return 1 - math.Pow(1-v, 3)
}

func easeInOut(v float64) float64 {
	return v * v * (3 - 2*v)
}

func lerp(a, b, p float64) float64 {
	return a + (b-a)*p
}

func initState(w Wrestler, x, y float64) wrestlerState {
	return wrestlerState{
		WrestlerID:    w.ID,
		X:             x,
		Y:             y,
		Energy:        66 + float64(w.Stats.Stamina)*5 + float64(w.Stats.Spirit)*1.2 + traitEnergy(w),
		LastCounterAt: -100,
		LastRallyAt:   -100,
	}
}

func stepPair(a, b *wrestlerState, wa, wb Wrestler, archetypeA, archetypeB, environment string, tick int, t float64, rng *RNG, result *MatchResult) {
	ax, ay := unit(b.X-a.X, b.Y-a.Y)
	bx, by := -ax, -ay
	distance := math.Hypot(b.X-a.X, b.Y-a.Y)

	applyArchetypeCounter(a, b, wa, wb, archetypeA, archetypeB, tick, t, rng, result)
	applyArchetypeCounter(b, a, wb, wa, archetypeB, archetypeA, tick, t, rng, result)

	forceA := pushForce(wa, *a, *b, archetypeA, archetypeB, environment, tick, rng, result)
	forceB := pushForce(wb, *b, *a, archetypeB, archetypeA, environment, tick, rng, result)
	resistA := resistForce(wa, *a, archetypeA, archetypeB, environment)
	resistB := resistForce(wb, *b, archetypeB, archetypeA, environment)

	const contactDistance = 1.45
	if distance > contactDistance {
		approachA := math.Max(1.2, forceA-resistB*0.36)
		approachB := math.Max(1.2, forceB-resistA*0.36)
		a.VX += ax * approachA * 0.54 * DT
		a.VY += ay * approachA * 0.54 * DT
		b.VX += bx * approachB * 0.54 * DT
		b.VY += by * approachB * 0.54 * DT
	} else {
		pressureA := forceA + resistA*0.30
		pressureB := forceB + resistB*0.30
		shove := pressureA - pressureB
		if math.Abs(shove) < 0.18 {
			shove += rng.Range(-0.22, 0.22)
		}
		a.VX += ax * shove * 0.42 * DT
		a.VY += ay * shove * 0.42 * DT
		b.VX += ax * shove * 0.70 * DT
		b.VY += ay * shove * 0.70 * DT
		keepBodiesApart(a, b, ax, ay, distance, contactDistance)
	}

	sideA := footworkDrift(wa, environment, rng)
	sideB := footworkDrift(wb, environment, rng)
	a.VX += -ay * sideA * DT
	a.VY += ax * sideA * DT
	b.VX += -by * sideB * DT
	b.VY += bx * sideB * DT

	applyStumble(a, wa, environment, tick, t, rng, result)
	applyStumble(b, wb, environment, tick, t, rng, result)

	dampA := 0.90 + float64(wa.Stats.Balance)*0.004
	dampB := 0.90 + float64(wb.Stats.Balance)*0.004
	a.VX *= dampA
	a.VY *= dampA
	b.VX *= dampB
	b.VY *= dampB

	a.X += a.VX * DT
	a.Y += a.VY * DT
	b.X += b.VX * DT
	b.Y += b.VY * DT
	ax, ay = unit(b.X-a.X, b.Y-a.Y)
	keepBodiesApart(a, b, ax, ay, math.Hypot(b.X-a.X, b.Y-a.Y), contactDistance)
	applyRally(a, b, wa, wb, tick, t, rng, result)
	applyRally(b, a, wb, wa, tick, t, rng, result)
	applyEdgeTurn(a, b, wa, wb, tick, t, rng, result)
	applyEdgeTurn(b, a, wb, wa, tick, t, rng, result)
	a.Energy = math.Max(0, a.Energy-energyCost(wa, environment))
	b.Energy = math.Max(0, b.Energy-energyCost(wb, environment))
	tickDownCounters(a)
	tickDownCounters(b)
}

func keepBodiesApart(a, b *wrestlerState, ax, ay, distance, minDistance float64) {
	if distance <= 0 || distance >= minDistance {
		return
	}
	correction := (minDistance - distance) * 0.5
	a.X -= ax * correction
	a.Y -= ay * correction
	b.X += ax * correction
	b.Y += ay * correction
}

func tryEarlySave(a, b *wrestlerState, wa, wb Wrestler, tick int, t float64, rng *RNG, result *MatchResult) bool {
	if outOfRing(*a) && maybeSaveFromRingOut(a, b, wa, wb, tick, t, rng, result) {
		return true
	}
	if outOfRing(*b) && maybeSaveFromRingOut(b, a, wb, wa, tick, t, rng, result) {
		return true
	}
	return false
}

func maybeSaveFromRingOut(self, opponent *wrestlerState, w, opponentW Wrestler, tick int, t float64, rng *RNG, result *MatchResult) bool {
	if tick-self.LastRallyAt < 18 {
		return false
	}
	chance := 0.24 +
		float64(w.Stats.Balance)*0.018 +
		float64(w.Stats.Footwork)*0.014 +
		float64(w.Stats.Spirit)*0.010 -
		float64(opponentW.Stats.Power)*0.010
	if w.Trait == TraitLowCenter || w.Trait == TraitSoftStep || w.Trait == TraitCalmBreath {
		chance += 0.10
	}
	if chance > 0.62 {
		chance = 0.62
	}
	if rng.Float64() >= chance {
		return false
	}

	inX, inY := unit(-self.X, -self.Y)
	self.LastRallyAt = tick
	self.RallyTicks = 12
	self.X = inX * -(RingRadius - 0.82)
	self.Y = inY * -(RingRadius - 0.82)
	self.VX = inX * 2.6
	self.VY = inY * 2.6
	opponent.StumbleTicks = maxInt(opponent.StumbleTicks, 3)
	addEvent(result, tick, t, w.ID, "edge_turn", "边缘回身")
	return true
}

func applyRally(self, opponent *wrestlerState, w, opponentW Wrestler, tick int, t float64, rng *RNG, result *MatchResult) {
	if tick < 24 || tick-self.LastRallyAt < 22 || self.RallyTicks > 0 {
		return
	}
	trailing := distanceFromCenter(*self) - distanceFromCenter(*opponent)
	if trailing < 0.35 || distanceFromCenter(*self) < RingRadius*0.25 {
		return
	}
	chance := 0.010 +
		float64(w.Stats.Stamina)*0.0022 +
		float64(w.Stats.Spirit)*0.0018 +
		float64(w.Stats.Footwork)*0.0013 -
		float64(opponentW.Stats.Balance)*0.0008
	if w.Trait == TraitCalmBreath || w.Trait == TraitCounterGrip {
		chance += 0.010
	}
	if w.Style == StyleDefensive {
		chance += 0.004
	}
	if chance > 0.060 {
		chance = 0.060
	}
	if rng.Float64() >= chance {
		return
	}

	self.LastRallyAt = tick
	self.RallyTicks = 10
	self.Energy = math.Min(130, self.Energy+2.2)
	inX, inY := unit(-self.X, -self.Y)
	outX, outY := unit(opponent.X-self.X, opponent.Y-self.Y)
	self.VX += (inX*1.9 + outX*0.7) * DT
	self.VY += (inY*1.9 + outY*0.7) * DT
	opponent.StumbleTicks = maxInt(opponent.StumbleTicks, 2)
	addEvent(result, tick, t, w.ID, "rally", "憋住反扑")
}

func applyEdgeTurn(self, opponent *wrestlerState, w, opponentW Wrestler, tick int, t float64, rng *RNG, result *MatchResult) {
	if tick < 34 || tick-self.LastRallyAt < 18 || distanceFromCenter(*self) < RingRadius*0.74 {
		return
	}
	chance := 0.018 +
		float64(w.Stats.Balance)*0.0022 +
		float64(w.Stats.Footwork)*0.0018 +
		float64(w.Stats.Spirit)*0.0012 -
		float64(opponentW.Stats.Power)*0.0012
	if w.Trait == TraitLowCenter || w.Trait == TraitSoftStep {
		chance += 0.012
	}
	if chance > 0.070 {
		chance = 0.070
	}
	if rng.Float64() >= chance {
		return
	}

	self.LastRallyAt = tick
	self.RallyTicks = 12
	inX, inY := unit(-self.X, -self.Y)
	self.X *= 0.94
	self.Y *= 0.94
	self.VX = self.VX*0.35 + inX*2.4
	self.VY = self.VY*0.35 + inY*2.4
	opponent.StumbleTicks = maxInt(opponent.StumbleTicks, 3)
	opponent.Energy = math.Max(0, opponent.Energy-1.6)
	addEvent(result, tick, t, w.ID, "edge_turn", "边缘回身")
}

func applyArchetypeCounter(self, opponent *wrestlerState, w, opponentW Wrestler, ownArchetype, opponentArchetype string, tick int, t float64, rng *RNG, result *MatchResult) {
	if !counters(ownArchetype, opponentArchetype) || tick-self.LastCounterAt < 18 {
		return
	}
	chance := 0.006
	switch ownArchetype {
	case ArchetypeAgile:
		chance += float64(w.Stats.Footwork-opponentW.Stats.Footwork)*0.003 + float64(opponentW.Stats.Power-w.Stats.Power)*0.0015
	case ArchetypePower:
		chance += float64(w.Stats.Power-opponentW.Stats.Balance) * 0.003
	case ArchetypeEndurance:
		chance += float64(w.Stats.Stamina-opponentW.Stats.Stamina) * 0.004
	case ArchetypeSpirit:
		chance += float64(w.Stats.Spirit-opponentW.Stats.Spirit) * 0.004
	case ArchetypeGuard:
		chance += float64(w.Stats.Balance-opponentW.Stats.Footwork)*0.003 + float64(w.Stats.Weight-opponentW.Stats.Weight)*0.002
	}
	if chance < 0.004 {
		chance = 0.004
	}
	if chance > 0.045 {
		chance = 0.045
	}
	if rng.Float64() >= chance {
		return
	}

	self.LastCounterAt = tick
	switch ownArchetype {
	case ArchetypeAgile:
		self.CounterTicks = 6
		opponent.StumbleTicks = maxInt(opponent.StumbleTicks, 3)
		addEvent(result, tick, t, w.ID, "counter_agile", "闪身借力")
	case ArchetypePower:
		self.BreakerTicks = 8
		addEvent(result, tick, t, w.ID, "counter_power", "强推破节奏")
	case ArchetypeEndurance:
		self.DrainTicks = 10
		opponent.Energy = math.Max(0, opponent.Energy-3.5)
		addEvent(result, tick, t, w.ID, "counter_endurance", "拖住消耗")
	case ArchetypeSpirit:
		self.BreakerTicks = 7
		opponent.LockedTicks = maxInt(opponent.LockedTicks, 4)
		addEvent(result, tick, t, w.ID, "counter_spirit", "气势破防")
	case ArchetypeGuard:
		self.LockedTicks = 9
		opponent.CounterTicks = 0
		addEvent(result, tick, t, w.ID, "counter_guard", "扎根卡位")
	}
}

func archetypePushBonus(w Wrestler, self wrestlerState, ownArchetype, opponentArchetype string) float64 {
	bonus := 0.0
	if self.CounterTicks > 0 {
		bonus += 0.75 + float64(w.Stats.Footwork)*0.045
	}
	if self.BreakerTicks > 0 {
		bonus += 0.75 + float64(w.Stats.Power+w.Stats.Spirit)*0.028
	}
	if self.DrainTicks > 0 {
		bonus += 0.65 + float64(w.Stats.Stamina)*0.05
	}
	if self.RallyTicks > 0 {
		bonus += 1.05 + float64(w.Stats.Spirit+w.Stats.Stamina)*0.045
	}
	if ownArchetype == ArchetypeEndurance {
		bonus += 0.20 + float64(w.Stats.Stamina)*0.025
	}
	if ownArchetype == ArchetypeBalanced {
		bonus += 0.24
	}
	if ownArchetype == ArchetypeAgile && opponentArchetype == ArchetypePower {
		bonus += float64(w.Stats.Footwork) * 0.012
	}
	return bonus
}

func archetypeResistBonus(w Wrestler, self wrestlerState, ownArchetype, opponentArchetype string) float64 {
	bonus := 0.0
	if self.LockedTicks > 0 {
		bonus += 1.1 + float64(w.Stats.Weight+w.Stats.Balance)*0.04
	}
	if self.DrainTicks > 0 {
		bonus += 0.9 + float64(w.Stats.Stamina)*0.05
	}
	if ownArchetype == ArchetypeEndurance {
		bonus += 0.45 + float64(w.Stats.Stamina)*0.035
	}
	if ownArchetype == ArchetypeBalanced {
		bonus += 0.30
	}
	if self.RallyTicks > 0 {
		bonus += 0.85 + float64(w.Stats.Balance+w.Stats.Footwork)*0.035
	}
	if ownArchetype == ArchetypeGuard && opponentArchetype == ArchetypeAgile {
		bonus += float64(w.Stats.Balance) * 0.035
	}
	return bonus
}

func tickDownCounters(s *wrestlerState) {
	if s.CounterTicks > 0 {
		s.CounterTicks--
	}
	if s.LockedTicks > 0 {
		s.LockedTicks--
	}
	if s.BreakerTicks > 0 {
		s.BreakerTicks--
	}
	if s.DrainTicks > 0 {
		s.DrainTicks--
	}
	if s.RallyTicks > 0 {
		s.RallyTicks--
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func pushForce(w Wrestler, self, opponent wrestlerState, ownArchetype, opponentArchetype, environment string, tick int, rng *RNG, result *MatchResult) float64 {
	force := 7.6 +
		float64(w.Stats.Power)*0.31 +
		float64(w.Stats.Spirit)*0.20 +
		float64(w.Stats.Footwork)*0.31 +
		float64(w.Stats.Stamina)*0.24
	switch w.Style {
	case StyleAggressive:
		force += 0.8
	case StyleDefensive:
		force -= 0.35
	case StyleTrickster:
		force += rng.Range(-0.8, 1.1)
	}
	force += environmentPushBonus(w, environment, tick)
	if self.StumbleTicks > 0 {
		force *= 0.50
	}
	if self.Energy < 20 {
		force -= (20 - self.Energy) * 0.08
	}
	force += archetypePushBonus(w, self, ownArchetype, opponentArchetype)
	switch w.Trait {
	case TraitBullRush:
		if tick < 30 {
			force += 1.3
		}
	case TraitBigRoar:
		if tick == 20 {
			addEvent(result, tick, float64(tick)*DT, w.ID, "roar", "大吼一声")
		}
		if tick >= 20 && tick < 34 {
			force += 1.0
		}
	case TraitCounterGrip:
		if distanceFromCenter(self) > distanceFromCenter(opponent) {
			force += 1.0
		}
	case TraitLuckyBelly:
		if rng.Float64() < 0.012 {
			force += 2.4
			addEvent(result, tick, float64(tick)*DT, w.ID, "lucky", "肚皮弹反")
		}
	}
	return math.Max(1, force)
}

func resistForce(w Wrestler, self wrestlerState, ownArchetype, opponentArchetype, environment string) float64 {
	resist := 5.0 +
		float64(w.Stats.Weight)*0.62 +
		float64(w.Stats.Balance)*0.58 +
		float64(w.Stats.Stamina)*0.38 +
		float64(w.Stats.Spirit)*0.18
	switch w.Style {
	case StyleDefensive:
		resist += 1.1
	case StyleAggressive:
		resist -= 0.4
	}
	resist += environmentResistBonus(w, environment)
	if self.StumbleTicks > 0 {
		resist *= 0.46
	}
	if self.Energy < 20 {
		resist -= (20 - self.Energy) * 0.06
	}
	resist += archetypeResistBonus(w, self, ownArchetype, opponentArchetype)
	switch w.Trait {
	case TraitIronFeet:
		resist += 1.2
	case TraitLowCenter:
		resist += 1.0
	case TraitCalmBreath:
		if self.Energy < 35 {
			resist += 1.0
		}
	}
	return math.Max(1, resist)
}

func applyStumble(s *wrestlerState, w Wrestler, environment string, tick int, t float64, rng *RNG, result *MatchResult) {
	if s.StumbleTicks > 0 {
		s.StumbleTicks--
		return
	}
	chance := 0.002 + float64(10-w.Stats.Balance)*0.001 + float64(10-w.Stats.Footwork)*0.00045
	if w.Style == StyleAggressive {
		chance += 0.002
	}
	if w.Trait == TraitLowCenter {
		chance *= 0.65
	}
	chance += environmentStumbleBonus(w, environment)
	if rng.Float64() < chance {
		s.StumbleTicks = 7
		addEvent(result, tick, t, w.ID, "stumble", "脚下一滑")
	}
}

func footworkDrift(w Wrestler, environment string, rng *RNG) float64 {
	width := 0.18 + float64(w.Stats.Footwork)*0.070 + float64(w.Stats.Balance)*0.012
	width *= environmentDriftMultiplier(environment)
	if w.Trait == TraitSoftStep {
		width *= 1.8
	}
	if w.Style == StyleTrickster {
		width *= 1.35
	}
	return rng.Range(-width, width)
}

func energyCost(w Wrestler, environment string) float64 {
	cost := 0.38 - float64(w.Stats.Stamina)*0.012
	if w.Style == StyleAggressive {
		cost += 0.08
	}
	if w.Style == StyleDefensive {
		cost -= 0.04
	}
	if w.Trait == TraitCalmBreath {
		cost *= 0.78
	}
	cost *= environmentEnergyMultiplier(environment)
	return cost
}

func environmentPushBonus(w Wrestler, environment string, tick int) float64 {
	switch environment {
	case EnvTenseStart:
		if tick < 28 {
			return float64(w.Stats.Spirit)*0.12 + 0.35
		}
	case EnvCrowdRoar:
		if tick >= 40 {
			return float64(w.Stats.Spirit) * 0.09
		}
	case EnvSoftSand:
		return -0.25 + float64(w.Stats.Footwork)*0.025
	}
	return 0
}

func environmentResistBonus(w Wrestler, environment string) float64 {
	switch environment {
	case EnvFirmRing:
		return float64(w.Stats.Weight)*0.10 + float64(w.Stats.Balance)*0.06
	case EnvSoftSand:
		return -0.20 + float64(w.Stats.Balance)*0.04
	}
	return 0
}

func environmentStumbleBonus(w Wrestler, environment string) float64 {
	switch environment {
	case EnvSoftSand:
		return 0.0025 - float64(w.Stats.Footwork)*0.00018
	case EnvTenseStart:
		return 0.0012 - float64(w.Stats.Spirit)*0.00008
	}
	return 0
}

func environmentDriftMultiplier(environment string) float64 {
	switch environment {
	case EnvSoftSand:
		return 1.35
	case EnvFirmRing:
		return 0.82
	default:
		return 1
	}
}

func environmentEnergyMultiplier(environment string) float64 {
	switch environment {
	case EnvSoftSand:
		return 1.12
	case EnvCrowdRoar:
		return 1.06
	default:
		return 1
	}
}

func tieBreak(a, b wrestlerState, wa, wb Wrestler, rng *RNG) string {
	scoreA := RingRadius - distanceFromCenter(a) +
		float64(wa.Stats.Power)*0.08 +
		float64(wa.Stats.Weight)*0.18 +
		float64(wa.Stats.Balance)*0.16 +
		float64(wa.Stats.Footwork)*0.16 +
		float64(wa.Stats.Stamina)*0.16 +
		float64(wa.Stats.Spirit)*0.16 +
		rng.Range(0, 0.02)
	scoreB := RingRadius - distanceFromCenter(b) +
		float64(wb.Stats.Power)*0.08 +
		float64(wb.Stats.Weight)*0.18 +
		float64(wb.Stats.Balance)*0.16 +
		float64(wb.Stats.Footwork)*0.16 +
		float64(wb.Stats.Stamina)*0.16 +
		float64(wb.Stats.Spirit)*0.16 +
		rng.Range(0, 0.02)
	if scoreA >= scoreB {
		return wa.ID
	}
	return wb.ID
}

func outOfRing(s wrestlerState) bool {
	return distanceFromCenter(s) >= RingRadius
}

func distanceFromCenter(s wrestlerState) float64 {
	return math.Hypot(s.X, s.Y)
}

func unit(x, y float64) (float64, float64) {
	length := math.Hypot(x, y)
	if length == 0 {
		return 1, 0
	}
	return x / length, y / length
}

func snapshot(s wrestlerState) WrestlerSnapshot {
	return WrestlerSnapshot{
		WrestlerID: s.WrestlerID,
		X:          round3(s.X),
		Y:          round3(s.Y),
		Energy:     round3(s.Energy),
		Stumbling:  s.StumbleTicks > 0,
	}
}

func addEvent(result *MatchResult, tick int, t float64, wrestlerID, eventType, message string) {
	result.Events = append(result.Events, ReplayEvent{
		Tick:     tick,
		Time:     round3(t),
		Wrestler: wrestlerID,
		Type:     eventType,
		Message:  message,
	})
}

func traitEnergy(w Wrestler) float64 {
	if w.Trait == TraitCalmBreath {
		return 8
	}
	return 0
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
