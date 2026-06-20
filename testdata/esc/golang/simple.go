package testdata

import "fmt"

type Calculator struct {
	Value int
}

func (c *Calculator) Add(n int) {
	c.Value += n
	fmt.Println("Added", n)
}

func Add(a, b int) int {
	return a + b
}

func Main() {
	calc := &Calculator{Value: 0}
	calc.Add(5)
	res := Add(1, 2)
	fmt.Println(res)
}
