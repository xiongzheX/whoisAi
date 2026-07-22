package runner

import "sort"

func randomCourse(rng *RNG) Course {
	obstacleKinds := []string{ObstacleHurdle, ObstaclePuddle, ObstacleCone}
	obstacles := []CourseObstacle{
		{Position: round3(rng.Range(19, 28)), Kind: obstacleKinds[rng.Intn(len(obstacleKinds))]},
		{Position: round3(rng.Range(44, 55)), Kind: obstacleKinds[rng.Intn(len(obstacleKinds))]},
		{Position: round3(rng.Range(72, 84)), Kind: obstacleKinds[rng.Intn(len(obstacleKinds))]},
	}
	sort.Slice(obstacles, func(i, j int) bool {
		return obstacles[i].Position < obstacles[j].Position
	})

	firstCurveStart := round3(rng.Range(29, 36))
	secondCurveStart := round3(rng.Range(58, 66))
	shape, path := randomCoursePath(rng)
	return Course{
		Obstacles: obstacles,
		Curves: []CourseCurve{
			{
				Start:     firstCurveStart,
				End:       round3(firstCurveStart + rng.Range(9, 13)),
				Direction: randomDirection(rng),
			},
			{
				Start:     secondCurveStart,
				End:       round3(secondCurveStart + rng.Range(10, 15)),
				Direction: randomDirection(rng),
			},
		},
		Shape: shape,
		Path:  path,
	}
}

func randomDirection(rng *RNG) string {
	if rng.Float64() < 0.5 {
		return "left"
	}
	return "right"
}

func randomCoursePath(rng *RNG) (string, []CoursePathPoint) {
	switch rng.Intn(4) {
	case 0:
		return "L型路线", []CoursePathPoint{
			{Meter: 0, X: 0, Y: 50},
			{Meter: 36, X: 36, Y: 50},
			{Meter: 48, X: 42, Y: 24},
			{Meter: 100, X: 100, Y: 24},
		}
	case 1:
		return "T型路线", []CoursePathPoint{
			{Meter: 0, X: 0, Y: 52},
			{Meter: 34, X: 34, Y: 52},
			{Meter: 46, X: 46, Y: 28},
			{Meter: 58, X: 58, Y: 52},
			{Meter: 100, X: 100, Y: 52},
		}
	case 2:
		return "Z型路线", []CoursePathPoint{
			{Meter: 0, X: 0, Y: 34},
			{Meter: 30, X: 30, Y: 34},
			{Meter: 48, X: 48, Y: 68},
			{Meter: 70, X: 70, Y: 68},
			{Meter: 100, X: 100, Y: 36},
		}
	default:
		return "S型路线", []CoursePathPoint{
			{Meter: 0, X: 0, Y: 50},
			{Meter: 24, X: 24, Y: 30},
			{Meter: 48, X: 48, Y: 70},
			{Meter: 74, X: 74, Y: 32},
			{Meter: 100, X: 100, Y: 50},
		}
	}
}
