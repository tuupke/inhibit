package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/creack/pty"
	"github.com/foresthoffman/reap"
)

func env(key, fb string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}

	return fb
}

var (
	files = env("FILES", "/tmp/inhibit/")
	// inhibit is a comma separated list of inhibitors to use.
	inhibit = env("INHIBIT", "suspend")
	log     zerolog.Logger
)

func main() {
	// Create dir
	if err := os.MkdirAll(files, 0755); err != nil {
		panic(err)
	}

	if len(os.Args) < 3 {
		os.Exit(1)
	}

	cmd, pid := os.Args[1], os.Args[2]

	// Create logfile and logger.
	logfile, err := os.OpenFile(files+"inhibit.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}

	l, err := zerolog.ParseLevel(env("LOG_LEVEL", "info"))
	if err != nil {
		l = zerolog.DebugLevel
	}

	log = zerolog.New(logfile).With().Str("command", cmd).Int("own_pid", os.Getpid()).Str("pid", pid).Timestamp().Logger().Level(l)
	defer logfile.Close()

	switch cmd {
	case "login":
		log.Err(startInhibit(pid)).Msg("handled")
	case "logout":
		log.Err(stopInhibit(pid)).Msg("handled")
	case "forkoff":
		// Use cat instead of --inhibit-only to not have to send ctrl+c signal to the process.
		c := exec.Command("gnome-session-inhibit", "--inhibit", inhibit, "--app-id", "test.test", "--reason", "Suspend disabled because of active ssh session(s).", "cat")
		f, err := pty.Start(c)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to start pty")
		}

		// Ignore errors, exiting anyway.
		_, _ = io.Copy(logfile, f)
		_ = f.Close()
	}
}

func pidFileCheck(name string) (string, bool) {
	f, err := os.OpenFile(files+name+".pid", os.O_RDONLY, 0644)
	if err != nil {
		return "", false
	}

	defer f.Close()

	pid, err := io.ReadAll(f)
	if err != nil {
		return "", false
	}

	return string(pid), pidCheck(string(pid))
}

func makePidFile(name, pid string) error {
	f, err := os.OpenFile(files+name+".pid", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = fmt.Fprintf(f, "%s", pid)
	return err
}

func pidCheck(pid string) bool {
	out, err := exec.Command("kill", "-s", "0", pid).CombinedOutput()
	return err == nil && string(out) == ""
}

func startInhibit(pid string) error {
	if _, running := pidFileCheck(pid); running {
		log.Warn().Msg("pid already exists")
		return nil
	}

	log.Info().Msg("creating pid file")

	if _, running := pidFileCheck("inhibit"); !running {
		// Not running, start and write pid
		cpid, err := reap.Exec(os.Args[0], "forkoff", "1337")
		log.Err(err).Msg("started forkoff")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to start forkoff")
		}

		if err = makePidFile("inhibit", strconv.Itoa(cpid)); err != nil {
			return err
		}
	} else {
		log.Debug().Msg("forkoff already running, skipping")
	}

	err := makePidFile(pid, pid)
	log.Err(err).Msg("wrote pid")
	return err
}

func stopInhibit(pid string) error {
	matches, err := filepath.Glob(files + "*.pid")
	if err != nil {
		return err
	}

	var running int
	for _, match := range matches {
		match = match[len(files):]
		match = match[:len(match)-len(".pid")]

		log.Debug().Str("match", match).Msg("checking pid match")
		if match == "inhibit" {
			continue
		}

		// If the match is not running, remove it
		if running := pidCheck(match); !running || match == pid {
			log.Warn().Str("match", match).Bool("running", running).Msg("pid not running")
			_ = os.Remove(files + match + ".pid")
			continue
		}

		running++
	}

	if running > 0 {
		log.Info().Int("running", running).Msg("still running, not stopping")
		return nil
	}

	pid, inhibitRunning := pidFileCheck("inhibit")
	if !inhibitRunning {
		log.Warn().Msg("forkoff already dead")
		return nil
	}

	err = exec.Command("kill", "-9", pid).Run()
	log.Err(err).Msg("killed forkoff")

	return err
}
