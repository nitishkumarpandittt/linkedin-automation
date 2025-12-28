package stealth

import (
	"math"
	"math/rand"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// SleepRandom sleeps for a random duration between min and max milliseconds
func SleepRandom(minMs, maxMs int) {
	if maxMs < minMs {
		maxMs = minMs
	}
	d := time.Duration(minMs+rand.Intn(maxMs-minMs+1)) * time.Millisecond
	time.Sleep(d)
}

// SleepGaussian sleeps for a duration following a Gaussian distribution
// More realistic than uniform distribution - most delays cluster around mean
func SleepGaussian(meanMs, stdDevMs int) {
	// Use Box-Muller transform for Gaussian distribution
	u1 := rand.Float64()
	u2 := rand.Float64()
	z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
	delay := int(float64(meanMs) + z*float64(stdDevMs))

	// Clamp to reasonable bounds (mean ± 3*stdDev)
	minDelay := meanMs - 3*stdDevMs
	maxDelay := meanMs + 3*stdDevMs
	if delay < minDelay {
		delay = minDelay
	} else if delay > maxDelay {
		delay = maxDelay
	}

	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
}

func ThinkTime() { SleepGaussian(1400, 600) } // Mean 1.4s, StdDev 600ms

// MoveMouseHumanLike moves the mouse along a bezier curve with variable speed,
// natural overshoot, and micro-corrections
func MoveMouseHumanLike(p *rod.Page, fromX, fromY, toX, toY int) error {
	// Calculate distance for speed variance
	dist := math.Sqrt(math.Pow(float64(toX-fromX), 2) + math.Pow(float64(toY-fromY), 2))

	// Longer distances = more steps, but not linear
	baseSteps := 40 + int(dist/20)
	steps := baseSteps + rand.Intn(15)

	// Control points for cubic bezier with more natural curve
	cx1 := fromX + (toX-fromX)/3 + rand.Intn(100) - 50
	cy1 := fromY + (toY-fromY)/3 + rand.Intn(100) - 50
	cx2 := fromX + 2*(toX-fromX)/3 + rand.Intn(100) - 50
	cy2 := fromY + 2*(toY-fromY)/3 + rand.Intn(100) - 50

	// Add natural overshoot (30% chance)
	overshoot := rand.Float64() < 0.3
	var overshootX, overshootY int
	if overshoot {
		overshootMag := 5 + rand.Intn(15)
		angle := math.Atan2(float64(toY-fromY), float64(toX-fromX))
		overshootX = toX + int(float64(overshootMag)*math.Cos(angle))
		overshootY = toY + int(float64(overshootMag)*math.Sin(angle))
	}

	// Main movement
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)

		// Ease-in-out for more natural acceleration
		t = easeInOutCubic(t)

		var x, y int
		if overshoot && i > steps-5 {
			// Overshoot phase
			progress := float64(i-(steps-5)) / 5.0
			x = int(cubicBezier(float64(fromX), float64(cx1), float64(cx2), float64(overshootX), 1.0)*(1-progress) + float64(toX)*progress)
			y = int(cubicBezier(float64(fromY), float64(cy1), float64(cy2), float64(overshootY), 1.0)*(1-progress) + float64(toY)*progress)
		} else {
			x = int(cubicBezier(float64(fromX), float64(cx1), float64(cx2), float64(toX), t))
			y = int(cubicBezier(float64(fromY), float64(cy1), float64(cy2), float64(toY), t))
		}

		// Add micro-jitter for realism
		x += rand.Intn(3) - 1
		y += rand.Intn(3) - 1

		_ = proto.InputDispatchMouseEvent{
			Type: proto.InputDispatchMouseEventTypeMouseMoved,
			X:    float64(x),
			Y:    float64(y),
		}.Call(p)

		// Variable speed - faster in middle, slower at start/end
		delay := 8 + rand.Intn(10)
		if i < 5 || i > steps-5 {
			delay += 5 // Slower at endpoints
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Micro-correction (small adjustments after reaching target)
	if rand.Float64() < 0.4 {
		for j := 0; j < 2; j++ {
			dx := rand.Intn(3) - 1
			dy := rand.Intn(3) - 1
			_ = proto.InputDispatchMouseEvent{
				Type: proto.InputDispatchMouseEventTypeMouseMoved,
				X:    float64(toX + dx),
				Y:    float64(toY + dy),
			}.Call(p)
			time.Sleep(time.Duration(20+rand.Intn(30)) * time.Millisecond)
		}
	}

	return nil
}

// easeInOutCubic provides smooth acceleration and deceleration
func easeInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - math.Pow(-2*t+2, 3)/2
}

// cubicBezier calculates point on cubic bezier curve
func cubicBezier(p0, p1, p2, p3, t float64) float64 {
	return math.Pow(1-t, 3)*p0 +
		3*math.Pow(1-t, 2)*t*p1 +
		3*(1-t)*math.Pow(t, 2)*p2 +
		math.Pow(t, 3)*p3
}

func bezier(p0, p1, p2, t float64) float64 {
	return math.Pow(1-t, 2)*p0 + 2*(1-t)*t*p1 + math.Pow(t, 2)*p2
}

// MouseIdleMovement simulates natural mouse movements when not clicking
// Humans don't keep mouse perfectly still
func MouseIdleMovement(p *rod.Page) error {
	// Always do some movement to make it more visible (changed from 30% to 100%)
	if true { // Always execute for visibility
		// Get window dimensions
		width := 1400 // Default viewport width
		height := 900 // Default viewport height

		if dims, err := p.Eval(`() => ({width: window.innerWidth, height: window.innerHeight})`); err == nil {
			if w := dims.Value.Get("width").Int(); w > 0 {
				width = w
			}
			if h := dims.Value.Get("height").Int(); h > 0 {
				height = h
			}
		}

		// Random point in safe area (not edges)
		margin := 100
		x := margin + rand.Intn(width-2*margin)
		y := margin + rand.Intn(height-2*margin)

		// Get current mouse position (estimate from center)
		fromX := width / 2
		fromY := height / 2

		// First move to a random point with visible bezier movement
		MoveMouseHumanLike(p, fromX, fromY, x, y)
		SleepRandom(200, 500)

		// Small wandering movement (increased count for more visibility)
		for i := 0; i < 3+rand.Intn(4); i++ {
			dx := rand.Intn(40) - 20
			dy := rand.Intn(40) - 20
			_ = proto.InputDispatchMouseEvent{
				Type: proto.InputDispatchMouseEventTypeMouseMoved,
				X:    float64(x + dx),
				Y:    float64(y + dy),
			}.Call(p)
			SleepRandom(100, 400)
		}
	}
	return nil
}

// ClickHumanLike performs a scroll-into-view and a click with realistic mouse movement
func ClickHumanLike(p *rod.Page, el *rod.Element) error {
	_ = el.ScrollIntoView()
	SleepGaussian(300, 150)

	// Get element position
	shape, err := el.Shape()
	if err != nil || len(shape.Quads) == 0 {
		return el.Click("left", 1) // Fallback to simple click
	}

	// Use first quad to get bounding box
	quad := shape.Quads[0]
	minX, maxX := quad[0], quad[0]
	minY, maxY := quad[1], quad[1]
	for i := 0; i < len(quad); i += 2 {
		if quad[i] < minX {
			minX = quad[i]
		}
		if quad[i] > maxX {
			maxX = quad[i]
		}
		if quad[i+1] < minY {
			minY = quad[i+1]
		}
		if quad[i+1] > maxY {
			maxY = quad[i+1]
		}
	}

	width := maxX - minX
	height := maxY - minY

	// Random point within element (more natural than center)
	targetX := int(minX + width*0.3 + rand.Float64()*width*0.4)
	targetY := int(minY + height*0.3 + rand.Float64()*height*0.4)

	// Get current viewport dimensions
	fromX := 700 // Default center
	fromY := 450

	if dims, err := p.Eval(`() => ({width: window.innerWidth, height: window.innerHeight})`); err == nil {
		if w := dims.Value.Get("width").Int(); w > 0 {
			fromX = w / 2
		}
		if h := dims.Value.Get("height").Int(); h > 0 {
			fromY = h / 2
		}
	}

	// Move mouse to element
	_ = MoveMouseHumanLike(p, fromX, fromY, targetX, targetY)

	SleepRandom(50, 150)

	// Mouse down
	_ = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMousePressed,
		X:          float64(targetX),
		Y:          float64(targetY),
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(p)

	// Human reaction time for click
	SleepRandom(30, 90)

	// Mouse up
	_ = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMouseReleased,
		X:          float64(targetX),
		Y:          float64(targetY),
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(p)

	return nil
}

// TypeHumanLike simulates realistic typing with variable delays, occasional typos, and corrections
func TypeHumanLike(el *rod.Element, text string) error {
	for i, r := range text {
		ch := string(r)

		// 2% chance of typo (then correction)
		if rand.Float64() < 0.02 && i > 3 {
			wrongChar := randomNearbyRune(r)
			_ = el.Input(wrongChar)
			SleepRandom(80, 180)

			// Realize mistake and backspace
			_ = el.Input("\b")
			SleepRandom(100, 250)
		}

		if err := el.Input(ch); err != nil {
			return err
		}

		// Realistic typing rhythm
		baseDelay := 25
		if i < 10 {
			baseDelay = 40 // Slower at start (thinking)
		} else if r == ' ' || r == ',' || r == '.' {
			baseDelay = 60 // Pause at punctuation
		} else if i > 0 && text[i-1] == ' ' {
			baseDelay = 35 // Slight pause after space
		}

		// Add Gaussian noise to typing speed
		SleepGaussian(baseDelay, 20)

		// Occasional longer pauses (re-reading, thinking)
		if rand.Float64() < 0.05 {
			SleepGaussian(300, 150)
		}
	}
	return nil
}

func randomNearbyRune(r rune) string {
	// Keyboard-proximity based typos
	nearby := map[rune][]rune{
		'a': {'s', 'q', 'w', 'z'},
		'e': {'w', 'r', 'd'},
		'i': {'u', 'o', 'k', 'j'},
		'o': {'i', 'p', 'l', 'k'},
		's': {'a', 'd', 'w', 'x'},
		't': {'r', 'y', 'g', 'f'},
	}

	if opts, ok := nearby[r]; ok && len(opts) > 0 {
		return string(opts[rand.Intn(len(opts))])
	}

	// Generic fallback
	opts := []rune{'a', 'e', 'i', 'o', 'u', 's', 'n', 't', 'r', 'l'}
	return string(opts[rand.Intn(len(opts))])
}

// ScrollHumanLike scrolls with realistic human patterns
func ScrollHumanLike(p *rod.Page) {
	// Variable number of scroll actions
	steps := 3 + rand.Intn(5)

	for i := 0; i < steps; i++ {
		// Variable scroll distance
		px := 300 + rand.Intn(500)

		// Sometimes scroll in chunks
		if rand.Float64() < 0.3 {
			chunks := 2 + rand.Intn(3)
			for j := 0; j < chunks; j++ {
				_, _ = p.Eval(`(dy) => window.scrollBy({top: dy, behavior: 'smooth'})`, px/chunks)
				SleepRandom(100, 300)
			}
		} else {
			_, _ = p.Eval(`(dy) => window.scrollBy({top: dy, behavior: 'smooth'})`, px)
		}

		SleepGaussian(400, 200)

		// Occasionally pause to "read"
		if rand.Float64() < 0.4 {
			SleepGaussian(1200, 500)
		}
	}

	// Sometimes scroll back up (re-reading)
	if rand.Float64() < 0.4 {
		_, _ = p.Eval(`(dy) => window.scrollBy({top: dy, behavior: 'smooth'})`, -(100 + rand.Intn(120)))
		SleepRandom(300, 700)
	}
}

// RandomHover moves mouse over arbitrary elements (simulates browsing)
func RandomHover(p *rod.Page, selectors []string) {
	if len(selectors) == 0 {
		return
	}

	// Try to hover over 1-2 elements for more visible movement
	attempts := 1 + rand.Intn(2)
	for i := 0; i < attempts && i < len(selectors); i++ {
		sel := selectors[rand.Intn(len(selectors))]
		if el, err := p.Timeout(2 * time.Second).Element(sel); err == nil {
			shape, err := el.Shape()
			if err == nil && len(shape.Quads) > 0 {
				quad := shape.Quads[0]
				// Calculate center of element
				centerX := (quad[0] + quad[2] + quad[4] + quad[6]) / 4
				centerY := (quad[1] + quad[3] + quad[5] + quad[7]) / 4

				// Get viewport center
				fromX := 700
				fromY := 450
				if dims, err := p.Eval(`() => ({width: window.innerWidth, height: window.innerHeight})`); err == nil {
					if w := dims.Value.Get("width").Int(); w > 0 {
						fromX = w / 2
					}
					if h := dims.Value.Get("height").Int(); h > 0 {
						fromY = h / 2
					}
				}

				_ = MoveMouseHumanLike(p, fromX, fromY, int(centerX), int(centerY))
				SleepRandom(300, 800)
			}
		}
	}
}

// WakeUpMovement creates a visible "wake up" mouse movement at the start of page interactions
// Simulates a human moving their mouse when they start engaging with a page
func WakeUpMovement(p *rod.Page) error {
	// Get window dimensions
	width := 1400
	height := 900

	if dims, err := p.Eval(`() => ({width: window.innerWidth, height: window.innerHeight})`); err == nil {
		if w := dims.Value.Get("width").Int(); w > 0 {
			width = w
		}
		if h := dims.Value.Get("height").Int(); h > 0 {
			height = h
		}
	}

	// Start from a corner or edge (like user just came to the window)
	startPositions := []struct{ x, y int }{
		{100, 100},         // Top left
		{width - 100, 100}, // Top right
		{width / 2, 100},   // Top center
		{100, height / 2},  // Left center
	}
	start := startPositions[rand.Intn(len(startPositions))]

	// Move to center-ish area with visible movement
	targetX := width/2 + rand.Intn(200) - 100
	targetY := height/2 + rand.Intn(200) - 100

	MoveMouseHumanLike(p, start.x, start.y, targetX, targetY)
	SleepRandom(300, 600)

	return nil
}

// TakeBreak simulates a human taking a break (checking other tabs, etc.)
func TakeBreak() {
	if rand.Float64() < 0.15 { // 15% chance of taking a break
		breakDuration := 3000 + rand.Intn(5000) // 3-8 seconds
		time.Sleep(time.Duration(breakDuration) * time.Millisecond)
	}
}

// InActiveWindow enforces time window
func InActiveWindow(start, end string) bool {
	now := time.Now()
	s, _ := time.Parse("15:04", start)
	e, _ := time.Parse("15:04", end)
	startToday := time.Date(now.Year(), now.Month(), now.Day(), s.Hour(), s.Minute(), 0, 0, now.Location())
	endToday := time.Date(now.Year(), now.Month(), now.Day(), e.Hour(), e.Minute(), 0, 0, now.Location())
	return now.After(startToday) && now.Before(endToday)
}
