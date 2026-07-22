package sumo

import "math/rand"

type RNG struct {
	r *rand.Rand
}

func NewRNG(seed int64) *RNG {
	return &RNG{r: rand.New(rand.NewSource(seed))}
}

func (r *RNG) Float64() float64 {
	return r.r.Float64()
}

func (r *RNG) Intn(n int) int {
	return r.r.Intn(n)
}

func (r *RNG) Range(min, max float64) float64 {
	return min + r.Float64()*(max-min)
}

func RandomTrait(rng *RNG) string {
	return AllTraits[rng.Intn(len(AllTraits))]
}
