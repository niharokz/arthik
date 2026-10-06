// Command arthik is a double-entry personal finance app (a Bluecoins-style PWA) that
// keeps its data as YAML .md files.
//
//	arthik serve            run the web app (default)
//	arthik hash-password    print a bcrypt hash for ARTHIK_PASSWORD
//	arthik check [--demo]   validate the data files, list problems (exit 1 if any)
//	arthik rebuild          re-read everything and rewrite all computed values
//	arthik demo-seed        regenerate the demo dataset now
//	arthik health           exit 0 if the running server answers (Docker healthcheck)
//	arthik version
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // zone data compiled in, so TZ works on minimal images

	"gitlab.com/niharokz/arthik/internal/auth"
	"gitlab.com/niharokz/arthik/internal/book"
	"gitlab.com/niharokz/arthik/internal/config"
	"gitlab.com/niharokz/arthik/internal/demo"
	"gitlab.com/niharokz/arthik/internal/feature/state"
	"gitlab.com/niharokz/arthik/internal/httpx"
	"gitlab.com/niharokz/arthik/internal/server"
)

// version is set at build time: -ldflags "-X main.version=1.0.0".
var version = "1.0.0"

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

	own, err := book.Open(cfg.DataDir, "main", false)
	if err != nil {
		return err
	}
	go own.Watch(ctx, cfg.WatchEvery)
	go own.RunScheduler(ctx, 30*time.Second)

	accounts := []*auth.Account{{User: httpx.User{Name: cfg.User, Book: own}, Password: cfg.Password}}
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
		accounts = append(accounts, &auth.Account{User: httpx.User{Name: cfg.DemoUser, ReadOnly: true, Book: db}, Password: cfg.DemoPassword, ShowHint: true})
	}
	a := auth.New(cfg.Secret, accounts...)
	if cfg.Secret == "" {
		log.Printf("WARN ARTHIK_SECRET is empty — sign-ins will not survive a restart")
	}
	if le := own.LoadError(); le != nil {
		log.Printf("WARN %v", le)
	}

	srv := &http.Server{Addr: cfg.Listen, Handler: server.New(a),
		ReadTimeout: server.ReadTimeout, WriteTimeout: server.WriteTimeout, IdleTimeout: server.IdleTimeout}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Printf("arthik %s listening on %s — data %s — users: %s", version, cfg.Listen, cfg.DataDir, a)

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
		fmt.Printf("%s: %d accounts, %d categories, %d labels, %d reminders, %d transactions\n",
			dir, len(d.Accounts), len(d.Categories), len(d.Labels), len(d.Reminders), len(d.Txns))
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
