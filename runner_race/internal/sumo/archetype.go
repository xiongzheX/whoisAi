package sumo

func Archetype(w Wrestler) string {
	s := w.Stats
	switch {
	case s.Power >= 8 && s.Power >= s.Weight && s.Power >= s.Balance && s.Power >= s.Footwork && s.Power >= s.Stamina && s.Power >= s.Spirit:
		return ArchetypePower
	case s.Footwork+s.Balance >= 15 && s.Footwork >= 7:
		return ArchetypeAgile
	case s.Weight+s.Balance >= 15 && s.Weight >= 7:
		return ArchetypeGuard
	case s.Spirit >= 8 && s.Spirit >= s.Power && s.Spirit >= s.Stamina:
		return ArchetypeSpirit
	case s.Stamina+s.Weight >= 15 && s.Stamina >= 7:
		return ArchetypeEndurance
	default:
		return ArchetypeBalanced
	}
}

func counters(attacker, defender string) bool {
	switch attacker {
	case ArchetypeAgile:
		return defender == ArchetypePower
	case ArchetypePower:
		return defender == ArchetypeEndurance
	case ArchetypeEndurance:
		return defender == ArchetypeSpirit
	case ArchetypeSpirit:
		return defender == ArchetypeGuard
	case ArchetypeGuard:
		return defender == ArchetypeAgile
	default:
		return false
	}
}
