package search

import (
	"context"
	"fmt"
	"net/url"
	"os"
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

type Criteria struct {
	Title    string
	Company  string
	Location string
	Keywords string
	Limit    int
}

func New(br *browser.Browser, cfg *config.Config, st *store.Store) *Service {
	return &Service{br: br, cfg: cfg, st: st, log: logging.New(cfg.Logging.Level).With("module", "search")}
}

func (s *Service) SearchAndStoreTargets(ctx context.Context, c Criteria) (int, error) {
	if c.Limit <= 0 {
		c.Limit = s.cfg.Limits.MaxProfilesPerSearch
	}
	p, err := s.br.NewPage(ctx)
	if err != nil {
		return 0, err
	}
	defer p.Close()

	// 1. Build a single, effective keyword string.
	parts := []string{}
	if strings.TrimSpace(c.Title) != "" {
		parts = append(parts, c.Title)
	}
	if strings.TrimSpace(c.Company) != "" {
		parts = append(parts, c.Company)
	}
	if strings.TrimSpace(c.Location) != "" {
		parts = append(parts, c.Location)
	}
	if strings.TrimSpace(c.Keywords) != "" {
		parts = append(parts, c.Keywords)
	}
	kw := strings.Join(parts, " ")

	// 2. Construct the base URL for search.
	baseSearchURL := fmt.Sprintf(
		"%ssearch/results/people/?keywords=%s&origin=GLOBAL_SEARCH_HEADER",
		s.cfg.LinkedIn.BaseURL,
		url.QueryEscape(kw),
	)

	collected := 0
	pageNum := 1
	s.log.Info("starting search", "keywords", kw, "limit", c.Limit)

	// 3. Loop through pages by URL parameter.
	for ; collected < c.Limit; pageNum++ {
		pageURL := fmt.Sprintf("%s&page=%d", baseSearchURL, pageNum)
		s.log.Info("navigating to search page", "url", pageURL)

		if err := p.Navigate(pageURL); err != nil {
			s.log.Warn("failed to navigate to page", "page", pageNum, "err", err)
			break // Stop if navigation fails
		}

		// Wait for the main container of search results to appear.
		if err := p.WaitLoad(); err != nil {
			s.log.Warn("page load failed", "page", pageNum, "err", err)
			break
		}

		// Wake up movement on each search page for visibility
		if pageNum == 1 {
			stealth.WakeUpMovement(p)
		}

		// Wait for the results container to be visible
		_, err = p.Element(".search-results-container")
		if err != nil {
			s.log.Warn("search results container not found", "page", pageNum, "err", err)
			browser.ScreenshotOnError(p, "search_fail", err)
			break
		}

		// Visible mouse movement and hover over search results
		stealth.MouseIdleMovement(p)
		stealth.RandomHover(p, []string{"h3", "div.entity-result__title-text", "a[href*='/in/']"})

		// Scroll to trigger lazy loading.
		stealth.ScrollHumanLike(p)

		// More visible movement during waiting period
		stealth.MouseIdleMovement(p)
		time.Sleep(2500 * time.Millisecond) // Longer pause for JS to render

		// 4. Extract profile links using multiple selector strategies
		var links rod.Elements

		// Strategy 1: Try modern structure with specific attributes
		links, err = p.Elements(`a[href*="/in/"][data-test-app-aware-link]`)
		if err == nil && len(links) > 0 {
			s.log.Info("found links using strategy 1 (data-test-app-aware-link)", "count", len(links))
		} else {
			// Strategy 2: Any link in search results container pointing to /in/
			links, err = p.Elements(`.search-results-container a[href*="/in/"]`)
			if err == nil && len(links) > 0 {
				s.log.Info("found links using strategy 2 (search-results-container)", "count", len(links))
			} else {
				// Strategy 3: Look for list items and then find profile links within
				listItems, _ := p.Elements(`ul[role="list"] li`)
				if len(listItems) > 0 {
					s.log.Info("found list items", "count", len(listItems))
					links = nil
					for _, item := range listItems {
						// Find all links within this list item
						itemLinks, _ := item.Elements(`a[href*="/in/"]`)
						if len(itemLinks) > 0 {
							// Take only the first link (profile link) from each item
							links = append(links, itemLinks[0])
						}
					}
					s.log.Info("found links using strategy 3 (list items)", "count", len(links))
				} else {
					// Strategy 4: Fallback - any anchor with /in/ in the href
					links, _ = p.Elements(`a[href*="/in/"]`)
					s.log.Info("found links using strategy 4 (fallback)", "count", len(links))
				}
			}
		}

		if err != nil {
			s.log.Warn("all selectors failed to find profile links", "page", pageNum, "err", err)
			break
		}

		s.log.Info("links found on page", "page", pageNum, "count", len(links))
		if len(links) == 0 {
			if pageNum > 1 {
				s.log.Info("no more links found, ending search")
			} else {
				s.log.Warn("no links found on first page, search may have failed. Saving debug files.")
				browser.ScreenshotOnError(p, "search_fail", fmt.Errorf("no results"))
				// Save full page HTML for debugging
				html, _ := p.HTML()
				_ = os.WriteFile("search_fail_full.html", []byte(html), 0644)
				// Also save just the container if it exists
				if container, err := p.Element(".search-results-container"); err == nil {
					containerHTML, _ := container.HTML()
					_ = os.WriteFile("search_fail_container.html", []byte(containerHTML), 0644)
				}
			}
			break // End of results
		}

		seenOnPage := map[string]bool{}
		for i, linkEl := range links {
			if collected >= c.Limit {
				s.log.Info("reached collection limit", "collected", collected, "limit", c.Limit)
				break
			}
			href, err := linkEl.Attribute("href")
			if err != nil || href == nil {
				s.log.Debug("skipping link with no href", "index", i)
				continue
			}

			profileURL := normalizeProfileURL(*href)

			// Filter out non-profile links
			if !strings.Contains(profileURL, "/in/") {
				s.log.Debug("skipping non-profile link", "url", profileURL)
				continue
			}

			// Skip duplicates on same page
			if seenOnPage[profileURL] {
				s.log.Debug("skipping duplicate on page", "url", profileURL)
				continue
			}
			seenOnPage[profileURL] = true

			// Try to extract name/headline if available (for better tracking)
			pmodel := models.Profile{LinkedInURL: profileURL}

			// Store in database
			_, err = s.st.UpsertProfile(ctx, &pmodel)
			if err != nil {
				s.log.Warn("failed to store profile", "url", profileURL, "err", err)
				continue
			}

			collected++
			s.log.Info("profile stored", "url", profileURL, "total_collected", collected)
		}

		// If we didn't collect anything on this page, likely end of results
		if len(seenOnPage) == 0 {
			s.log.Info("no unique profiles on this page, ending search")
			break
		}

		// Small delay between pages to be respectful
		if pageNum < 10 && collected < c.Limit {
			stealth.SleepRandom(2000, 4000)
		}
	}

	s.log.Info("search completed", "total_collected", collected, "pages_visited", pageNum-1)
	return collected, nil
}

func normalizeProfileURL(u string) string {
	if i := strings.Index(u, "?"); i >= 0 {
		u = u[:i]
	}
	if !strings.HasPrefix(u, "http") {
		u = "https://www.linkedin.com" + u
	}
	return u
}
