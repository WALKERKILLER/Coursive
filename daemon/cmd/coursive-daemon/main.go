// daemon is the Coursive daemon: source plugins for course platforms, a
// high-throughput downloader, transcription and the library index.
// The Flutter app starts it and talks to it over a token-protected local API.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/agent"
	"github.com/walkerkiller/coursive/daemon/internal/agentenv"
	"github.com/walkerkiller/coursive/daemon/internal/asr"
	"github.com/walkerkiller/coursive/daemon/internal/datasync"
	"github.com/walkerkiller/coursive/daemon/internal/dl"
	"github.com/walkerkiller/coursive/daemon/internal/events"
	"github.com/walkerkiller/coursive/daemon/internal/jobs"
	"github.com/walkerkiller/coursive/daemon/internal/library"
	"github.com/walkerkiller/coursive/daemon/internal/lms"
	"github.com/walkerkiller/coursive/daemon/internal/localasr"
	"github.com/walkerkiller/coursive/daemon/internal/mineru"
	"github.com/walkerkiller/coursive/daemon/internal/notify"
	"github.com/walkerkiller/coursive/daemon/internal/notion"
	"github.com/walkerkiller/coursive/daemon/internal/pluginbridge"
	"github.com/walkerkiller/coursive/daemon/internal/rootfs"
	"github.com/walkerkiller/coursive/daemon/internal/server"
	"github.com/walkerkiller/coursive/daemon/internal/sources"
	"github.com/walkerkiller/coursive/daemon/internal/store"
	"github.com/walkerkiller/coursive/daemon/internal/vault"
)

func defaultDataDir() string {
	name := "coursive"
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, name)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", name)
}

func defaultLibrary() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Coursive")
}

// unpack is how the Android app installs its Linux workspace:
// daemon unpack <archive.tar.gz> <dir> [sha256]. It prints "PROGRESS <bytes>"
// lines and runs without the guest, straight from the app's library folder.
func unpack(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: daemon unpack <archive.tar.gz> <dir> [sha256]")
		os.Exit(2)
	}
	f, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	want := ""
	if len(args) > 2 {
		want = args[2]
	}
	var last int64
	err = rootfs.Unpack(f, args[1], want, func(n int64) {
		if n-last >= 1<<20 {
			last = n
			fmt.Printf("PROGRESS %d\n", n)
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("DONE")
}

// prune is how release packaging slims the bundled agent:
// daemon prune <node_modules>... deletes declarations and source maps
// with the same rule as the in-app runtime setup.
func prune(dirs []string) {
	var freed int64
	for _, d := range dirs {
		freed += agentenv.PruneModules(d)
	}
	fmt.Printf("freed %d MB\n", freed>>20)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "package-info" {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"data": defaultDataDir(), "library": defaultLibrary()})
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "unpack" {
		unpack(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "prune" {
		prune(os.Args[2:])
		return
	}
	dataDir := flag.String("data", defaultDataDir(), "private state (source plugins, keys, agent config)")
	libRoot := flag.String("library", "", "library root (courses, notes, library.db)")
	addr := flag.String("addr", "127.0.0.1:0", "listen address")
	node := flag.String("node", "", "node binary for the agent host")
	agentScript := flag.String("agent", "", "agent host entry script")
	watchStdin := flag.Bool("exit-on-stdin-eof", false, "exit when the parent closes stdin")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(*dataDir, *libRoot, *addr, *node, *agentScript, *watchStdin); err != nil {
		slog.Error("daemon failed", "err", err)
		os.Exit(1)
	}
}

type config struct {
	LibraryRoot string `json:"library_root"`
}

func run(dataDir, libRoot, addr, node, agentScript string, watchStdin bool) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	cfgPath := filepath.Join(dataDir, "config.json")
	var cfg config
	if b, err := os.ReadFile(cfgPath); err == nil {
		json.Unmarshal(b, &cfg)
	}
	if libRoot != "" {
		cfg.LibraryRoot = libRoot
	}
	if cfg.LibraryRoot == "" {
		cfg.LibraryRoot = defaultLibrary()
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(cfgPath, b, 0o600)

	token := os.Getenv("COURSIVE_TOKEN")
	if token == "" {
		var t [24]byte
		rand.Read(t[:])
		token = hex.EncodeToString(t[:])
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if watchStdin {
		go func() {
			io.Copy(io.Discard, os.Stdin)
			slog.Info("parent closed stdin, shutting down")
			stop()
		}()
	}

	if err := os.MkdirAll(cfg.LibraryRoot, 0o755); err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(cfg.LibraryRoot, "library.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	lib, err := library.Open(cfg.LibraryRoot, st)
	if err != nil {
		return err
	}

	bus := events.New()
	vault.KeyPath = filepath.Join(dataDir, "seal.key")
	// Downloads and API clients honour the system's proxy environment.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	eng := dl.New(transport, dl.Options{MaxConns: 8})
	sources.SetDaemonVersion(server.Version)
	reg := &sources.Registry{Dir: filepath.Join(dataDir, "plugins", "sources"), DataDir: filepath.Join(dataDir, "sources"),
		Publish: bus.Publish}
	if err := reg.Load(); err != nil {
		slog.Warn("source plugins", "err", err)
	}
	defer reg.Stop()
	// The market reads the signed official index; COURSIVE_PLUGIN_INDEX names
	// a local, unsigned index.json instead (docs/specs/0025-plugin-marketplace.md).
	market := &sources.Market{Index: os.Getenv("COURSIVE_PLUGIN_INDEX"), IndexURL: sources.OfficialIndex, Keys: sources.OfficialKeys,
		Cache: filepath.Join(dataDir, "plugins", ".index"), Staging: filepath.Join(dataDir, "plugins", ".staging"),
		Registry: reg, HTTP: &http.Client{Transport: transport, Timeout: 5 * time.Minute}}
	os.RemoveAll(market.Staging)
	mgr := jobs.New(st, lib, nil, eng, asr.NewPool(filepath.Join(dataDir, "asr-usage.json")), bus)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	base := fmt.Sprintf("http://127.0.0.1:%d", port)

	rt := &agentenv.Manager{Dir: filepath.Join(dataDir, "runtime"), FlagNode: node,
		Publish: func(in agentenv.Install) { bus.Publish("runtime", in) }}
	if agentScript != "" {
		rt.AgentDir = filepath.Dir(filepath.Dir(agentScript)) // agent/src/host.mjs → agent
	}
	node, _ = rt.Node() // -node, else the Node.js an in-app install left
	sup := &agent.Supervisor{Node: node, Script: agentScript, Token: token, Env: []string{
		"COURSIVE_URL=" + base,
		"AGENT_DATA_DIR=" + filepath.Join(dataDir, "agent"), "LIBRARY_ROOT=" + cfg.LibraryRoot,
	}}
	mgr.MinerU = mineru.NewService(filepath.Join(dataDir, "mineru.json"), vault.Seal, vault.Unseal)
	rt.Restart = sup.Restart
	local := &localasr.Manager{Dir: filepath.Join(dataDir, "asr")}
	srv := &server.Server{Token: token, Sources: reg, Market: market, St: st, Lib: lib, Jobs: mgr, Eng: eng, Bus: bus, Agent: sup, Runtime: rt, Ctx: ctx, MinerU: mgr.MinerU, Local: local}
	local.Publish = srv.OnLocalASR
	// The assistant host is the plugin registry and executor until the Plugin
	// Fabric store lands; no approval source is wired, so a write is refused.
	hostPlugins := &agent.Plugins{Agent: sup}
	srv.PluginActions = &pluginbridge.Bridge{Registry: hostPlugins, Executor: hostPlugins}
	srv.Notes = notify.Open(filepath.Join(dataDir, "notifications.json"), func(it notify.Item) { bus.Publish("notification", it) })
	srv.ApplySettings(ctx)
	mgr.Source = srv.Source
	srv.LMS = &lms.Service{Path: filepath.Join(dataDir, "lms.json"), Sources: srv.LMSSources, Notify: srv.Notes.Add,
		Publish: bus.Publish, Fetch: func(ctx context.Context, a sources.Address, dest string) error {
			return eng.Fetch(ctx, dl.Request{URL: a.URL, Header: a.Header(), Dest: dest}, nil)
		}}

	go reg.Refresh(ctx)
	go mgr.Run(ctx)
	srv.Notion = &notion.Service{Path: filepath.Join(dataDir, "notion.json"), Docs: srv.NotionDocs, Publish: bus.Publish,
		HC: &http.Client{Timeout: 2 * time.Minute, Transport: transport}}
	srv.Data = &datasync.Service{Roots: datasync.Roots{Library: cfg.LibraryRoot, Data: dataDir}, App: server.Version,
		State: srv.UserState, MergeState: srv.MergeUserState, ApplyFiles: lib.ApplySyncFiles, CourseState: srv.CourseState, MergeCourseState: srv.MergeCourseState, Changed: srv.DataArrived, Publish: bus.Publish,
		Seal: vault.Seal, Unseal: vault.Unseal}
	go srv.Data.Run(ctx)
	go srv.RunWatcher(ctx)
	go sup.Run(ctx)
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		last := sup.Status()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if cur := sup.Status(); cur != last {
					last = cur
					bus.Publish("agent", cur)
				}
			}
		}
	}()

	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		hs.Shutdown(sctx)
	}()
	ready, _ := json.Marshal(map[string]any{"port": port, "token": token, "library": cfg.LibraryRoot, "version": server.Version})
	fmt.Printf("COURSIVE_READY %s\n", ready)
	os.Stdout.Sync()
	slog.Info("daemon listening", "addr", ln.Addr().String(), "library", cfg.LibraryRoot)
	if err := hs.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
