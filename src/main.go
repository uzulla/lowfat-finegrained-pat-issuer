// getfgpat (lowfat-finegrained-pat-issuer) helps you create a GitHub
// Fine-grained Personal Access Token (PAT) intended for use with the `gh` CLI.
// It opens GitHub's PAT creation page in your browser with the permissions the
// `gh` command commonly needs (and the token name/description) pre-filled.
//
// Note: GitHub's URL prefill does NOT support pre-selecting repositories
// (it's a client-side React form and no query parameter is wired to the
// repository picker). So this tool embeds the repo list into the token name
// and description, and prints the repos you still need to pick manually.
//
// Requirements:
//   - GitHub CLI (`gh`) is optional. When installed AND authenticated, it is
//     used to verify each repository and fetch its ID. When `gh` is missing or
//     not logged in, validation is skipped (you pick repos on GitHub anyway).
//
// Usage:
//   go run ./src                          # enter repository names interactively
//   go run ./src owner/repo-a owner/repo-b
//   make build && ./build/getfgpat owner/repo-a owner/repo-b
//
//   --no-open   only build and print the URL (don't open the browser)
//   --expires   expiration in days (1-366) or "none". Default: 90
//
// Spec reference:
//   https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens
package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// baseURL is the Fine-grained PAT creation page.
const baseURL = "https://github.com/settings/personal-access-tokens/new"

// permissions are the scopes commonly needed when working with `gh`.
// Values follow GitHub's spec: read / write / admin.
// write implies read; admin implies write.
var permissions = map[string]string{
	"contents":      "write", // read/write repository contents
	"metadata":      "read",  // metadata (mandatory, always read)
	"issues":        "write", // read/write issues
	"pull_requests": "write", // read/write pull requests
	"actions":       "read",  // view GitHub Actions
}

// GitHub limits the token name to 40 characters.
const maxTokenNameLen = 40

func main() {
	openBrowser := true
	expiresIn := "90"
	var repoArgs []string

	// Minimal argument parsing (separate flags from positional args).
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--no-open":
			openBrowser = false
		case a == "--expires":
			if i+1 >= len(args) {
				fatal("--expires requires a value (e.g. --expires 30)")
			}
			i++
			expiresIn = args[i]
		case strings.HasPrefix(a, "--expires="):
			expiresIn = strings.TrimPrefix(a, "--expires=")
		case a == "-h" || a == "--help":
			printUsage()
			return
		case strings.HasPrefix(a, "-"):
			fatal("unknown option: " + a)
		default:
			repoArgs = append(repoArgs, a)
		}
	}

	// `gh` is optional: when available and authenticated, we verify each repo
	// and fetch its internal ID. When it's missing, we skip validation entirely
	// (the final repository selection happens on GitHub anyway).
	validate := ghReady()
	if !validate {
		fmt.Println("note: `gh` not available/authenticated — skipping repository validation.")
	}

	// Get repository names from args or interactively.
	repos := repoArgs
	if len(repos) == 0 {
		repos = promptRepos()
	}
	if len(repos) == 0 {
		fatal("no repositories specified.")
	}

	// Normalize repos, confirm a single owner, and (if gh is available) validate.
	var ids []string
	var cleaned []string // normalized owner/repo list
	var owner string
	for _, full := range repos {
		full = strings.TrimSpace(strings.TrimSuffix(full, "/"))
		if !strings.Contains(full, "/") {
			fatal(fmt.Sprintf("repository must be in owner/repo form: %q", full))
		}
		o := strings.SplitN(full, "/", 2)[0]
		if owner == "" {
			owner = o
		} else if owner != o {
			// A fine-grained PAT can only target repos under a single owner.
			fatal(fmt.Sprintf("mixed owners (%s and %s). Specify repos under the same owner.", owner, o))
		}

		if validate {
			id, err := repoID(full)
			if err != nil {
				fatal(fmt.Sprintf("failed to verify repository: %s\n%v", full, err))
			}
			fmt.Printf("  ✓ %-40s id=%s\n", full, id)
			ids = append(ids, id)
		} else {
			fmt.Printf("  • %s\n", full)
		}
		cleaned = append(cleaned, full)
	}

	date := time.Now().Format("2006-01-02")

	// Build the URL.
	link := buildURL(owner, cleaned, ids, expiresIn, date)
	fmt.Println("\nGenerated URL:")
	fmt.Println(link)

	// GitHub's prefill can't pre-select specific repositories, so guide the manual step.
	fmt.Println("\n⚠ GitHub cannot pre-select repositories via URL.")
	fmt.Println("  Under 'Repository access', choose 'Only select repositories' and add:")
	for _, r := range cleaned {
		fmt.Println("   - " + r)
	}

	if !openBrowser {
		return
	}
	if err := openInBrowser(link); err != nil {
		fmt.Fprintf(os.Stderr, "\nCould not open the browser automatically: %v\nOpen the URL above manually.\n", err)
		os.Exit(1)
	}
	fmt.Println("\nOpened in your browser.")
}

// ghReady reports whether `gh` is installed and authenticated.
func ghReady() bool {
	if _, err := exec.LookPath("gh"); err != nil {
		return false
	}
	return exec.Command("gh", "auth", "status").Run() == nil
}

// repoID fetches the internal repo ID via `gh api repos/<owner>/<repo> --jq '.id'`.
// (`--template '{{.id}}'` renders the number in scientific notation, so we use --jq.)
func repoID(fullName string) (string, error) {
	out, err := exec.Command("gh", "api", "repos/"+fullName, "--jq", ".id").Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return "", fmt.Errorf("empty ID returned")
	}
	return id, nil
}

// buildURL assembles the pre-filled PAT creation URL.
// repos is the normalized owner/repo list, ids are their internal IDs,
// date is the issue date (YYYY-MM-DD).
func buildURL(owner string, repos, ids []string, expiresIn, date string) string {
	q := url.Values{}
	q.Set("name", tokenName(date, repos))
	q.Set("description", tokenDescription(date, repos))
	q.Set("target_name", owner) // target owner (user / organization)
	q.Set("expires_in", expiresIn)

	// Best-effort: GitHub currently ignores this parameter, but include it (when
	// we have IDs from gh) so it works if GitHub ever wires up the repo picker.
	if len(ids) > 0 {
		q.Set("repository_ids", strings.Join(ids, ","))
	}

	// Permissions.
	for perm, level := range permissions {
		q.Set(perm, level)
	}
	return baseURL + "?" + q.Encode()
}

// tokenName builds "<date> <repo names>" within the 40-char limit.
func tokenName(date string, repos []string) string {
	// List repos by short name (the part after "owner/").
	shorts := make([]string, len(repos))
	for i, r := range repos {
		shorts[i] = r[strings.Index(r, "/")+1:]
	}
	name := date + " " + strings.Join(shorts, ",")
	return truncate(name, maxTokenNameLen)
}

// tokenDescription builds a description that states this is a PAT for the gh
// CLI and enumerates the allowed repositories.
func tokenDescription(date string, repos []string) string {
	return fmt.Sprintf("PAT for the gh CLI (generated %s). Repos: %s", date, strings.Join(repos, ", "))
}

// truncate cuts s to max runes (appends … when it overflows).
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// openInBrowser opens the URL in the OS default browser.
func openInBrowser(link string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", link).Start()
	case "windows":
		// Use rundll32 so "&" in the URL isn't treated as a shell separator.
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", link).Start()
	default: // linux, bsd, etc.
		return exec.Command("xdg-open", link).Start()
	}
}

// promptRepos reads owner/repo entries one per line, interactively.
func promptRepos() []string {
	fmt.Println("Enter repositories in owner/repo form (blank line or Ctrl-D to finish):")
	var repos []string
	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			break
		}
		repos = append(repos, line)
	}
	return repos
}

func printUsage() {
	fmt.Print(`getfgpat — create a Fine-grained PAT for the gh CLI (page pre-filled)

Usage:
  getfgpat [options] [owner/repo ...]

Options:
  --expires <days|none>   expiration (1-366) or none. Default: 90
  --no-open               print the URL only, don't open the browser
  -h, --help              show this help

Examples:
  getfgpat owner/repo-a owner/repo-b
  getfgpat --expires 30 --no-open owner/repo
`)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error: "+msg)
	os.Exit(1)
}
