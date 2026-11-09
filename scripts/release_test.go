package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWeeklyReleaseTags(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release workflow uses Bash")
	}
	workflow, err := os.ReadFile("../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, step, ok := strings.Cut(string(workflow), "      - name: Create weekly patch tag\n")
	if !ok {
		t.Fatal("weekly tag step missing")
	}
	_, script, ok := strings.Cut(step, "        run: |\n")
	if !ok {
		t.Fatal("weekly tag script missing")
	}
	script, _, _ = strings.Cut(script, "\n      - name:")
	script = strings.ReplaceAll(script, "          ", "")
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, tag, extraTag, want string
		changed                   bool
	}{
		{"first release", "", "", "v0.1.0", false},
		{"patch release", "v1.2.3", "", "v1.2.4", true},
		{"no changes", "v1.2.3", "", "", false},
		{"ignore prerelease", "v1.2.3", "v2.0.0-rc.1", "v1.2.4", true},
		{"no stable tags", "", "v2.0.0-rc.1", "v0.1.0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			run := func(args ...string) {
				t.Helper()
				cmd := exec.Command(git, args...)
				cmd.Dir = dir
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
			}
			run("init", "-q")
			run("config", "user.name", "Release Test")
			run("config", "user.email", "release@example.invalid")
			run("commit", "--allow-empty", "-qm", "initial")
			for _, tag := range []string{tc.tag, tc.extraTag} {
				if tag != "" {
					run("tag", tag)
				}
			}
			if tc.changed {
				run("commit", "--allow-empty", "-qm", "change")
			}
			bin := filepath.Join(dir, "bin")
			if err := os.Mkdir(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			wrapper := "#!/bin/sh\nif [ \"$1\" = push ]; then echo \"$3\" >> \"$PUSHED\"; exit 0; fi\nexec \"$REAL_GIT\" \"$@\"\n"
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "output")
			pushed := filepath.Join(dir, "pushed")
			if err := os.WriteFile(output, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(pushed, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", "-euo", "pipefail", "-c", script)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "REAL_GIT="+git, "GITHUB_OUTPUT="+output, "PUSHED="+pushed)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("weekly release: %v\n%s", err, out)
			}
			for name, want := range map[string]string{output: "tag=" + tc.want + "\n", pushed: tc.want + "\n"} {
				if tc.want == "" {
					want = ""
				}
				got, err := os.ReadFile(name)
				if err != nil || string(got) != want {
					t.Errorf("%s = %q, %v; want %q", filepath.Base(name), got, err, want)
				}
			}
		})
	}
}
