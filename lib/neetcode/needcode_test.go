package neetcode

import (
	"fmt"
	"sort"
	"testing"
)

func TestNeetCode(t *testing.T) {
	x := []int{100, 200, -100, -200, 300, -300, 400, -400, 500, -500, 600, -600, 700, -700, 800, -800, 900, -900, 1000, -1000, 100}
	sort.Ints(x)
	fmt.Println(x)
}
