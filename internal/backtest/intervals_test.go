package backtest

import (
	"fmt"
	"testing"
	"time"
)

func TestSplitIntoWeeks(t *testing.T) {
	i := SplitIntoWeeks(time.Now().Add(-12*31*24*time.Hour).UTC(), time.Now().UTC())
	for _, v := range i {
		fmt.Printf("%v - %v\n", v.Start, v.End)
		for _, h := range DivideIntoHours(v.Start, v.End) {
			fmt.Printf("hour - %v\n", h)
		}
	}
}
