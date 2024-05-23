package main

import (
	"bufio"
	"fmt"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"image/color"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	received, err := os.Open("received")
	if err != nil {
		log.Fatalln(err)
	}
	type Data struct {
		id        uint64
		timestamp uint64
	}
	var data []Data
	var scanner = bufio.NewScanner(received)
	for scanner.Scan() {
		var strData = strings.Split(scanner.Text(), " ")
		i, err := strconv.ParseUint(strData[0], 10, 64)
		if err != nil {
			log.Fatalln(err)
		}
		timestamp, err := strconv.ParseUint(strData[1], 10, 64)
		if err != nil {
			log.Fatalln(err)
		}
		data = append(data, Data{
			id:        i,
			timestamp: timestamp,
		})
	}

	//sort.Slice(data, func(i, j int) bool {
	//	return data[i].timestamp < data[j].timestamp
	//})

	var dd = map[int64]uint64{}
	var last uint64
	var total = uint64(len(data))
	for _, i := range data {
		if last == 0 {
			last = i.timestamp
			continue
		}
		dd[int64(i.timestamp)-int64(last)]++
		last = i.timestamp
	}

	var ddd = map[int64]uint64{}
	for i := range dd {
		ddd[i/1000000]++
	}

	fmt.Printf("total: %d\n", total)
	var bins []plotter.HistogramBin
	for i, v := range ddd {
		//fmt.Printf("%2d: %.8f %d\n", i, float64(v)/float64(total), v)
		//if !(-1000 < i && i < 1000) {
		//	continue
		//}

		if i == 1 {
			v = 0
		}
		bins = append(bins, plotter.HistogramBin{
			Min:    float64(i)/1000 - 0.5,
			Max:    float64(i)/1000 + 0.5,
			Weight: float64(v),
		})
	}

	var histogram = &plotter.Histogram{
		Bins:      bins,
		Width:     2,
		FillColor: color.Gray{Y: 128},
		LineStyle: plotter.DefaultLineStyle,
	}

	var p = plot.New()
	p.Add(histogram)
	err = p.Save(10000, 10000, "main.png")
	if err != nil {
		panic(err)
	}
}

func foo() {
	var action int
	action = 1

	var data string
	var result string

	data = "##"

	switch action {
	case 0:
		var i = 0
		fmt.Println("case 0:", i, data)
		result = strconv.Itoa(i)
	case 1:
		var j = "bar"
		fmt.Println("case 1:", j, data)
		result = j + data
	}

	fmt.Println(result)
}

type Action0 struct {
	i int
}

func (c Action0) Do(data string) string {
	c.i = 0
	fmt.Println("case 0:", c.i, data)
	return strconv.Itoa(c.i)
}

type Action1 struct {
	j string
}

func (c Action1) Parse(i int) {

}

func (c Action1) Do(data string) string {
	c.j = "bar"
	fmt.Println("case 1:", c.j, data)
	return c.j + data
}

type Cond interface {
	Do(data string) string
}

func foo_oop() {
	var cond Cond

	var result = cond.Do("###")

	fmt.Println(result)
}
