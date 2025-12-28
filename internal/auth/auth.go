package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/linkedbot/internal/browser"
	"github.com/example/linkedbot/internal/config"
	"github.com/example/linkedbot/internal/logging"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type Auth struct {
	br  *browser.Browser
	cfg *config.Config
	log *logging.Logger
}

func New(br *browser.Browser, cfg *config.Config) *Auth {
	return &Auth{br: br, cfg: cfg, log: logging.New(cfg.Logging.Level).With("module", "auth")}
}

func (a *Auth) EnsureLoggedIn(ctx context.Context) error {
	p, err := a.br.NewPage(ctx)
	if err != nil {
		return err
	}
	defer p.Close()
	// Try cookies first
	if err := a.loadCookies(p); err == nil {
		if ok := a.validateSession(ctx, p); ok {
			a.log.Info("session validated using cookies")
			return nil
		}
	}
	// Fresh login
	if err := a.login(ctx, p); err != nil {
		return err
	}
	if err := a.saveCookies(p); err != nil {
		a.log.Warn("save cookies failed", "err", err)
	}
	return nil
}

func (a *Auth) login(ctx context.Context, p *rod.Page) error {
	email := os.Getenv("LINKEDIN_EMAIL")
	pass := os.Getenv("LINKEDIN_PASSWORD")
	if email == "" || pass == "" {
		return errors.New("missing LINKEDIN_EMAIL or LINKEDIN_PASSWORD env")
	}

	a.log.Info("attempting login", "email", email)

	// Navigate to login page
	url := a.cfg.LinkedIn.BaseURL + "login"
	if err := p.Navigate(url); err != nil {
		return fmt.Errorf("failed to navigate to login page: %w", err)
	}

	if err := p.WaitLoad(); err != nil {
		return fmt.Errorf("login page load failed: %w", err)
	}

	time.Sleep(1 * time.Second)

	// Check if we're on the right login page
	usernameInput, err := p.Timeout(5 * time.Second).Element("input#username")
	if err != nil {
		// Try alternative login URL
		a.log.Info("trying alternative login URL")
		if err := p.Navigate(a.cfg.LinkedIn.BaseURL + "uas/login"); err != nil {
			return fmt.Errorf("failed to navigate to alternative login: %w", err)
		}
		if err := p.WaitLoad(); err != nil {
			return fmt.Errorf("alternative login page load failed: %w", err)
		}
		usernameInput, err = p.Timeout(5 * time.Second).Element("input#username")
		if err != nil {
			browser.ScreenshotOnError(p, "login_page_fail", err)
			return fmt.Errorf("username input not found: %w", err)
		}
	}

	// Fill email
	a.log.Info("filling email")
	if err := usernameInput.Input(email); err != nil {
		return fmt.Errorf("failed to input email: %w", err)
	}
	time.Sleep(300 * time.Millisecond)

	// Fill password
	a.log.Info("filling password")
	passwordInput, err := p.Timeout(5 * time.Second).Element("input#password")
	if err != nil {
		return fmt.Errorf("password input not found: %w", err)
	}
	if err := passwordInput.Input(pass); err != nil {
		return fmt.Errorf("failed to input password: %w", err)
	}
	time.Sleep(200 * time.Millisecond)

	// Click submit button
	a.log.Info("clicking submit button")
	submitBtn, err := p.Timeout(5 * time.Second).Element("button[type='submit']")
	if err != nil {
		return fmt.Errorf("submit button not found: %w", err)
	}
	if err := submitBtn.Click("left", 1); err != nil {
		return fmt.Errorf("failed to click submit: %w", err)
	}

	// Wait for navigation to complete
	a.log.Info("waiting for navigation after login submit")
	time.Sleep(5 * time.Second)

	// Check if login was successful
	a.log.Info("checking login success", "current_url", p.MustInfo().URL)

	// Strategy 1: Check current URL - successful login usually redirects to feed or home
	currentURL := p.MustInfo().URL
	if strings.Contains(currentURL, "/feed/") || strings.Contains(currentURL, "/feed") {
		a.log.Info("login successful - detected feed URL")
		return nil
	}

	// Strategy 2: Look for LinkedIn header elements that appear when logged in
	success := false
	var successMethod string

	// Check 1: Search box (only visible when logged in)
	if el, err := p.Timeout(5 * time.Second).Element("input[placeholder*='Search'], input[aria-label*='Search']"); err == nil {
		if visible, _ := el.Visible(); visible {
			success = true
			successMethod = "search box"
		}
	}

	// Check 2: Global navigation bar
	if !success {
		if _, err := p.Timeout(3 * time.Second).Element("nav.global-nav, header.global-alert-offset"); err == nil {
			success = true
			successMethod = "navigation bar"
		}
	}

	// Check 3: Feed link in navigation
	if !success {
		if _, err := p.Timeout(3 * time.Second).Element("a[href*='/feed']"); err == nil {
			success = true
			successMethod = "feed link"
		}
	}

	// Check 4: Profile/Me menu
	if !success {
		if _, err := p.Timeout(3 * time.Second).Element("[data-control-name='identity_profile_photo'], .global-nav__me-photo"); err == nil {
			success = true
			successMethod = "profile menu"
		}
	}

	// Check 5: Any element with class containing 'global-nav'
	if !success {
		if _, err := p.Timeout(3 * time.Second).Element("[class*='global-nav']"); err == nil {
			success = true
			successMethod = "global nav element"
		}
	}

	// Check 6: Just check if we're NOT on login page anymore
	if !success {
		if !strings.Contains(currentURL, "/login") && !strings.Contains(currentURL, "/uas/login") {
			// We navigated away from login page - likely successful
			success = true
			successMethod = "navigation away from login page"
		}
	}

	if success {
		a.log.Info("login successful", "detection_method", successMethod, "url", currentURL)
		return nil
	}

	// If we're here, login likely failed
	a.log.Warn("login verification failed, checking for errors")

	// Check for error messages
	if errEl, err := p.Timeout(2 * time.Second).Element(".alert--error, .form__label--error, .error"); err == nil {
		if errText, _ := errEl.Text(); errText != "" {
			a.log.Error("login error message found", "message", errText)
			browser.ScreenshotOnError(p, "login_error", errors.New("login failed"))
			return fmt.Errorf("login failed: %s", errText)
		}
	}

	// Check for verification/checkpoint
	if _, err := p.Timeout(2 * time.Second).Element("[data-test-id='checkpoint'], .challenge-dialog"); err == nil {
		a.log.Error("checkpoint detected")
		browser.ScreenshotOnError(p, "login_checkpoint", errors.New("checkpoint"))
		return errors.New("login blocked by checkpoint/verification - please login manually in browser first")
	}

	// Still on login page?
	if strings.Contains(currentURL, "/login") {
		a.log.Error("still on login page", "url", currentURL)
		browser.ScreenshotOnError(p, "login_still_on_login_page", errors.New("stuck on login"))

		// Try to get page title for debugging
		if title, err := p.Eval("() => document.title"); err == nil {
			a.log.Info("page title", "title", title.Value.String())
		}

		return errors.New("login failed: still on login page after submitting credentials")
	}

	// Unknown state - save debug info
	a.log.Error("login verification failed - unknown state", "url", currentURL)
	browser.ScreenshotOnError(p, "login_unknown_fail", errors.New("unknown login failure"))

	// Save HTML for debugging
	if html, err := p.HTML(); err == nil {
		_ = os.WriteFile("login_fail_page.html", []byte(html), 0644)
		a.log.Info("saved page HTML to login_fail_page.html for debugging")
	}

	return errors.New("login failed: could not verify successful login - check screenshot and login_fail_page.html")
}

func (a *Auth) validateSession(ctx context.Context, p *rod.Page) bool {
	_ = p.Navigate(a.cfg.LinkedIn.BaseURL + "feed/")
	if err := p.WaitLoad(); err != nil {
		return false
	}
	if _, err := p.Element("a[href*='/feed/']"); err == nil {
		return true
	}
	return false
}

func cookiesPath() string {
	return filepath.Join(".cache", "cookies.json")
}

func (a *Auth) loadCookies(p *rod.Page) error {
	b, err := os.ReadFile(cookiesPath())
	if err != nil {
		return err
	}
	var cookies []*proto.NetworkCookie
	if err := json.Unmarshal(b, &cookies); err != nil {
		return err
	}
	for _, c := range cookies {
		_, _ = proto.NetworkSetCookie{Domain: c.Domain, Name: c.Name, Value: c.Value, Path: c.Path, Expires: c.Expires, HTTPOnly: c.HTTPOnly, Secure: c.Secure}.Call(p)
	}
	return nil
}

func (a *Auth) saveCookies(p *rod.Page) error {
	// Increase timeout and retry once to avoid deadline issues
	pp := p.Timeout(20 * time.Second)
	cookies, err := proto.StorageGetCookies{}.Call(pp)
	if err != nil {
		// brief retry
		time.Sleep(500 * time.Millisecond)
		cookies, err = proto.StorageGetCookies{}.Call(pp)
		if err != nil {
			return err
		}
	}
	b, _ := json.MarshalIndent(cookies.Cookies, "", "  ")
	_ = os.MkdirAll(filepath.Dir(cookiesPath()), 0o755)
	return os.WriteFile(cookiesPath(), b, 0644)
}
