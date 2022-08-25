package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sync/atomic"
	"time"
)

var coord []point
var mag []point
var done chan struct{}
var resChan chan ene
var paraChan chan para

type point struct {
	x, y, z float64
}

//全读取
func readFromOut(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		r := point{}
		m := point{}
		fmt.Sscanf(line, "%f %f %f %f %f %f", &r.x, &r.y, &r.z, &m.x, &m.y, &m.z)
		coord = append(coord, r)
		mag = append(mag, m)
	}
}

//读前lines行
func readLines(filename string, lines int) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for i := 0; i < lines; i++ {
		scanner.Scan()
		line := scanner.Text()
		//fmt.Println(line)
		r := point{}
		m := point{}
		fmt.Sscanf(line, "%f %f %f %f %f %f", &r.x, &r.y, &r.z, &m.x, &m.y, &m.z)
		coord = append(coord, r)
		mag = append(mag, m)
	}
}

//calDe的入参
type para struct {
	r1 point
	r2 point
	m1 point
	m2 point
}

type ene struct {
	energyRaw float64
	energy    float64
	step      int64
}

var step int64

//常数，可能有更精确的值
const mu_0 = 4.0 * math.Pi * 1e-7
const mu_e = 1.25663706e-6
const MAXLENGTH = 884736

func calDe(p para) {
	dr := point{p.r1.x - p.r2.x, p.r1.y - p.r2.y, p.r1.z - p.r2.z}
	dr1 := math.Sqrt(dr.x*dr.x + dr.y*dr.y + dr.z*dr.z)
	dr = point{dr.x / dr1, dr.y / dr1, dr.z / dr1}
	deRaw := 3*(p.m1.x*dr.x+p.m1.y*dr.y+p.m1.z*dr.z)*(p.m2.x*dr.x+p.m2.y*dr.y+p.m2.z*dr.z) - (p.m1.x*p.m2.x + p.m1.y*p.m2.y + p.m1.z*p.m2.z)
	de := -deRaw * mu_0 / 4 / math.Pi / (dr1 * dr1 * dr1) / 0.5 / 0.5 * mu_e * mu_e
	//fmt.Println(de)
	atomic.AddInt64(&step, 1)
	e := ene{deRaw, de, step}
	resChan <- e
}

func writeStepToFile(w io.Writer, e ene) {
	//
	fmt.Fprintf(w, "%d\t%e\t%e\n", e.step, e.energyRaw, e.energy)
}

var result float64 = 0.0

const paraChanSize = 10000
const resChanSize = 10000

const writeStep = 100

func main() {
	//defer profile.Start(profile.ProfilePath("."), profile.MemProfileRate(1024), profile.NoShutdownHook).Stop()

	coord = make([]point, 0)
	mag = make([]point, 0)
	//读1000行
	readLines("./data", 10000)
	//参数管道，缓冲区10
	paraChan = make(chan para, paraChanSize)
	//结果管道，缓冲区10
	resChan = make(chan ene, resChanSize)
	done = make(chan struct{})
	SIZE1 := len(coord)

	//计算次数
	count := SIZE1 * (SIZE1 - 1) / 2
	fmt.Println("count:", count)
	outputStep := count / writeStep
	startTime := time.Now()

	//打开一个文件
	file, err := os.OpenFile("out", os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	//开个协程，用来传参数到paraChan
	go func() {
		for i := 0; i < SIZE1; i++ {
			for j := i + 1; j < SIZE1; j++ {
				//fmt.Println(i, j)
				paraChan <- para{coord[i], coord[j], mag[i], mag[j]}
			}
		}
	}()

	//子协程，select有paraChan中的参数进入时，开新协程计算de，并将结果写入resChan
	go func() {
		for {
			select {
			case pp := <-paraChan:
				//fmt.Println("START calDe")
				go calDe(pp)

			case <-done:
				//关闭done后，返回
				now := time.Since(startTime)
				fmt.Fprintln(file, "energy", result, "Time", now, "Step", step)
				fmt.Println("DONE", result, "Time", now, "Step", step)
				return
			}
		}
	}()

	//step, energyRaw, energy
	//file.Write([]byte("step\tenergyRaw\tenergy\n"))
	//主协程，从resChan中读取结果
	for i := 0; i < count; i++ {
		r := <-resChan
		//将r写入file,科学计数法
		//fmt.Println(r.step, r.energyRaw, r.energy)
		//if i%writeStep == 0 || i == count-1 {
		//	go func() { }()
		//}
		if i%outputStep == 0 {
			//输出百分比和时间
			now := time.Since(startTime)
			fmt.Println(float64(i)/float64(count)*100, "%", now)
			writeStepToFile(file, r)
		}
		result += r.energy
	}

	//读完关闭done管道
	close(done)
	time.Sleep(1 * time.Second)

}
