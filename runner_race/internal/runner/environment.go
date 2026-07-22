package runner

const (
	EnvClearTrack = "clear_track"
	EnvTailwind   = "tailwind"
	EnvWetTrack   = "wet_track"
	EnvLoudCrowd  = "loud_crowd"
)

var allEnvironments = []string{
	EnvClearTrack,
	EnvTailwind,
	EnvWetTrack,
	EnvLoudCrowd,
}

func randomEnvironment(rng *RNG) string {
	return allEnvironments[rng.Intn(len(allEnvironments))]
}
