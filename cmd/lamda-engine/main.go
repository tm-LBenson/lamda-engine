package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tm-LBenson/lamda-engine/internal/engine"
)

var version = "0.1.0"

type install struct {
	Retail string `json:"retail"`
	Config string `json:"config"`
}
type state struct {
	PID    int           `json:"pid"`
	Config engine.Config `json:"config"`
	Rows   []engine.Row  `json:"rows"`
	Update string        `json:"update,omitempty"`
}

func main() {
	retail := flag.String("retail", "", "WoW _retail_ directory")
	saved := flag.String("config", "", "lamdaUI.lua SavedVariables path")
	replay := flag.String("replay", "", "Replay a combat log to stdout without starting engine")
	noOverlay := flag.Bool("no-overlay", false, "Disable Windows overlay")
	ver := flag.Bool("version", false, "Show version")
	flag.Parse()
	if *ver {
		fmt.Println(version)
		return
	}
	if *replay != "" {
		if e := runReplay(*replay); e != nil {
			log.Fatal(e)
		}
		return
	}
	exe, e := os.Executable()
	if e != nil {
		log.Fatal(e)
	}
	base := filepath.Dir(exe)
	home, e := os.UserConfigDir()
	if e != nil {
		log.Fatal(e)
	}
	dir := filepath.Join(home, "LamdaUI")
	if e = os.MkdirAll(dir, 0700); e != nil {
		log.Fatal(e)
	}
	logfile, e := os.OpenFile(filepath.Join(dir, "engine.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		log.Fatal(e)
	}
	defer logfile.Close()
	log.SetOutput(logfile)
	// A loopback bind is used only as a singleton lock. No service or commands exposed.
	lock, e := net.Listen("tcp4", "127.0.0.1:47639")
	if e != nil {
		log.Print("Engine already running or singleton port unavailable")
		return
	}
	defer lock.Close()
	var ins install
	if b, e := os.ReadFile(filepath.Join(dir, "install.json")); e == nil {
		_ = json.Unmarshal([]byte(strings.TrimPrefix(string(b), "\ufeff")), &ins)
	}
	if *retail == "" {
		*retail = ins.Retail
	}
	if *saved == "" {
		*saved = ins.Config
	}
	if *retail == "" {
		log.Fatal("WoW path missing; rerun installer with -WowPath")
	}
	statePath := filepath.Join(dir, "state.json")
	defer os.Remove(statePath)
	cfg := engine.DefaultConfig()
	model := engine.NewModel()
	tail := &engine.Tail{}
	var overlay *exec.Cmd
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if runtime.GOOS == "windows" && !*noOverlay {
		overlay = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-STA", "-File", filepath.Join(base, "overlay.ps1"), "-StatePath", statePath, "-EnginePID", fmt.Sprint(os.Getpid()))
		hideWindow(overlay)
		overlay.Stdout = logfile
		overlay.Stderr = logfile
		if e := overlay.Start(); e != nil {
			log.Printf("Overlay: %v", e)
		} else {
			defer overlay.Process.Kill()
			go func() {
				if e := overlay.Wait(); e != nil {
					log.Printf("Overlay exit: %v", e)
				}
			}()
		}
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	tick := 0
	nextUpdate := time.Time{}
	update := ""
	updateResults := make(chan string, 1)
	checking := false
	log.Printf("Lamda Engine %s started", version)
	for {
		select {
		case <-ctx.Done():
			return
		case u := <-updateResults:
			checking = false
			update = u
		case now := <-ticker.C:
			tick++
			if tick%8 == 1 {
				p := *saved
				if p == "" {
					matches, _ := filepath.Glob(filepath.Join(*retail, "WTF", "Account", "*", "SavedVariables", "lamdaUI.lua"))
					if len(matches) == 1 {
						p = matches[0]
					}
				}
				if p != "" {
					if c, e := engine.ReadConfig(p); e == nil {
						if cfg.Enabled != c.Enabled {
							model = engine.NewModel()
						}
						cfg = c
					}
				}
			}
			if cfg.Updates && !checking && now.After(nextUpdate) {
				checking = true
				nextUpdate = now.Add(time.Duration(cfg.Days) * 24 * time.Hour)
				go func() { updateResults <- checkUpdate(ctx) }()
			}
			oldPath := tail.Path
			lines, e := tail.Poll(filepath.Join(*retail, "Logs"))
			if e != nil && tick%40 == 1 {
				log.Printf("Log read: %v", e)
			}
			if oldPath != "" && oldPath != tail.Path {
				model = engine.NewModel()
			}
			for _, line := range lines {
				if strings.Contains(line, "  ZONE_CHANGE,") || strings.Contains(line, "  CHALLENGE_MODE_START,") || strings.Contains(line, "  CHALLENGE_MODE_END,") {
					model = engine.NewModel()
					continue
				}
				if !cfg.Enabled {
					continue
				}
				c, e := engine.Parse(line)
				if e != nil {
					continue
				}
				if model.Observe(c, now) {
					log.Printf("Observed spell=%d delivery=%.3fs", c.Spell, now.Sub(c.At).Seconds())
				}
			}
			shownUpdate := ""
			if cfg.Updates && cfg.Notify {
				shownUpdate = update
			}
			if e := engine.WriteJSON(statePath, state{os.Getpid(), cfg, model.Active(now), shownUpdate}); e != nil && tick%40 == 1 {
				log.Printf("State write: %v", e)
			}
		}
	}
}
func checkUpdate(ctx context.Context) string {
	if tag := checkRelease(ctx); tag != "" {
		return tag
	}
	// Private repositories can use Git's existing credentials without a stored token.
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-c", "credential.interactive=false", "ls-remote", "--tags", "https://github.com/tm-LBenson/lamda-engine.git")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	hideWindow(cmd)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return newestTag(string(output), version)
}
func newestTag(output, current string) string {
	best := current
	found := ""
	for _, line := range strings.Split(output, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		tag := strings.TrimPrefix(f[1], "refs/tags/")
		if newer(tag, best) {
			best = tag
			found = tag
		}
	}
	return found
}
func newer(tag, current string) bool {
	parse := func(v string) ([3]int, bool) {
		var n [3]int
		parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
		if len(parts) != 3 {
			return n, false
		}
		for i, p := range parts {
			if p == "" {
				return n, false
			}
			for _, c := range p {
				if c < '0' || c > '9' {
					return n, false
				}
			}
			value, e := strconv.Atoi(p)
			if e != nil {
				return n, false
			}
			n[i] = value
		}
		return n, true
	}
	a, ok := parse(tag)
	b, bok := parse(current)
	if !ok || !bok {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
func checkRelease(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/tm-LBenson/lamda-engine/releases/latest", nil)
	req.Header.Set("User-Agent", "LamdaEngine/"+version)
	r, e := http.DefaultClient.Do(req)
	if e != nil {
		return ""
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return ""
	}
	var release struct {
		Tag   string `json:"tag_name"`
		Draft bool   `json:"draft"`
		Pre   bool   `json:"prerelease"`
	}
	if json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(&release) != nil {
		return ""
	}
	if !release.Draft && !release.Pre && newer(release.Tag, version) {
		return release.Tag
	}
	return ""
}
func runReplay(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 65536), 1<<20)
	enc := json.NewEncoder(os.Stdout)
	count := 0
	for scan.Scan() {
		c, e := engine.Parse(scan.Text())
		if e != nil {
			continue
		}
		s, ok := engine.Catalog[c.Spell]
		if !ok {
			continue
		}
		count++
		enc.Encode(map[string]any{"at": c.At, "player": c.Name, "spell": c.Spell, "name": s.Name, "estimatedCooldown": s.Cooldown})
	}
	fmt.Fprintf(os.Stderr, "Matched %d teammate defensive casts\n", count)
	return scan.Err()
}
