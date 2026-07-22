package sumo

const (
	EnvFirmRing   = "firm_ring"
	EnvSoftSand   = "soft_sand"
	EnvTenseStart = "tense_start"
	EnvCrowdRoar  = "crowd_roar"
)

var allEnvironments = []string{
	EnvFirmRing,
	EnvSoftSand,
	EnvTenseStart,
	EnvCrowdRoar,
}

func randomEnvironment(rng *RNG) string {
	return allEnvironments[rng.Intn(len(allEnvironments))]
}
