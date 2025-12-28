package browser

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/example/linkedbot/internal/config"
	"github.com/example/linkedbot/internal/logging"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

type Browser struct {
	Rod *rod.Browser
	Cfg *config.Config
	log *logging.Logger
}

func New(ctx context.Context, cfg *config.Config) (*Browser, error) {
	log := logging.New(cfg.Logging.Level).With("module", "browser")
	// Use normal launcher but disable leakless to avoid AV false positives on Windows
	l := launcher.New().Leakless(false)
	// Force headful for visibility during runs
	l = l.Headless(false)
	url, err := l.Launch()
	if err != nil {
		return nil, err
	}
	rb := rod.New().ControlURL(url).MustConnect()
	br := &Browser{Rod: rb, Cfg: cfg, log: log}
	if err := br.init(ctx); err != nil {
		return nil, err
	}
	return br, nil
}

func (b *Browser) init(ctx context.Context) error {
	b.Rod = b.Rod.MustIgnoreCertErrors(true)

	// Create a default page for initial stealth setup
	p := b.Rod.MustPage("about:blank")

	// 1. User Agent Randomization
	ua := b.Cfg.Stealth.UserAgent
	if ua == "" {
		// Latest realistic user agents
		uas := []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		}
		ua = uas[rand.Intn(len(uas))]
	}

	// Extract platform info from UA for consistency
	platform := "Win32"
	if contains(ua, "Macintosh") {
		platform = "MacIntel"
	} else if contains(ua, "Linux") {
		platform = "Linux x86_64"
	}

	_ = proto.EmulationSetUserAgentOverride{
		UserAgent: ua,
		Platform:  platform,
	}.Call(p)

	// 2. Viewport Randomization (realistic dimensions)
	w := randRange(b.Cfg.Stealth.ViewportWidthMin, b.Cfg.Stealth.ViewportWidthMax)
	h := randRange(b.Cfg.Stealth.ViewportHeightMin, b.Cfg.Stealth.ViewportHeightMax)
	_ = p.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             w,
		Height:            h,
		DeviceScaleFactor: 1,
		Mobile:            false,
	})

	// 3. Comprehensive Fingerprint Masking
	_, _ = p.Eval(getStealthScript(w, h, platform))

	p.MustClose()
	b.log.Info("browser fingerprint initialized", "ua", ua, "viewport", fmt.Sprintf("%dx%d", w, h))
	return nil
}

// getStealthScript returns comprehensive anti-detection JavaScript
func getStealthScript(width, height int, platform string) string {
	return `(width, height, platform) => {
		// 1. Remove webdriver property
		Object.defineProperty(navigator, 'webdriver', {
			get: () => undefined
		});
		
		// 2. Mock chrome object (makes it look like real Chrome)
		window.chrome = {
			runtime: {},
			loadTimes: function() {},
			csi: function() {},
			app: {}
		};
		
		// 3. Mock plugins (realistic set)
		Object.defineProperty(navigator, 'plugins', {
			get: () => [
				{
					name: 'PDF Viewer',
					filename: 'internal-pdf-viewer',
					description: 'Portable Document Format'
				},
				{
					name: 'Chrome PDF Viewer', 
					filename: 'internal-pdf-viewer',
					description: 'Portable Document Format'
				},
				{
					name: 'Chromium PDF Viewer',
					filename: 'internal-pdf-viewer', 
					description: 'Portable Document Format'
				}
			]
		});
		
		// 4. Mock languages (realistic)
		Object.defineProperty(navigator, 'languages', {
			get: () => ['en-US', 'en']
		});
		
		// 5. Override permission API
		const originalQuery = window.navigator.permissions.query;
		window.navigator.permissions.query = (parameters) => (
			parameters.name === 'notifications' 
				? Promise.resolve({ state: Notification.permission })
				: originalQuery(parameters)
		);
		
		// 6. Mock hardware concurrency (realistic CPU count)
		Object.defineProperty(navigator, 'hardwareConcurrency', {
			get: () => 4 + Math.floor(Math.random() * 8) // 4-12 cores
		});
		
		// 7. Mock device memory
		Object.defineProperty(navigator, 'deviceMemory', {
			get: () => 8
		});
		
		// 8. Canvas fingerprint randomization (slight noise)
		const originalToDataURL = HTMLCanvasElement.prototype.toDataURL;
		HTMLCanvasElement.prototype.toDataURL = function(type) {
			const context = this.getContext('2d');
			const imageData = context.getImageData(0, 0, this.width, this.height);
			
			// Add minimal noise to prevent fingerprinting
			for (let i = 0; i < imageData.data.length; i += 4) {
				if (Math.random() < 0.001) {
					imageData.data[i] = imageData.data[i] + Math.floor(Math.random() * 2) - 1;
				}
			}
			
			context.putImageData(imageData, 0, 0);
			return originalToDataURL.apply(this, arguments);
		};
		
		// 9. WebGL fingerprint masking
		const getParameter = WebGLRenderingContext.prototype.getParameter;
		WebGLRenderingContext.prototype.getParameter = function(parameter) {
			// Mask specific WebGL parameters
			if (parameter === 37445) { // UNMASKED_VENDOR_WEBGL
				return 'Intel Inc.';
			}
			if (parameter === 37446) { // UNMASKED_RENDERER_WEBGL
				return 'Intel Iris OpenGL Engine';
			}
			return getParameter.apply(this, arguments);
		};
		
		// 10. Screen dimensions consistency
		Object.defineProperty(window.screen, 'width', {
			get: () => width + 100 // Slightly larger than viewport
		});
		Object.defineProperty(window.screen, 'height', {
			get: () => height + 100
		});
		Object.defineProperty(window.screen, 'availWidth', {
			get: () => width + 100
		});
		Object.defineProperty(window.screen, 'availHeight', {
			get: () => height + 60 // Account for taskbar
		});
		
		// 11. Platform consistency
		Object.defineProperty(navigator, 'platform', {
			get: () => platform
		});
		
		// 12. Battery API masking (avoid giving extra fingerprint data)
		if ('getBattery' in navigator) {
			navigator.getBattery = () => Promise.resolve({
				charging: true,
				chargingTime: 0,
				dischargingTime: Infinity,
				level: 1.0
			});
		}
		
		// 13. Mock realistic connection
		Object.defineProperty(navigator, 'connection', {
			get: () => ({
				effectiveType: '4g',
				downlink: 10,
				rtt: 50,
				saveData: false
			})
		});
		
		// 14. Timezone consistency
		Date.prototype.getTimezoneOffset = function() {
			return -300; // EST/EDT
		};
	}`
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func (b *Browser) NewPage(ctx context.Context) (*rod.Page, error) {
	p := b.Rod.MustPage("")

	// Set a very long default timeout to handle slow typing operations
	p = p.Timeout(300 * time.Second) // 5 minutes

	// Apply stealth scripts to each new page
	w := randRange(b.Cfg.Stealth.ViewportWidthMin, b.Cfg.Stealth.ViewportWidthMax)
	h := randRange(b.Cfg.Stealth.ViewportHeightMin, b.Cfg.Stealth.ViewportHeightMax)
	platform := "Win32"

	// Apply stealth on every page navigation
	p.EvalOnNewDocument(getStealthScript(w, h, platform))

	return p, nil
}

func (b *Browser) Close() {
	if b.Rod != nil {
		_ = b.Rod.Close()
	}
}

func randRange(min, max int) int {
	if min >= max {
		return min
	}
	return min + rand.Intn(max-min+1)
}

// Helpers
func WaitVisible(p *rod.Page, sel string, d time.Duration) error {
	if err := p.Timeout(d).WaitLoad(); err != nil {
		return err
	}
	el, err := p.Timeout(d).Element(sel)
	if err != nil {
		return err
	}
	return el.WaitVisible()
}

func Click(p *rod.Page, sel string) error {
	el, err := p.Timeout(10 * time.Second).Element(sel)
	if err != nil {
		return err
	}
	if err := el.WaitVisible(); err != nil {
		return err
	}
	return el.Click("left", 1)
}

func Type(p *rod.Page, sel, text string) error {
	el, err := p.Timeout(10 * time.Second).Element(sel)
	if err != nil {
		return err
	}
	if err := el.WaitVisible(); err != nil {
		return err
	}
	return el.Input(text)
}

// ClickByText clicks an element containing specific text
func ClickByText(p *rod.Page, text string) error {
	// Try button first
	el, err := p.Timeout(5*time.Second).ElementR("button", text)
	if err != nil {
		// Try any element
		el, err = p.Timeout(5*time.Second).ElementR("*", text)
	}
	if err != nil {
		return err
	}
	if err := el.WaitVisible(); err != nil {
		return err
	}
	return el.Click("left", 1)
}

// HasElement checks if an element exists
func HasElement(p *rod.Page, sel string) bool {
	_, err := p.Timeout(2 * time.Second).Element(sel)
	return err == nil
}

// HasElementWithText checks if an element with text exists
func HasElementWithText(p *rod.Page, text string) bool {
	_, err := p.Timeout(2*time.Second).ElementR("*", text)
	return err == nil
}

func ScreenshotOnError(p *rod.Page, prefix string, err error) error {
	if p == nil || err == nil {
		return err
	}
	path := fmt.Sprintf("%s-%d.png", prefix, time.Now().Unix())
	bts, _ := p.Screenshot(true, &proto.PageCaptureScreenshot{})
	_ = os.WriteFile(path, bts, 0644)
	return err
}
