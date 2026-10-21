// Package chart renders the statistics chart of the Iota Coder bot as a PNG
// image. It only uses the standard library so the bot does not need a
// plotting dependency.
package chart

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
)

// Palette holds the colors of the chart.
type Palette struct {
	Background color.RGBA
	Plot       color.RGBA
	Grid       color.RGBA
	Text       color.RGBA
	Muted      color.RGBA
	Chip       color.RGBA
	ChipBorder color.RGBA
	Accent     color.RGBA
	Series     []color.RGBA
}

// DefaultPalette returns the dark palette of the bot.
func DefaultPalette() Palette {
	return Palette{
		Background: color.RGBA{0x08, 0x11, 0x1F, 0xFF},
		Plot:       color.RGBA{0x0F, 0x1B, 0x2D, 0xFF},
		Grid:       color.RGBA{0x22, 0x32, 0x4A, 0xFF},
		Text:       color.RGBA{0xE2, 0xE8, 0xF0, 0xFF},
		Muted:      color.RGBA{0x94, 0xA3, 0xB8, 0xFF},
		Chip:       color.RGBA{0x0D, 0x17, 0x28, 0xFF},
		ChipBorder: color.RGBA{0x22, 0x32, 0x4A, 0xFF},
		Accent:     color.RGBA{0x38, 0xBD, 0xF8, 0xFF},
		Series: []color.RGBA{
			{0x38, 0xBD, 0xF8, 0xFF},
			{0x6C, 0x5D, 0xD3, 0xFF},
			{0xF5, 0x9E, 0x0B, 0xFF},
			{0x34, 0xD3, 0x99, 0xFF},
			{0xF8, 0x71, 0x71, 0xFF},
			{0xF4, 0x72, 0xB6, 0xFF},
		},
	}
}

// Point is one data point of a series.
type Point struct {
	Label string
	Value int
}

// Series is a named list of points.
type Series struct {
	Name   string
	Points []Point
}

// Chip is a small summary box drawn in the header of the chart.
type Chip struct {
	Title string
	Value string
}

// Options configures the rendering.
type Options struct {
	Title    string
	Subtitle string
	Series   []Series
	Chips    []Chip
	Width    int
	Height   int
	Palette  Palette
}

// Render draws the chart and returns the PNG bytes.
func Render(opts Options) ([]byte, error) {
	if opts.Width <= 0 {
		opts.Width = 1100
	}
	if opts.Height <= 0 {
		opts.Height = 600
	}
	if len(opts.Palette.Series) == 0 {
		opts.Palette = DefaultPalette()
	}

	canvas := image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))
	drawBackground(canvas, opts.Palette)

	drawText(canvas, opts.Title, 70, 34, 3, opts.Palette.Text)
	if opts.Subtitle != "" {
		drawText(canvas, opts.Subtitle, 70, 76, 2, opts.Palette.Muted)
	}
	drawChips(canvas, opts.Chips, opts.Width-40, 30, opts.Palette)

	left, top := 80, 150
	right, bottom := opts.Width-40, opts.Height-70
	if right <= left || bottom <= top {
		return nil, fmt.Errorf("the chart area is empty")
	}

	maximum := 0
	for _, series := range opts.Series {
		for _, point := range series.Points {
			if point.Value > maximum {
				maximum = point.Value
			}
		}
	}
	if maximum <= 0 {
		maximum = 1
	}

	drawGrid(canvas, left, top, right, bottom, maximum, opts.Palette)

	for index, series := range opts.Series {
		seriesColor := opts.Palette.Accent
		if index < len(opts.Palette.Series) {
			seriesColor = opts.Palette.Series[index]
		}
		drawSeries(canvas, series, left, top, right, bottom, maximum, seriesColor, opts.Palette)
	}

	drawLabels(canvas, opts.Series, left, right, bottom, opts.Palette)
	drawLegend(canvas, opts.Series, left, top-34, opts.Palette)

	buffer := &bytes.Buffer{}
	if err := png.Encode(buffer, canvas); err != nil {
		return nil, fmt.Errorf("encode the chart: %w", err)
	}
	return buffer.Bytes(), nil
}

// drawBackground fills the canvas with the background color.
func drawBackground(canvas *image.RGBA, palette Palette) {
	for y := 0; y < canvas.Bounds().Dy(); y++ {
		for x := 0; x < canvas.Bounds().Dx(); x++ {
			canvas.Set(x, y, palette.Background)
		}
	}
}

// drawGrid paints the horizontal grid lines and the axis values.
func drawGrid(canvas *image.RGBA, left, top, right, bottom, maximum int, palette Palette) {
	steps := 4
	for step := 0; step <= steps; step++ {
		y := bottom - (bottom-top)*step/steps
		for x := left; x < right; x++ {
			canvas.Set(x, y, palette.Grid)
		}
		value := maximum * step / steps
		label := fmt.Sprintf("%d", value)
		drawText(canvas, label, left-14-textWidth(label, 2), y-7, 2, palette.Muted)
	}
}

// drawSeries paints the area and the line of a series.
func drawSeries(canvas *image.RGBA, series Series, left, top, right, bottom, maximum int, line color.RGBA, palette Palette) {
	if len(series.Points) == 0 {
		return
	}

	count := len(series.Points)
	step := 0
	if count > 1 {
		step = (right - left) / (count - 1)
	}

	fill := blend(palette.Plot, line, 0.18)
	previousX, previousY := -1, -1
	for index, point := range series.Points {
		x := left + index*step
		if count == 1 {
			x = (left + right) / 2
		}
		height := (bottom - top) * point.Value / maximum
		y := bottom - height
		if y < top {
			y = top
		}

		if previousX >= 0 {
			drawArea(canvas, previousX, previousY, x, y, bottom, fill)
			drawLine(canvas, previousX, previousY, x, y, line, 2)
		}
		previousX, previousY = x, y

		drawRect(canvas, x-3, y-3, x+3, y+3, line)
	}
}

// drawArea fills the space between the line and the bottom of the plot.
func drawArea(canvas *image.RGBA, x0, y0, x1, y1, bottom int, fill color.RGBA) {
	steps := int(math.Max(math.Abs(float64(x1-x0)), math.Abs(float64(y1-y0))))
	if steps == 0 {
		steps = 1
	}
	for step := 0; step <= steps; step++ {
		x := x0 + (x1-x0)*step/steps
		y := y0 + (y1-y0)*step/steps
		for row := y; row <= bottom; row++ {
			canvas.Set(x, row, fill)
		}
	}
}

// drawLabels paints the labels of the first series below the plot.
func drawLabels(canvas *image.RGBA, series []Series, left, right, bottom int, palette Palette) {
	if len(series) == 0 || len(series[0].Points) == 0 {
		return
	}
	points := series[0].Points
	count := len(points)
	step := 0
	if count > 1 {
		step = (right - left) / (count - 1)
	}
	for index, point := range points {
		if count > 8 && index%2 == 1 {
			continue
		}
		x := left + index*step
		width := textWidth(point.Label, 2)
		drawText(canvas, point.Label, x-width/2, bottom+16, 2, palette.Text)
	}
}

// drawLegend paints the names of the series above the plot.
func drawLegend(canvas *image.RGBA, series []Series, x, y int, palette Palette) {
	if len(series) <= 1 {
		return
	}
	cursor := x
	for index, item := range series {
		seriesColor := palette.Accent
		if index < len(palette.Series) {
			seriesColor = palette.Series[index]
		}
		drawRect(canvas, cursor, y+2, cursor+10, y+12, seriesColor)
		cursor += 16
		drawText(canvas, item.Name, cursor, y, 2, palette.Text)
		cursor += textWidth(item.Name, 2) + 26
	}
}

// drawChips paints the summary boxes in the header of the chart.
func drawChips(canvas *image.RGBA, chips []Chip, right, top int, palette Palette) {
	if len(chips) == 0 {
		return
	}
	cursor := right
	for index := len(chips) - 1; index >= 0; index-- {
		chip := chips[index]
		width := textWidth(chip.Value, 2)
		if titleWidth := textWidth(chip.Title, 2); titleWidth > width {
			width = titleWidth
		}
		width += 24
		height := 44
		boxLeft := cursor - width
		drawRect(canvas, boxLeft, top, cursor, top+height, palette.Chip)
		drawFrame(canvas, boxLeft, top, cursor, top+height, palette.ChipBorder)
		drawText(canvas, chip.Title, boxLeft+12, top+7, 2, palette.Muted)
		drawText(canvas, chip.Value, boxLeft+12, top+24, 2, palette.Text)
		cursor = boxLeft - 16
	}
}

// drawFrame paints the border of a rectangle.
func drawFrame(canvas *image.RGBA, x0, y0, x1, y1 int, color color.RGBA) {
	for x := x0; x <= x1; x++ {
		canvas.Set(x, y0, color)
		canvas.Set(x, y1, color)
	}
	for y := y0; y <= y1; y++ {
		canvas.Set(x0, y, color)
		canvas.Set(x1, y, color)
	}
}

// drawLine paints a straight line.
func drawLine(canvas *image.RGBA, x0, y0, x1, y1 int, color color.RGBA, thickness int) {
	steps := int(math.Max(math.Abs(float64(x1-x0)), math.Abs(float64(y1-y0))))
	if steps == 0 {
		steps = 1
	}
	for step := 0; step <= steps; step++ {
		x := x0 + (x1-x0)*step/steps
		y := y0 + (y1-y0)*step/steps
		for offset := 0; offset < thickness; offset++ {
			canvas.Set(x, y+offset, color)
			canvas.Set(x+offset, y, color)
		}
	}
}

// drawRect paints a filled rectangle.
func drawRect(canvas *image.RGBA, x0, y0, x1, y1 int, color color.RGBA) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if image.Pt(x, y).In(canvas.Bounds()) {
				canvas.Set(x, y, color)
			}
		}
	}
}

// blend mixes two colors with the given weight of the second one.
func blend(base, overlay color.RGBA, weight float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(base.R)*(1-weight) + float64(overlay.R)*weight),
		G: uint8(float64(base.G)*(1-weight) + float64(overlay.G)*weight),
		B: uint8(float64(base.B)*(1-weight) + float64(overlay.B)*weight),
		A: 0xFF,
	}
}

// textWidth returns the width of a text in pixels.
func textWidth(value string, scale int) int {
	longest := 0
	for _, line := range strings.Split(value, "\n") {
		width := 0
		for _, r := range line {
			width += (glyphWidth(r) + 1) * scale
		}
		if width > longest {
			longest = width
		}
	}
	return longest
}

// drawText paints a text with the built in bitmap font.
func drawText(canvas *image.RGBA, value string, x, y, scale int, color color.RGBA) {
	for _, line := range strings.Split(value, "\n") {
		cursor := x
		for _, r := range line {
			glyph := glyphs[upperRune(r)]
			for row := 0; row < len(glyph); row++ {
				for column := 0; column < len(glyph[row]); column++ {
					if glyph[row][column] != '1' {
						continue
					}
					for dy := 0; dy < scale; dy++ {
						for dx := 0; dx < scale; dx++ {
							canvas.Set(cursor+column*scale+dx, y+row*scale+dy, color)
						}
					}
				}
			}
			cursor += (glyphWidth(r) + 1) * scale
		}
		y += (glyphHeight + 2) * scale
	}
}

// upperRune normalises a rune for the font lookup.
func upperRune(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

// glyphWidth returns the width of a glyph in font cells.
func glyphWidth(r rune) int {
	if glyph, ok := glyphs[upperRune(r)]; ok {
		return len(glyph[0])
	}
	return glyphWidthFallback
}

// glyphHeight is the height of a glyph in font cells.
const glyphHeight = 7

// glyphWidthFallback is the width of an unknown glyph.
const glyphWidthFallback = 5
