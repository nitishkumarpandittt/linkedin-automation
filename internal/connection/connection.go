package connection

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/example/linkedbot/internal/browser"
	"github.com/example/linkedbot/internal/config"
	"github.com/example/linkedbot/internal/logging"
	"github.com/example/linkedbot/internal/models"
	"github.com/example/linkedbot/internal/stealth"
	"github.com/example/linkedbot/internal/store"
	"github.com/go-rod/rod"
)

type Service struct {
	br  *browser.Browser
	cfg *config.Config
	st  *store.Store
	log *logging.Logger
}

func New(br *browser.Browser, cfg *config.Config, st *store.Store) *Service {
	return &Service{br: br, cfg: cfg, st: st, log: logging.New(cfg.Logging.Level).With("module", "connection")}
}

func (s *Service) SendConnections(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = s.cfg.Limits.MaxConnectionsPerDay
	}
	// respect daily cap
	today, err := s.st.CountActionsToday(ctx, "profiles", "")
	if err == nil && today >= s.cfg.Limits.MaxConnectionsPerDay {
		s.log.Info("daily connection cap reached", "count", today)
		return 0, nil
	}
	toSend := limit
	if capLeft := s.cfg.Limits.MaxConnectionsPerDay - today; toSend > capLeft {
		toSend = capLeft
	}

	profiles, err := s.st.GetProfilesNeedingConnection(ctx, toSend)
	if err != nil {
		return 0, err
	}

	s.log.Info("profiles to connect with", "count", len(profiles))
	if len(profiles) == 0 {
		return 0, nil
	}

	// Check active window once at the start
	if !stealth.InActiveWindow(s.cfg.Stealth.ActiveStart, s.cfg.Stealth.ActiveEnd) {
		s.log.Warn("currently outside configured active window",
			"active_hours", fmt.Sprintf("%s-%s", s.cfg.Stealth.ActiveStart, s.cfg.Stealth.ActiveEnd),
			"current_time", time.Now().Format("15:04"))
		s.log.Info("continuing anyway - to enforce active hours, run during the configured window or update config.yaml")
	}

	p, err := s.br.NewPage(ctx)
	if err != nil {
		return 0, err
	}
	defer p.Close()
	sent := 0
	for _, prof := range profiles {
		s.log.Info("processing profile", "url", prof.LinkedInURL)
		if err := s.sendOne(ctx, p, &prof); err != nil {
			s.log.Warn("send connection failed", "url", prof.LinkedInURL, "err", err)
			continue
		}
		sent++
		stealth.SleepRandom(s.cfg.Stealth.MinDelayMs+300, s.cfg.Stealth.MaxDelayMs+900)
	}
	return sent, nil
}

func (s *Service) sendOne(ctx context.Context, p *rod.Page, prof *models.Profile) error {
	if err := p.Navigate(prof.LinkedInURL); err != nil {
		return err
	}
	if err := p.WaitLoad(); err != nil {
		return err
	}

	// Wake up movement - visible mouse movement from edge to center
	stealth.WakeUpMovement(p)

	// Additional idle movement for natural feel
	stealth.MouseIdleMovement(p)
	stealth.ThinkTime()

	stealth.ScrollHumanLike(p)
	time.Sleep(1 * time.Second)

	// Random hover over page elements to appear natural
	stealth.RandomHover(p, []string{"h1", "div.pv-text-details__left-panel", "button"})

	// Extract profile information if not already present
	if prof.Name == "" || prof.Headline == "" || prof.Company == "" {
		s.log.Info("extracting profile information")
		s.extractProfileInfo(p, prof)
	}

	// Visible mouse movement before looking for connect button
	stealth.MouseIdleMovement(p)
	stealth.SleepRandom(500, 1000)

	// Find Connect button using multiple strategies
	var connectBtn *rod.Element
	var err error

	// Strategy 1: Direct Connect button by aria-label
	connectBtn, err = p.Timeout(5 * time.Second).Element(`button[aria-label*="Invite"][aria-label*="connect"]`)
	if err != nil {
		// Strategy 2: Connect button by text using ElementR
		connectBtn, err = p.Timeout(5*time.Second).ElementR("button", "^Connect$")
	}
	if err != nil {
		// Strategy 3: Try to find and click More button first
		moreBtn, err2 := p.Timeout(3*time.Second).ElementR("button", "More")
		if err2 == nil {
			s.log.Info("clicking More button")
			_ = stealth.ClickHumanLike(p, moreBtn)
			time.Sleep(800 * time.Millisecond)
			// Now try to find Connect in dropdown
			connectBtn, err = p.Timeout(5*time.Second).ElementR("div", "^Connect$")
		}
	}

	if err != nil {
		browser.ScreenshotOnError(p, "connect_button_fail", err)
		return fmt.Errorf("connect button not found: %w", err)
	}

	s.log.Info("found connect button, clicking")
	if err := stealth.ClickHumanLike(p, connectBtn); err != nil {
		return fmt.Errorf("failed to click connect: %w", err)
	}
	time.Sleep(1 * time.Second)

	// Try to add a note
	addNoteBtn, err := p.Timeout(5*time.Second).ElementR("button", "Add a note")
	if err == nil {
		s.log.Info("clicking Add a note")
		_ = stealth.ClickHumanLike(p, addNoteBtn)
		time.Sleep(800 * time.Millisecond)
		// Visible movement after clicking
		stealth.MouseIdleMovement(p)
	} else {
		s.log.Info("Add a note button not found, trying with default message")
	}

	// Type note if textarea available
	note := renderTemplate(s.cfg.Templates.ConnectionNote, prof)
	if len(note) > 280 {
		note = note[:280]
	}

	// Find textarea - use page default timeout for typing operations
	// First check if it exists with a short timeout
	_, err = p.Timeout(5 * time.Second).Element(`textarea[name="message"]`)
	if err == nil {
		// Re-acquire the element without custom timeout so it uses page's 180s default
		textarea, err := p.Element(`textarea[name="message"]`)
		if err == nil {
			s.log.Info("typing note into textarea", "length", len(note))
			if err := stealth.TypeHumanLike(textarea, note); err != nil {
				return fmt.Errorf("failed to type note: %w", err)
			}
			s.log.Info("note typed successfully")
		} else {
			s.log.Warn("failed to re-acquire textarea", "err", err)
		}
	} else {
		s.log.Info("textarea not found, sending without custom note")
	}

	time.Sleep(1 * time.Second)

	// Click Send button - use reasonable timeout
	var sendBtn *rod.Element
	sendBtn, err = p.Timeout(15*time.Second).ElementR("button", "Send")
	if err != nil {
		// Try alternative selector
		sendBtn, err = p.Timeout(15 * time.Second).Element(`button[aria-label*="Send"]`)
	}
	if err != nil {
		// Last resort - try finding Send button by inspecting all buttons
		buttons, _ := p.Elements("button")
		for _, btn := range buttons {
			if text, _ := btn.Text(); text == "Send" || text == "Send invitation" {
				sendBtn = btn
				err = nil
				break
			}
		}
	}
	if err != nil || sendBtn == nil {
		browser.ScreenshotOnError(p, "send_button_fail", err)
		return fmt.Errorf("send button not found: %w", err)
	}

	// Visible movement before final send
	stealth.MouseIdleMovement(p)
	stealth.SleepRandom(300, 700)

	s.log.Info("clicking send button")
	if err := stealth.ClickHumanLike(p, sendBtn); err != nil {
		return fmt.Errorf("failed to click send: %w", err)
	}

	// Movement after sending
	stealth.MouseIdleMovement(p)
	time.Sleep(1 * time.Second)

	// Mark as sent in database
	if err := s.st.MarkConnectionSent(ctx, prof.ID, note); err != nil {
		return fmt.Errorf("failed to mark connection sent: %w", err)
	}

	s.log.Info("connection request sent successfully", "url", prof.LinkedInURL)
	return nil
}

func (s *Service) extractProfileInfo(p *rod.Page, prof *models.Profile) {
	// Extract name from h1 heading
	if nameEl, err := p.Timeout(3 * time.Second).Element("h1"); err == nil {
		if name, err := nameEl.Text(); err == nil {
			prof.Name = strings.TrimSpace(name)
			s.log.Info("extracted name", "name", prof.Name)
		}
	}

	// Extract headline/title - usually in a div after the h1
	// Try multiple selectors for headline
	headlineSelectors := []string{
		`div.text-body-medium`,
		`div[class*="headline"]`,
		`.pv-text-details__left-panel div:nth-child(2)`,
	}

	for _, sel := range headlineSelectors {
		if headlineEl, err := p.Timeout(2 * time.Second).Element(sel); err == nil {
			if headline, err := headlineEl.Text(); err == nil {
				headline = strings.TrimSpace(headline)
				// Make sure it's not the name
				if headline != prof.Name && len(headline) > 0 {
					prof.Headline = headline
					s.log.Info("extracted headline", "headline", prof.Headline)
					break
				}
			}
		}
	}

	// Extract company from the headline or experience section
	// The company is often in the headline like "Software Engineer at Company"
	if prof.Company == "" && prof.Headline != "" {
		// Try to extract company from headline using "at" keyword
		if idx := strings.Index(strings.ToLower(prof.Headline), " at "); idx >= 0 {
			prof.Company = strings.TrimSpace(prof.Headline[idx+4:])
			s.log.Info("extracted company from headline", "company", prof.Company)
		}
	}

	// If we still don't have company, try the experience section
	if prof.Company == "" {
		if companyEl, err := p.Timeout(2 * time.Second).Element(`#experience ~ div span[aria-hidden="true"]`); err == nil {
			if company, err := companyEl.Text(); err == nil {
				prof.Company = strings.TrimSpace(company)
				s.log.Info("extracted company from experience", "company", prof.Company)
			}
		}
	}

	// Update profile in database with extracted info
	if prof.Name != "" || prof.Headline != "" || prof.Company != "" {
		ctx := context.Background()
		if _, err := s.st.UpsertProfile(ctx, prof); err != nil {
			s.log.Warn("failed to update profile info", "err", err)
		}
	}
}

func renderTemplate(t string, p *models.Profile) string {
	name := p.Name
	company := p.Company
	title := p.Headline

	// Extract first name only for more personal touch
	firstName := name
	if idx := strings.Index(name, " "); idx > 0 {
		firstName = name[:idx]
	}

	// Simplify long headlines - extract just the job title part
	// Remove everything after @ or | symbols
	if idx := strings.Index(title, "@"); idx > 0 {
		title = strings.TrimSpace(title[:idx])
	} else if idx := strings.Index(title, "|"); idx > 0 {
		title = strings.TrimSpace(title[:idx])
	} else if idx := strings.Index(title, " at "); idx > 0 {
		// Handle "Software Engineer at Company" format
		title = strings.TrimSpace(title[:idx])
	}

	// Limit title length to avoid exceeding message limits
	if len(title) > 50 {
		// Take first 50 chars and try to end at a word boundary
		title = title[:50]
		if idx := strings.LastIndex(title, " "); idx > 20 {
			title = title[:idx]
		}
	}

	r := strings.NewReplacer(
		"{{Name}}", firstName,
		"{{Company}}", company,
		"{{Title}}", title,
		"{{Keywords}}", "",
	)
	return r.Replace(t)
}
