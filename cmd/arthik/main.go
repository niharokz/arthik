// Command arthik is a double-entry personal finance app (a Bluecoins-style PWA) that
// keeps its data as YAML .md files.
//
//	arthik serve            run the web app (default)
//	arthik hash-password    print a bcrypt hash for ARTHIK_PASSWORD
//	arthik check [--demo]   validate the data files, list problems (exit 1 if any)
//	arthik rebuild          re-read everything and rewrite all computed values
//	arthik demo-seed        regenerate the demo dataset now
//	arthik health           exit 0 if the running server answers (Docker healthcheck)
//	arthik summary          month spend vs budget and the inbox, as JSON (phone widget)
//	arthik bans             list banned addresses
//	arthik unban IP|all     lift a ban
//	arthik version
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // zone data compiled in, so TZ works on minimal images

	"gitlab.com/niharokz/arthik/internal/ai"
	"gitlab.com/niharokz/arthik/internal/auth"
	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/config"
	"gitlab.com/niharokz/arthik/internal/demo"
	"gitlab.com/niharokz/arthik/internal/feature/state"
	"gitlab.com/niharokz/arthik/internal/feature/summary"
	"gitlab.com/niharokz/arthik/internal/guard"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/inbox"
	"gitlab.com/niharokz/arthik/internal/server"
)

// version is set at build time: -ldflags "-X main.version=1.0.1".
var version = "1.0.1"

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	_ = config.LoadDotEnv(".env")
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "hash-password":
		err = hashPassword()
	case "check":
		err = check(len(os.Args) > 2 && os.Args[2] == "--demo")
	case "rebuild":
		err = rebuild()
	case "demo-seed":
		err = demoSeed()
	case "health":
		err = health()
	case "summary":
		err = printSummary()
	case "parse-sms":
		err = parseSMS(os.Args[2:])
	case "bans":
		err = listBans()
	case "unban":
		err = unban(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("arthik", version)
	case "help", "--help", "-h":
		fmt.Println(strings.TrimSpace(usage))
	default:
		err = fmt.Errorf("unknown command %q\n\n%s", cmd, strings.TrimSpace(usage))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "arthik:", err)
		os.Exit(1)
	}
}

const usage = `
usage: arthik <command>

  serve            run the web app (default)
  hash-password    print a bcrypt hash to use as ARTHIK_PASSWORD
  check [--demo]   validate the data files and list problems
  rebuild          re-read all files and rewrite every computed value
  demo-seed        regenerate the demo dataset
  health           check that the running server answers
  summary          this month's spending and the inbox as JSON (read-only)
  parse-sms TEXT   show how an SMS would be read (uses <data>/inbox/sms_rules.md too)
  bans             list addresses banned after failed sign-ins
  unban IP|all     lift a ban (the running server notices at once)
  version          print the version
`

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	state.Version = version
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.ExitOnEOF { // phone app: stop when the app process goes away
		go func() {
			_, _ = io.Copy(io.Discard, os.Stdin)
			log.Printf("parent app closed — exiting")
			stop()
		}()
	}

	// The owner's dataset (absent in local mode until a folder is chosen).
	var accounts []*auth.Account
	var own *book.Book
	if cfg.DataDir != "" {
		name := "main"
		if cfg.Local() {
			name = "phone"
		}
		own, err = book.OpenWith(cfg.DataDir, book.Options{Name: name, Replica: !cfg.Primary})
		if err != nil {
			return err
		}
		go own.Watch(ctx, cfg.WatchEvery)
		go own.RunScheduler(ctx, 30*time.Second)
		ir := &inbox.Runner{Book: own, Cfg: cfg.Inbox}
		if cfg.Inbox.AIEnabled() {
			ir.AI = ai.New(cfg.Inbox.AIURL, cfg.Inbox.AIModel, cfg.Inbox.AITimeout)
			log.Printf("inbox AI: %s at %s", cfg.Inbox.AIModel, cfg.Inbox.AIURL)
		}
		go ir.Run(ctx, cfg.WatchEvery)
		pw := cfg.Password
		if cfg.Local() {
			pw = "" // no password sign-in on the phone
		}
		accounts = append(accounts, &auth.Account{User: httpx.User{Name: cfg.User, Book: own}, Password: pw})
	}

	// The read-only demo dataset.
	if cfg.DemoEnabled {
		if demo.NeedsSeed(cfg.DemoDir()) {
			if err := demo.Seed(cfg.DemoDir()); err != nil {
				return err
			}
			log.Printf("[demo] demo data generated in %s", cfg.DemoDir())
		}
		db, err := book.Open(cfg.DemoDir(), "demo", true)
		if err != nil {
			return err
		}
		go db.RunScheduler(ctx, 30*time.Second)
		go refreshDemo(ctx, cfg.DemoDir(), db)
		pw, hint := cfg.DemoPassword, true
		if cfg.Local() {
			pw, hint = "", false
		}
		accounts = append(accounts, &auth.Account{User: httpx.User{Name: cfg.DemoUser, ReadOnly: true, Book: db}, Password: pw, ShowHint: hint})
	}
	if len(accounts) == 0 {
		return fmt.Errorf("nothing to serve: set ARTHIK_DATA or enable the demo")
	}
	opt := auth.Options{SessionTTL: cfg.Security.SessionTTL}
	var g *guard.Guard
	if !cfg.Local() {
		trusted, err := guard.ParseNets(cfg.Security.TrustedProxies)
		if err != nil {
			return fmt.Errorf("ARTHIK_TRUSTED_PROXIES: %w", err)
		}
		exempt, err := guard.ParseNets(cfg.Security.BanExempt)
		if err != nil {
			return fmt.Errorf("ARTHIK_BAN_EXEMPT: %w", err)
		}
		g = guard.New(guard.Options{Dir: cfg.Security.StateDir, MaxFails: cfg.Security.MaxFails, Term: cfg.Security.BanTerm, Exempt: exempt})
		opt.Guard, opt.DeviceKey = g, cfg.Security.DeviceKey
		opt.ClientIP = func(r *http.Request) string { return guard.ClientIP(r, trusted) }
	}
	a := auth.New(cfg.Secret, opt, accounts...)
	if cfg.Local() {
		a.EnableLocal(cfg.LocalToken)
	} else if cfg.Secret == "" {
		log.Printf("WARN ARTHIK_SECRET is empty — sign-ins (and remembered devices) end at every restart")
	}
	if own != nil {
		if le := own.LoadError(); le != nil {
			log.Printf("WARN %v", le)
		}
	}

	srv := &http.Server{Addr: cfg.Listen, Handler: server.New(a, server.Options{Local: cfg.Local(), Guard: g, ClientIP: opt.ClientIP}),
		ReadTimeout: server.ReadTimeout, WriteTimeout: server.WriteTimeout, IdleTimeout: server.IdleTimeout}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Printf("arthik %s (%s mode, primary=%v) listening on %s — data %q — users: %s", version, cfg.Mode, cfg.Primary, cfg.Listen, cfg.DataDir, a)

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Printf("shutting down")
		sctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(sctx)
	}
	return nil
}

// refreshDemo regenerates the demo data when a new month starts.
func refreshDemo(ctx context.Context, dir string, db *book.Book) {
	t := time.NewTicker(6 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if demo.NeedsSeed(dir) {
				if err := demo.Seed(dir); err != nil {
					log.Printf("[demo] WARN refresh failed: %v", err)
					continue
				}
				_ = db.Reload()
				log.Printf("[demo] demo data refreshed for the new month")
			}
		}
	}
}

func hashPassword() error {
	fmt.Fprint(os.Stderr, "Password: ")
	pw, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && pw == "" {
		return fmt.Errorf("no password given")
	}
	pw = strings.TrimRight(pw, "\r\n")
	if len(pw) < 8 {
		return fmt.Errorf("use at least 8 characters")
	}
	h, err := auth.HashPassword(pw)
	if err != nil {
		return err
	}
	fmt.Println(h)
	fmt.Fprintln(os.Stderr, "Put it in .env in single quotes so $ is kept: ARTHIK_PASSWORD='"+h+"'")
	return nil
}

func dataDir(demoSet bool) (string, error) {
	dir := os.Getenv("ARTHIK_DATA")
	if dir == "" {
		dir = "/data/finance"
	}
	if demoSet {
		return config.Config{DataDir: dir}.DemoDir(), nil
	}
	return dir, nil
}

func check(demoSet bool) error {
	dir, _ := dataDir(demoSet)
	b, err := book.Open(dir, "check", true)
	if err != nil {
		return err
	}
	if le := b.LoadError(); le != nil {
		return le
	}
	problems := b.Problems()
	_ = b.View(func(d *book.Data, c *book.Computed) error {
		fmt.Printf("%s: %d accounts, %d categories, %d labels, %d drafts, %d transactions\n",
			dir, len(d.Accounts), len(d.Categories), len(d.Labels), len(d.Drafts), len(d.Txns))
		fmt.Printf("net worth %s (assets %s, liabilities %s)\n", c.NetWorth.Format(), c.Assets.Format(), c.Liabilities.Format())
		return nil
	})
	if len(problems) == 0 {
		fmt.Println("no problems found")
		return nil
	}
	for _, p := range problems {
		fmt.Printf("  %s %s: %s\n", p.File, p.ID, p.Msg)
	}
	return fmt.Errorf("%d problem(s)", len(problems))
}

func rebuild() error {
	dir, _ := dataDir(false)
	b, err := book.Open(dir, "rebuild", false)
	if err != nil {
		return err
	}
	if le := b.LoadError(); le != nil {
		return le
	}
	fmt.Println("computed values rewritten in", dir)
	return nil
}

func demoSeed() error {
	dir, _ := dataDir(true)
	if err := demo.Seed(dir); err != nil {
		return err
	}
	fmt.Println("demo data generated in", dir)
	return nil
}

func health() error {
	addr := os.Getenv("ARTHIK_LISTEN")
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	c := http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %s", resp.Status)
	}
	return nil
}

// parseSMS shows what the inbox would read from a message (for writing SMS rules).
// printSummary prints the widget/notification summary without changing any file.
func printSummary() error {
	dir, _ := dataDir(false)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return fmt.Errorf("no finance folder at %s", dir)
	}
	b, err := book.Open(dir, "summary", true)
	if err != nil {
		return err
	}
	if le := b.LoadError(); le != nil {
		return le
	}
	var s summary.Summary
	_ = b.View(func(d *book.Data, c *book.Computed) error { s = summary.Build(d, c, 3); return nil })
	return json.NewEncoder(os.Stdout).Encode(s)
}

func parseSMS(args []string) error {
	text := strings.Join(args, " ")
	if text == "" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		text = string(b)
	}
	dir, _ := dataDir(false)
	p, err := inbox.LoadParser(dir)
	if err != nil {
		return err
	}
	r, ok := p.Parse(text)
	if !ok {
		fmt.Println("not a transaction (ignored, or no rule matched)")
		return nil
	}
	fmt.Printf("rule:      %s\nkind:      %s\namount:    %s\ndigits:    %s\nmerchant:  %s\ndate:      %s %s\n", r.Rule, r.Kind, r.Amount, r.Digits, r.Merchant, r.Date, r.Time)
	if r.Balance != 0 {
		fmt.Printf("balance:   %s\n", r.Balance)
	}
	if r.Available != 0 {
		fmt.Printf("available: %s\n", r.Available)
	}
	return nil
}

// stateDir is where the server keeps bans.json (same setting as the server).
func stateDir() (string, error) {
	cfg, err := config.Load()
	if err != nil && cfg.Security.StateDir == "" {
		return "", err
	}
	return cfg.Security.StateDir, nil
}

func listBans() error {
	dir, err := stateDir()
	if err != nil {
		return err
	}
	now := time.Now()
	ips, bans, err := guard.List(dir, now)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		fmt.Println("no banned addresses")
		return nil
	}
	for _, ip := range ips {
		b := bans[ip]
		fmt.Printf("%-40s until %s  (%s, since %s)\n", ip, b.Until.Local().Format("2006-01-02 15:04"), b.Reason, b.Since.Local().Format("2006-01-02 15:04"))
	}
	return nil
}

func unban(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: arthik unban IP|all")
	}
	dir, err := stateDir()
	if err != nil {
		return err
	}
	n, err := guard.Unban(dir, args[0])
	if err != nil {
		return err
	}
	if n == 0 {
		fmt.Println(args[0], "is not banned")
		return nil
	}
	fmt.Printf("lifted %d ban(s)\n", n)
	return nil
}
