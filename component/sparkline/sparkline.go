package sparkline

import (
	"fmt"
	"image"
	"sync"

	ui "github.com/gizak/termui/v3"
	"github.com/sqshq/sampler/component"
	"github.com/sqshq/sampler/component/util"
	"github.com/sqshq/sampler/config"
	"github.com/sqshq/sampler/console"
	"github.com/sqshq/sampler/data"
)

// SparkLine displays general shape of a measurement variation over time
type SparkLine struct {
	*ui.Block
	*data.Consumer
	values   []float64
	maxValue float64
	minValue float64
	scale    int
	gradient []ui.Color
	palette  console.Palette
	mutex    *sync.Mutex
	min      *float64
	max      *float64
}

func NewSparkLine(c config.SparkLineConfig, palette console.Palette) *SparkLine {

	line := &SparkLine{
		Block:    component.NewBlock(c.Title, true, palette),
		Consumer: data.NewConsumer(),
		values:   []float64{},
		scale:    *c.Scale,
		gradient: *c.Gradient,
		palette:  palette,
		mutex:    &sync.Mutex{},
		min:      c.Min,
		max:      c.Max,
	}

	fmt.Println("Test: ", line.min)

	go func() {
		for {
			select {
			case sample := <-line.SampleChannel:
				line.consumeSample(sample)
			case alert := <-line.AlertChannel:
				line.Alert = alert
			}
		}
	}()

	return line
}

func (s *SparkLine) consumeSample(sample *data.Sample) {

	float, err := util.ParseFloat(sample.Value)
	if err != nil {
		s.HandleConsumeFailure("Failed to parse a number", err, sample)
		return
	}

	s.HandleConsumeSuccess()

	curValue := float
	if s.min != nil {
		curValue = max(*s.min, curValue)
	}

	if s.max != nil {
		curValue = min(*s.max, curValue)
	}

	s.values = append(s.values, curValue)
	max, min := s.values[0], s.values[0]

	for i := len(s.values) - 1; i >= 0; i-- {
		if len(s.values)-i > s.Dx() {
			break
		}
		if s.values[i] > max {
			max = s.values[i]
		}
		if s.values[i] < min {
			min = s.values[i]
		}
	}

	s.maxValue = max
	s.minValue = min

	if len(s.values)%100 == 0 {
		s.mutex.Lock()
		s.trimOutOfRangeValues(s.Dx())
		s.mutex.Unlock()
	}
}

func (s *SparkLine) trimOutOfRangeValues(maxSize int) {
	if maxSize < len(s.values) {
		s.values = append(s.values[:0], s.values[len(s.values)-maxSize:]...)
	}
}

func (s *SparkLine) Draw(buffer *ui.Buffer) {

	s.mutex.Lock()

	textStyle := ui.NewStyle(s.palette.BaseColor)

	height := s.Dy() - 2
	minLabel := util.FormatValue(s.minValue, s.scale)
	maxLabel := util.FormatValue(s.maxValue, s.scale)

	minValue := s.minValue
	maxValue := s.maxValue

	if s.min != nil {
		minLabel = util.FormatValue(*s.min, s.scale)
		minValue = *s.min
	}

	if s.max != nil {
		maxLabel = util.FormatValue(*s.max, s.scale)
		maxValue = *s.max
	}

	curLabel := util.FormatValue(0, s.scale)

	if len(s.values) > 0 {
		curLabel = util.FormatValue(s.values[len(s.values)-1], s.scale)
	}

	indent := 2 + util.Max([]int{
		len(minLabel), len(maxLabel), len(curLabel),
	})

	for i := len(s.values) - 1; i >= 0; i-- {

		n := len(s.values) - i

		if n > s.Dx()-indent-3 {
			break
		}

		top := 0

		if maxValue != minValue {
			top = int((s.values[i] - minValue) * float64(height) / (maxValue - minValue))
		}

		for j := 0; j <= top; j++ {
			buffer.SetCell(ui.NewCell(console.SymbolVerticalBar, ui.NewStyle(console.GetGradientColor(s.gradient, j, height))), image.Pt(s.Inner.Max.X-n-indent, s.Inner.Max.Y-j-1))
		}

		if i == len(s.values)-1 {
			buffer.SetString(curLabel, textStyle, image.Pt(s.Inner.Max.X-n-indent+2, s.Inner.Max.Y-top-1))
			if maxValue != minValue {
				buffer.SetString(minLabel, textStyle, image.Pt(s.Inner.Max.X-n-indent+2, s.Max.Y-2))
				buffer.SetString(maxLabel, textStyle, image.Pt(s.Inner.Max.X-n-indent+2, s.Min.Y+1))
			}
		}
	}

	s.mutex.Unlock()

	s.Block.Draw(buffer)
	component.RenderAlert(s.Alert, s.Rectangle, buffer)
}
