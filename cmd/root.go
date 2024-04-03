package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.PersistentFlags().StringP("dir", "d", ".", "Set the working directory")
}

func bail(err error) {
	if err == nil { return }
	fmt.Printf("\n\x1b[31;1m%+v\x1b[0m\n", fmt.Sprintf("error: %s", err))
	os.Exit(1)
}

type Change struct {
	SHA      string
	Title    string
	URL      string
}

type Release struct {
	Version string
	Date    string
	Changes []Change
}

const releaseTemplate = `
## [{{ .Version }}] - {{ .Date }}
{{ range .Changes }}
- {{ .Title }} {{ if .URL }}[{{ .SHA }}]({{ .URL }}){{ else }}[{{ .SHA }}]{{ end }}{{ end }}
`

var rootCmd = &cobra.Command{
	Use: "sumit",
	Short: "Generate a changelog from the git history",
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		version := args[0]
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			dir = "."
		}

		repo, err := git.PlainOpen(dir)
		if err != nil {
			bail(errors.Wrap(err, "failed to open git repository"))
		}

		rem, err := repo.Remote("origin")
		var useURL bool
		var remoteURL string
		if err == nil {
			url := rem.Config().URLs[0]
			remoteURL, err = parseRemoteURL(url)
			bail(err)
			useURL = true
		}

		ref, err := repo.Head()
		if err != nil {
			bail(errors.Wrap(err, "failed to get head ref"))
		}

		iter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
		if err != nil {
			bail(errors.Wrap(err, "failed to get commit log"))
		}

		date := time.Now().Format("2006-01-02")

		release := &Release{
			Version: version,
			Date:    date,
		}
		err = iter.ForEach(func(c *object.Commit) error {
			var changeURL string
			hashStr := c.Hash.String()
			if useURL { changeURL = remoteURL + "/commits/" + hashStr }
			change := Change{
				SHA:   hashStr[:7],
				Title: strings.Split(c.Message, "\n")[0],
				URL:   changeURL,
			}
			release.Changes = append(release.Changes, change)
			return nil
		})

		tmpl, _ := template.New("release").Parse(releaseTemplate)
		tmpl.Execute(os.Stdout, release)
	},
}

func parseRemoteURL(url string) (string, error) {
	var baseURL, ws, repoName string

	if strings.HasPrefix(url, "https://") {
		trimURL := strings.TrimPrefix(url, "https://")
		parts := strings.Split(trimURL, "/")
		if len(parts) < 3 {
			return "", errors.New(fmt.Sprintf("invalid remote url structure: %s", url))
		}
		baseURL = "https://" + parts[0]
		ws = parts[1]
		repoName = strings.TrimSuffix(parts[2], ".git")
	} else if strings.HasPrefix(url, "git@") {
		// git@bitbucket.org:username/repo.git
		trimURL := strings.TrimPrefix(url, "git@")
		// bitbucket.org:username/repo.git
		parts := strings.Split(trimURL, ":")
		if len(parts) < 2 {
			return "", errors.New(fmt.Sprintf("invalid remote url structure: %s", url))
		}
		baseURL = "https://" + parts[0]
		repoParts := strings.Split(parts[1], "/")
		if len(repoParts) < 2 {
			return "", errors.New(fmt.Sprintf("invalid remote url structure: %s", url))
		}
		ws = repoParts[0]
		repoName = strings.TrimSuffix(repoParts[1], ".git")
	} else {
		return "", errors.New(fmt.Sprintf("unsupported remote url structure: %s", url))
	}

	repoURL := fmt.Sprintf("%s/%s/%s", baseURL, ws, repoName)
	return repoURL, nil
}

func Execute() {
	bail(rootCmd.Execute())
}
