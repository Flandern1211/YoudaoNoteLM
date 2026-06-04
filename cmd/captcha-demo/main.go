package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"time"
)

const (
	w          = 300
	h          = 150
	sliderSize = 40
)

func main() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	sliderX := sliderSize + r.Intn(w-3*sliderSize)
	sliderY := sliderSize + r.Intn(h-2*sliderSize)
	fmt.Printf("slider position: X=%d Y=%d\n", sliderX, sliderY)

	bg := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			noise := r.Intn(20) - 10
			bg.Set(x, y, color.RGBA{uint8(100 + noise), uint8(130 + noise), uint8(180 + noise + (x*30)/w), 255})
		}
	}
	for y := sliderY; y < sliderY+sliderSize; y++ {
		for x := sliderX; x < sliderX+sliderSize; x++ {
			o := bg.RGBAAt(x, y)
			bg.Set(x, y, color.RGBA{uint8(int(o.R) * 60 / 100), uint8(int(o.G) * 60 / 100), uint8(int(o.B) * 60 / 100), 80})
		}
	}
	for i := 0; i < 5; i++ {
		drawLine(bg, r.Intn(w), r.Intn(h), r.Intn(w), r.Intn(h), color.RGBA{200, 200, 200, 80})
	}
	f1, _ := os.Create("captcha_background.png")
	png.Encode(f1, bg)
	f1.Close()

	sl := image.NewRGBA(image.Rect(0, 0, sliderSize, sliderSize))
	for y := 0; y < sliderSize; y++ {
		for x := 0; x < sliderSize; x++ {
			sl.Set(x, y, bg.RGBAAt(sliderX+x, sliderY+y))
		}
	}
	b := color.RGBA{255, 255, 255, 200}
	for i := 0; i < sliderSize; i++ {
		sl.Set(i, 0, b)
		sl.Set(i, sliderSize-1, b)
		sl.Set(0, i, b)
		sl.Set(sliderSize-1, i, b)
	}
	f2, _ := os.Create("captcha_slider.png")
	png.Encode(f2, sl)
	f2.Close()
	fmt.Println("done: captcha_background.png + captcha_slider.png")
}

func drawLine(img *image.RGBA, x1, y1, x2, y2 int, c color.RGBA) {
	dx, dy := abs(x2-x1), abs(y2-y1)
	sx, sy := 1, 1
	if x1 >= x2 {
		sx = -1
	}
	if y1 >= y2 {
		sy = -1
	}
	err := dx - dy
	for {
		img.Set(x1, y1, c)
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
