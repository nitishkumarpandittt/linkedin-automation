package messaging

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
	return &Service{br: br, cfg: cfg, st: st, log: logging.New(cfg.Logging.Level).With("module", "messaging")}
}

func (s *Service) SendFollowUps(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = s.cfg.Limits.MaxMessagesPerDay
	}
	// respect daily cap
	today, err := s.st.CountActionsToday(ctx, "message_logs", string(models.MessageTypeFollowUp))
	if err == nil && today >= s.cfg.Limits.MaxMessagesPerDay {
		return 0, fmt.Errorf("daily message cap reached: %d", today)
	}
	toSend := limit
	if capLeft := s.cfg.Limits.MaxMessagesPerDay - today; toSend > capLeft {
		toSend = capLeft
	}

	// First detect acceptances
	if err := s.detectAcceptances(ctx, 30); err != nil {
		s.log.Warn("acceptance detection partial", "err", err)
	}

	profiles, err := s.st.GetProfilesNeedingFollowUp(ctx, toSend)
	if err != nil {
		return 0, err
	}
	p, err := s.br.NewPage(ctx)
	if err != nil {
		return 0, err
	}
	defer p.Close()
	sent := 0
	for _, prof := range profiles {
		if err := s.messageOne(ctx, p, &prof); err != nil {
			s.log.Warn("send message failed", "url", prof.LinkedInURL, "err", err)
			continue
		}
		sent++
		stealth.SleepRandom(s.cfg.Stealth.MinDelayMs+300, s.cfg.Stealth.MaxDelayMs+1200)
	}
	return sent, nil
}

func (s *Service) detectAcceptances(ctx context.Context, batch int) error {
	p, err := s.br.NewPage(ctx)
	if err != nil {
		return err
	}
	defer p.Close()
	cands, err := s.st.GetPendingAcceptanceChecks(ctx, batch)
	if err != nil {
		return err
	}

	s.log.Info("checking for accepted connections", "count", len(cands))

	for _, cand := range cands {
		if err := p.Navigate(cand.LinkedInURL); err != nil {
			s.log.Warn("failed to navigate", "url", cand.LinkedInURL, "err", err)
			continue
		}
		_ = p.WaitLoad()
		time.Sleep(1 * time.Second)

		// Check if Message button exists (indicates connection accepted)
		if browser.HasElementWithText(p, "Message") || browser.HasElement(p, `button[aria-label*="Message"]`) {
			s.log.Info("connection accepted", "url", cand.LinkedInURL)
			_ = s.st.MarkAccepted(ctx, cand.ID)
		}
		stealth.SleepRandom(300, 900)
	}
	return nil
}

func (s *Service) messageOne(ctx context.Context, p *rod.Page, prof *models.Profile) error {
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

	// Random hover to appear natural
	stealth.RandomHover(p, []string{"h1", "div", "section"})
	time.Sleep(1 * time.Second)

	// Ensure we have profile information
	if prof.Name == "" || prof.Headline == "" || prof.Company == "" {
		s.log.Info("extracting profile information for messaging")
		s.extractProfileInfo(p, prof)
	}

	// Find and click Message button
	msgBtn, err := p.Timeout(5*time.Second).ElementR("button", "^Message$")
	if err != nil {
		msgBtn, err = p.Timeout(5 * time.Second).Element(`button[aria-label*="Message"]`)
	}
	if err != nil {
		return fmt.Errorf("message button not found: %w", err)
	}

	// Visible movement before clicking message
	stealth.MouseIdleMovement(p)

	s.log.Info("clicking message button")
	if err := stealth.ClickHumanLike(p, msgBtn); err != nil {
		return fmt.Errorf("failed to click message button: %w", err)
	}

	// Movement after message box opens
	stealth.MouseIdleMovement(p)
	time.Sleep(1500 * time.Millisecond)

	// Type message
	msg := renderTemplate(s.cfg.Templates.FollowUp, prof)

	// Try to find the message input field
	var msgInput *rod.Element
	_, err = p.Timeout(8 * time.Second).Element(`div.msg-form__contenteditable`)
	if err == nil {
		// Re-acquire without timeout to use page's default 180s timeout
		msgInput, err = p.Element(`div.msg-form__contenteditable`)
	} else {
		// Try alternative selectors
		_, err = p.Timeout(5 * time.Second).Element(`div[contenteditable="true"]`)
		if err == nil {
			msgInput, err = p.Element(`div[contenteditable="true"]`)
		}
	}
	if err != nil || msgInput == nil {
		browser.ScreenshotOnError(p, "message_input_fail", err)
		return fmt.Errorf("message input not found: %w", err)
	}

	s.log.Info("typing message", "length", len(msg))
	if err := stealth.TypeHumanLike(msgInput, msg); err != nil {
		return fmt.Errorf("failed to type message: %w", err)
	}
	s.log.Info("message typed successfully")

	time.Sleep(1 * time.Second)

	// Click Send button
	var sendBtn *rod.Element
	sendBtn, err = p.Timeout(15 * time.Second).Element(`button.msg-form__send-button`)
	if err != nil {
		sendBtn, err = p.Timeout(15*time.Second).ElementR("button", "Send")
	}
	if err != nil {
		// Fallback - find any button with Send text
		buttons, _ := p.Elements("button")
		for _, btn := range buttons {
			if text, _ := btn.Text(); text == "Send" {
				sendBtn = btn
				err = nil
				break
			}
		}
	}
	if err != nil || sendBtn == nil {
		browser.ScreenshotOnError(p, "send_message_fail", err)
		return fmt.Errorf("send button not found: %w", err)
	}

	// Visible movement before final send
	stealth.MouseIdleMovement(p)
	stealth.SleepRandom(400, 800)

	s.log.Info("clicking send button")
	if err := stealth.ClickHumanLike(p, sendBtn); err != nil {
		return fmt.Errorf("failed to click send: %w", err)
	}

	// Movement after sending
	stealth.MouseIdleMovement(p)
	time.Sleep(1 * time.Second)

	if err := s.st.MarkMessageSent(ctx, prof.ID, msg); err != nil {
		return fmt.Errorf("failed to mark message sent: %w", err)
	}

	s.log.Info("message sent successfully", "url", prof.LinkedInURL)
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

	// Extract headline/title
	headlineSelectors := []string{
		`div.text-body-medium`,
		`div[class*="headline"]`,
		`.pv-text-details__left-panel div:nth-child(2)`,
	}

	for _, sel := range headlineSelectors {
		if headlineEl, err := p.Timeout(2 * time.Second).Element(sel); err == nil {
			if headline, err := headlineEl.Text(); err == nil {
				headline = strings.TrimSpace(headline)
				if headline != prof.Name && len(headline) > 0 {
					prof.Headline = headline
					s.log.Info("extracted headline", "headline", prof.Headline)
					break
				}
			}
		}
	}

	// Extract company from headline
	if prof.Company == "" && prof.Headline != "" {
		if idx := strings.Index(strings.ToLower(prof.Headline), " at "); idx >= 0 {
			prof.Company = strings.TrimSpace(prof.Headline[idx+4:])
			s.log.Info("extracted company from headline", "company", prof.Company)
		}
	}

	// Update profile in database
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

	// Extract first name only
	firstName := name
	if idx := strings.Index(name, " "); idx > 0 {
		firstName = name[:idx]
	}

	// Simplify long headlines - extract just the job title part
	if idx := strings.Index(title, "@"); idx > 0 {
		title = strings.TrimSpace(title[:idx])
	} else if idx := strings.Index(title, "|"); idx > 0 {
		title = strings.TrimSpace(title[:idx])
	} else if idx := strings.Index(title, " at "); idx > 0 {
		title = strings.TrimSpace(title[:idx])
	}

	// Limit title length
	if len(title) > 50 {
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
