package chart

import "time"

// Supported periods of the statistics screen.
const (
	// PeriodWeek shows the last seven days.
	PeriodWeek = "Week"
	// PeriodMonth shows the last thirty days.
	PeriodMonth = "Month"
	// PeriodYear shows the last twelve months.
	PeriodYear = "Year"
)

// Periods lists the supported periods in the order of the keyboard.
var Periods = []string{PeriodWeek, PeriodMonth, PeriodYear}

// Bucket is one slot of the axis of a period.
type Bucket struct {
	// Key is the storage key of the bucket, for example 2024-05-17.
	Key string
	// Label is the short name shown below the axis.
	Label string
}

// Axis builds the buckets of a period that end with the given day.
func Axis(period string, now time.Time) []Bucket {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch period {
	case PeriodYear:
		monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
		buckets := make([]Bucket, 0, 12)
		for offset := 11; offset >= 0; offset-- {
			month := monthStart.AddDate(0, -offset, 0)
			buckets = append(buckets, Bucket{Key: month.Format("2006-01"), Label: month.Format("Jan 06")})
		}
		return buckets
	case PeriodMonth:
		buckets := make([]Bucket, 0, 30)
		for offset := 29; offset >= 0; offset-- {
			day := today.AddDate(0, 0, -offset)
			buckets = append(buckets, Bucket{Key: day.Format("2006-01-02"), Label: day.Format("Jan 02")})
		}
		return buckets
	default:
		buckets := make([]Bucket, 0, 7)
		for offset := 6; offset >= 0; offset-- {
			day := today.AddDate(0, 0, -offset)
			buckets = append(buckets, Bucket{Key: day.Format("2006-01-02"), Label: day.Format("Mon")})
		}
		return buckets
	}
}

// Values returns the value of every bucket for the given counts.
//
// Daily counts are summed up into the month buckets of the yearly period.
func Values(buckets []Bucket, counts map[string]int) []int {
	values := make([]int, len(buckets))
	for index, bucket := range buckets {
		if len(bucket.Key) == len("2006-01") {
			prefix := bucket.Key + "-"
			for key, count := range counts {
				if len(key) > len(prefix) && key[:len(prefix)] == prefix {
					values[index] += count
				}
			}
			continue
		}
		values[index] += counts[bucket.Key]
	}
	return values
}

// Points turns the values of a bucket axis into chart points.
func Points(buckets []Bucket, counts map[string]int) []Point {
	values := Values(buckets, counts)
	points := make([]Point, 0, len(buckets))
	for index, bucket := range buckets {
		points = append(points, Point{Label: bucket.Label, Value: values[index]})
	}
	return points
}

// Peak returns the largest value of a series together with its label.
func Peak(points []Point) (int, string) {
	peak, label := 0, "-"
	for _, point := range points {
		if point.Value > peak {
			peak, label = point.Value, point.Label
		}
	}
	return peak, label
}

// Total returns the sum of the values of a series.
func Total(points []Point) int {
	total := 0
	for _, point := range points {
		total += point.Value
	}
	return total
}

// Average returns the mean value of a series.
func Average(points []Point) float64 {
	if len(points) == 0 {
		return 0
	}
	return float64(Total(points)) / float64(len(points))
}
