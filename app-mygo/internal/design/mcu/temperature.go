package mcu

import (
	"math"
	"sort"
)

// temperatureCache is temperature_cache.dart, for the fidelity and content
// variants' tertiary palettes.
type temperatureCache struct {
	input     Hct
	byHue     []Hct
	byTemp    []Hct
	temps     map[uint32]float64
	inputTemp float64
}

func newTemperatureCache(input Hct) *temperatureCache {
	t := &temperatureCache{input: input, inputTemp: -1}
	for hue := 0.0; hue <= 360.0; hue += 1.0 {
		t.byHue = append(t.byHue, HctFrom(hue, input.Chroma, input.Tone))
	}
	t.temps = map[uint32]float64{}
	for _, h := range append(append([]Hct{}, t.byHue...), input) {
		t.temps[h.ToInt()] = rawTemperature(h)
	}
	t.byTemp = append(append([]Hct{}, t.byHue...), input)
	sort.SliceStable(t.byTemp, func(i, j int) bool {
		return t.temps[t.byTemp[i].ToInt()] < t.temps[t.byTemp[j].ToInt()]
	})
	return t
}

func (t *temperatureCache) warmest() Hct { return t.byTemp[len(t.byTemp)-1] }
func (t *temperatureCache) coldest() Hct { return t.byTemp[0] }

func (t *temperatureCache) temp(h Hct) float64 { return t.temps[h.ToInt()] }

func (t *temperatureCache) relativeTemperature(h Hct) float64 {
	rng := t.temp(t.warmest()) - t.temp(t.coldest())
	diff := t.temp(h) - t.temp(t.coldest())
	if rng == 0.0 {
		return 0.5
	}
	return diff / rng
}

func (t *temperatureCache) inputRelativeTemperature() float64 {
	if t.inputTemp >= 0.0 {
		return t.inputTemp
	}
	coldest := t.temp(t.coldest())
	rng := t.temp(t.warmest()) - coldest
	diff := t.temp(t.input) - coldest
	if rng == 0.0 {
		t.inputTemp = 0.5
	} else {
		t.inputTemp = diff / rng
	}
	return t.inputTemp
}

func isBetween(angle, a, b float64) bool {
	if a < b {
		return a <= angle && angle <= b
	}
	return a <= angle || angle <= b
}

func (t *temperatureCache) complement() Hct {
	coldestHue := t.coldest().Hue
	coldestTemp := t.temp(t.coldest())
	warmestHue := t.warmest().Hue
	warmestTemp := t.temp(t.warmest())
	rng := warmestTemp - coldestTemp
	startIsColdestToWarmest := isBetween(t.input.Hue, coldestHue, warmestHue)
	startHue, endHue := coldestHue, warmestHue
	if startIsColdestToWarmest {
		startHue, endHue = warmestHue, coldestHue
	}
	smallestError := 1000.0
	answer := t.byHue[dartRound(t.input.Hue)]
	complementRelativeTemp := 1.0 - t.inputRelativeTemperature()
	for addend := 0.0; addend <= 360.0; addend += 1.0 {
		hue := sanitizeDegreesDouble(startHue + addend)
		if !isBetween(hue, startHue, endHue) {
			continue
		}
		possible := t.byHue[dartRound(hue)]
		relative := (t.temp(possible) - coldestTemp) / rng
		if e := math.Abs(complementRelativeTemp - relative); e < smallestError {
			smallestError = e
			answer = possible
		}
	}
	return answer
}

func (t *temperatureCache) analogous(count, divisions int) []Hct {
	startHue := dartRound(t.input.Hue)
	startHct := t.byHue[startHue]
	lastTemp := t.relativeTemperature(startHct)
	all := []Hct{startHct}
	absoluteTotal := 0.0
	for i := 0; i < 360; i++ {
		hct := t.byHue[sanitizeDegreesInt(startHue+i)]
		temp := t.relativeTemperature(hct)
		absoluteTotal += math.Abs(temp - lastTemp)
		lastTemp = temp
	}
	hueAddend := 1
	tempStep := absoluteTotal / float64(divisions)
	total := 0.0
	lastTemp = t.relativeTemperature(startHct)
	for len(all) < divisions {
		hct := t.byHue[sanitizeDegreesInt(startHue+hueAddend)]
		temp := t.relativeTemperature(hct)
		total += math.Abs(temp - lastTemp)
		satisfied := total >= float64(len(all))*tempStep
		indexAddend := 1
		for satisfied && len(all) < divisions {
			all = append(all, hct)
			satisfied = total >= float64(len(all)+indexAddend)*tempStep
			indexAddend++
		}
		lastTemp = temp
		hueAddend++
		if hueAddend > 360 {
			for len(all) < divisions {
				all = append(all, hct)
			}
			break
		}
	}
	answers := []Hct{t.input}
	increase := int(math.Floor(float64(count-1) / 2.0))
	for i := 1; i < increase+1; i++ {
		index := 0 - i
		for index < 0 {
			index = len(all) + index
		}
		if index >= len(all) {
			index = index % len(all)
		}
		answers = append([]Hct{all[index]}, answers...)
	}
	decrease := count - increase - 1
	for i := 1; i < decrease+1; i++ {
		index := i
		if index >= len(all) {
			index = index % len(all)
		}
		answers = append(answers, all[index])
	}
	return answers
}

func rawTemperature(c Hct) float64 {
	lab := labFromArgb(c.ToInt())
	hue := sanitizeDegreesDouble(math.Atan2(lab[2], lab[1]) * 180.0 / math.Pi)
	chroma := math.Sqrt(lab[1]*lab[1] + lab[2]*lab[2])
	return -0.5 + 0.02*math.Pow(chroma, 1.07)*math.Cos(sanitizeDegreesDouble(hue-50.0)*math.Pi/180.0)
}
