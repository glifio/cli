package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

// GitHubRelease represents the structure of the release object from GitHub's API
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	PreRelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

// GitHubTag represents the structure of the tag object, which includes the commit hash
type GitHubTag struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

func getLatestRelease() (*GitHubRelease, error) {
	resp, err := http.Get("https://api.github.com/repos/glifio/cli/releases/latest")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}

	return &release, nil
}

func getCommitHashFromTag(tagName string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/glifio/cli/git/refs/tags/%s", tagName)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tag GitHubTag
	if err := json.Unmarshal(body, &tag); err != nil {
		return "", err
	}

	return tag.Object.SHA, nil
}

func getLatestTag() (string, string, bool, error) {
	release, err := getLatestRelease()
	if err != nil {
		return "", "", false, err
	}

	commitHash, err := getCommitHashFromTag(release.TagName)
	if err != nil {
		return "", "", false, err
	}

	return release.TagName, commitHash, release.Draft || release.PreRelease, nil
}

var rootInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Prints information about the CLI",
	Long:  `Prints information about the CLI`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Config directory: %s\n", cfgDir)
		fmt.Printf("Chain ID: %d\n", chainID)
		fmt.Printf("Commit hash: %s\n", CommitHash)
		tagName, release, stableVersion, err := getLatestTag()
		if err != nil {
			logFatal(err)
		}
		fmt.Printf("Tag: %s\n", tagName)

		fmt.Printf("Latest release: %s (prelease / draft release): %t\n", release, stableVersion)

		if stableVersion && release != CommitHash {
			fmt.Println("There may be a new version of the CLI available at https://github.com/glifio/glif/v2")
		}
	},
}

func init() {
	rootCmd.AddCommand(rootInfoCmd)
}
