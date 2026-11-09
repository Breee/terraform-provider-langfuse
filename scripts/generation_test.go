package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGenerationWorkflow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("generation requires a POSIX shell")
	}
	makefile, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		target     string
		fail       string
		wantEvents string
		wantError  bool
	}{
		{"refresh everything", "generate", "", "public\nadmin\nmocks\napi\ndocs\n", false},
		{"local without downloads", "generate-local", "", "mocks\napi\ndocs\n", false},
		{"first download fails", "generate", "public", "public\n", true},
		{"second download fails", "generate", "admin", "public\nadmin\n", true},
		{"empty download fails", "generate", "empty", "public\nadmin\n", true},
		{"generator fails", "generate", "api", "public\nadmin\nmocks\napi\n", true},
		{"drift check is local", "check-generated", "", "mocks\napi\ndocs\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, content string, mode os.FileMode) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), mode); err != nil {
					t.Fatal(err)
				}
			}
			write("Makefile", string(makefile), 0o644)
			write("openapi/langfuse.yml", "original public", 0o644)
			write("openapi/langfuse-organizations.yml", "original admin", 0o644)
			write("tools/.keep", "", 0o644)
			write("langfuse/mocks/generated.go", "generated\n", 0o644)
			write("bin/curl", `#!/bin/sh
set -eu
case "$*" in *organizations-api*) stage=admin ;; *) stage=public ;; esac
echo "$stage" >> "$EVENTS"
if [ "$FAIL_STAGE" = "$stage" ]; then echo "download failed" >&2; exit 22; fi
while [ "$1" != "-o" ]; do shift; done
if [ "$FAIL_STAGE" = empty ]; then : > "$2"; else echo "$stage" > "$2"; fi
`, 0o755)
			write("bin/go", `#!/bin/sh
set -eu
case "$*" in *-skip*) stage=docs ;; *-run*) stage=api ;; *) stage=mocks ;; esac
echo "$stage" >> "$EVENTS"
if [ "$FAIL_STAGE" = "$stage" ]; then echo "generator failed" >&2; exit 1; fi
if [ "$stage" = mocks ]; then echo generated > langfuse/mocks/generated.go; fi
`, 0o755)
			env := append(os.Environ(),
				"PATH="+filepath.Join(dir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
				"EVENTS="+filepath.Join(dir, "events"), "FAIL_STAGE="+tc.fail,
			)
			run := func(name string, args ...string) ([]byte, error) {
				t.Helper()
				cmd := exec.Command(name, args...)
				cmd.Dir = dir
				cmd.Env = env
				return cmd.CombinedOutput()
			}
			for _, args := range [][]string{{"init", "-q"}, {"add", "langfuse"}} {
				if out, err := run("git", args...); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
			}
			out, err := run("make", "-j4", tc.target)
			if (err != nil) != tc.wantError {
				t.Fatalf("make %s: %v\n%s", tc.target, err, out)
			}
			events, err := os.ReadFile(filepath.Join(dir, "events"))
			if err != nil || string(events) != tc.wantEvents {
				t.Fatalf("events = %q, err = %v; want %q", events, err, tc.wantEvents)
			}
			for name, original := range map[string]string{
				"langfuse.yml":               "original public",
				"langfuse-organizations.yml": "original admin",
			} {
				data, err := os.ReadFile(filepath.Join(dir, "openapi", name))
				if err != nil {
					t.Fatal(err)
				}
				shouldRefresh := tc.target == "generate" && (tc.fail == "" || tc.fail == "api")
				if (string(data) != original) != shouldRefresh {
					t.Fatalf("%s unexpectedly changed or failed to refresh: %q", name, data)
				}
			}
			leftovers, err := filepath.Glob(filepath.Join(dir, "openapi", ".download.*"))
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("download temporary files remain: %v, %v", leftovers, err)
			}
			if tc.target == "check-generated" {
				if err := os.WriteFile(filepath.Join(dir, "langfuse/mocks/generated.go"), []byte("stale\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if out, err := run("git", "add", "langfuse"); err != nil {
					t.Fatalf("git add: %v\n%s", err, out)
				}
				if out, err := run("make", "check-generated"); err == nil || !strings.Contains(string(out), "stale") {
					t.Fatalf("stale output was not rejected: %v\n%s", err, out)
				}
				if out, err := run("git", "add", "langfuse"); err != nil {
					t.Fatalf("git add: %v\n%s", err, out)
				}
				write("docs/new.md", "untracked output\n", 0o644)
				if out, err := run("make", "check-generated"); err == nil || !strings.Contains(string(out), "untracked") {
					t.Fatalf("untracked output was not rejected: %v\n%s", err, out)
				}
			}
		})
	}
}
