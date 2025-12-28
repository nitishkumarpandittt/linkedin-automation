package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/example/linkedbot/internal/auth"
	"github.com/example/linkedbot/internal/browser"
	"github.com/example/linkedbot/internal/config"
	"github.com/example/linkedbot/internal/connection"
	"github.com/example/linkedbot/internal/logging"
	"github.com/example/linkedbot/internal/messaging"
	"github.com/example/linkedbot/internal/search"
	"github.com/example/linkedbot/internal/store"
)

func main() {
	ctx := context.Background()

	// Global flags
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "config.yaml", "Path to config file")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `linkedbot - LinkedIn automation CLI (PoC)

Usage:
  linkedbot [--config config.yaml] <command> [options]

Commands:
  login                          Ensure logged in session (with cookie reuse)
  search [--title T --company C --location L --keywords K --limit N]
                                  Search and store target profiles
  send-connections [--limit N]   Send up to N connection requests
  send-messages [--limit N]      Send follow-up messages to newly accepted connections
  run-all                        Run login, search, send-connections, send-messages in order

Examples:
  linkedbot --config config.yaml login
  linkedbot search --title "Software Engineer" --location "India" --limit 100
`)
	}

	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}

	// Load config
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load error: %v\n", err)
		os.Exit(1)
	}
	log := logging.New(cfg.Logging.Level)
	log.Info("linkedbot starting", "version", "0.1.0")
	log.Info("config loaded", "db_path", cfg.Database.Path, "log_level", cfg.Logging.Level)

	// init store
	st, err := store.Open(cfg.Database.Path)
	if err != nil {
		log.Error("db open failed", "err", err)
		os.Exit(1)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		log.Error("db migration failed", "err", err)
		os.Exit(1)
	}

	cmd := flag.Arg(0)
	log.Info("executing command", "command", cmd)
	switch cmd {
	case "login":
		err = runLogin(ctx, cfg)
	case "search":
		err = runSearch(ctx, cfg, st)
	case "send-connections":
		err = runSendConnections(ctx, cfg, st)
	case "send-messages":
		err = runSendMessages(ctx, cfg, st)
	case "run-all":
		err = runAll(ctx, cfg, st)
	default:
		err = fmt.Errorf("unknown command: %s", cmd)
	}

	if err != nil {
		log.Error("command failed", "cmd", cmd, "err", err)
		fmt.Fprintf(os.Stderr, "\n❌ Command failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "💡 Tip: Run with LINKEDBOT_LOG_LEVEL=debug for more details\n")
		os.Exit(1)
	}
	log.Info("command completed successfully", "cmd", cmd)
	fmt.Printf("\n✅ %s completed successfully\n", cmd)
}

func runLogin(ctx context.Context, cfg *config.Config) error {
	br, err := browser.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer br.Close()
	au := auth.New(br, cfg)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	return au.EnsureLoggedIn(ctx)
}

func runSearch(ctx context.Context, cfg *config.Config, st *store.Store) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	var title, company, location, keywords string
	var limit int
	fs.StringVar(&title, "title", cfg.Search.Defaults.Title, "Job title filter")
	fs.StringVar(&company, "company", cfg.Search.Defaults.Company, "Company filter")
	fs.StringVar(&location, "location", cfg.Search.Defaults.Location, "Location filter")
	fs.StringVar(&keywords, "keywords", cfg.Search.Defaults.Keywords, "Keywords filter")
	fs.IntVar(&limit, "limit", cfg.Limits.MaxProfilesPerSearch, "Max profiles to collect in this run")
	if err := fs.Parse(flag.Args()[1:]); err != nil {
		return err
	}

	br, err := browser.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer br.Close()
	au := auth.New(br, cfg)
	if err := au.EnsureLoggedIn(ctx); err != nil {
		return err
	}

	svc := search.New(br, cfg, st)
	crit := search.Criteria{Title: title, Company: company, Location: location, Keywords: keywords, Limit: limit}
	newCount, err := svc.SearchAndStoreTargets(ctx, crit)
	if err != nil {
		return err
	}
	logging.New(cfg.Logging.Level).Info("search complete", "new_profiles", newCount)
	return nil
}

func runSendConnections(ctx context.Context, cfg *config.Config, st *store.Store) error {
	fs := flag.NewFlagSet("send-connections", flag.ContinueOnError)
	var limit int
	fs.IntVar(&limit, "limit", cfg.Limits.MaxConnectionsPerDay, "Max connections to send in this run")
	if err := fs.Parse(flag.Args()[1:]); err != nil {
		return err
	}

	br, err := browser.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer br.Close()
	au := auth.New(br, cfg)
	if err := au.EnsureLoggedIn(ctx); err != nil {
		return err
	}

	svc := connection.New(br, cfg, st)
	sent, err := svc.SendConnections(ctx, limit)
	if err != nil {
		return err
	}
	logging.New(cfg.Logging.Level).Info("connections sent", "count", sent)
	return nil
}

func runSendMessages(ctx context.Context, cfg *config.Config, st *store.Store) error {
	fs := flag.NewFlagSet("send-messages", flag.ContinueOnError)
	var limit int
	fs.IntVar(&limit, "limit", cfg.Limits.MaxMessagesPerDay, "Max follow-up messages to send in this run")
	if err := fs.Parse(flag.Args()[1:]); err != nil {
		return err
	}

	br, err := browser.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer br.Close()
	au := auth.New(br, cfg)
	if err := au.EnsureLoggedIn(ctx); err != nil {
		return err
	}

	svc := messaging.New(br, cfg, st)
	sent, err := svc.SendFollowUps(ctx, limit)
	if err != nil {
		return err
	}
	logging.New(cfg.Logging.Level).Info("messages sent", "count", sent)
	return nil
}

func runAll(ctx context.Context, cfg *config.Config, st *store.Store) error {
	if err := runLogin(ctx, cfg); err != nil {
		return err
	}
	if _, ok := os.LookupEnv("RUN_SEARCH"); ok {
		if err := runSearch(ctx, cfg, st); err != nil {
			return err
		}
	}
	if _, ok := os.LookupEnv("RUN_CONNECT"); ok {
		if err := runSendConnections(ctx, cfg, st); err != nil {
			return err
		}
	}
	if _, ok := os.LookupEnv("RUN_MESSAGE"); ok {
		if err := runSendMessages(ctx, cfg, st); err != nil {
			return err
		}
	}
	return nil
}
