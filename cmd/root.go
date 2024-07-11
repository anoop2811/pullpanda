package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Handles []string `yaml:"handles"`
	Orgs    []string `yaml:"orgs"`
	Repos   []string `yaml:"repos"`
}

type PullRequest struct {
	URL    string `json:"url"`
	Title  string `json:"title"`
	Merged bool   `json:"merged"`
}

var (
	configFile string
	token      string
	startDate  string
	endDate    string
	duration   string
	enableLog  bool
)

var rootCmd = &cobra.Command{
	Use:   "pullpanda",
	Short: "CLI to measure open-source contributions by fetching merged pull requests of specified GitHub handles",
	Run: func(cmd *cobra.Command, args []string) {
		config := loadConfig(configFile)
		if enableLog {
			log.Printf("Loaded config: %+v\n", config)
		}
		fetchAllMergedPRs(config)
	},
}

func Execute() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "config.yaml", "config file (default is config.yaml)")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "GitHub personal access token")
	rootCmd.PersistentFlags().StringVar(&startDate, "start-date", "", "Start date in YYYY-MM-DD format")
	rootCmd.PersistentFlags().StringVar(&endDate, "end-date", "", "End date in YYYY-MM-DD format")
	rootCmd.PersistentFlags().StringVar(&duration, "duration", "", "Duration like 1mo, 1w, 1d, 1h, 1m, 1s")
	rootCmd.PersistentFlags().BoolVar(&enableLog, "enable-log", false, "Enable logging")
	rootCmd.MarkPersistentFlagRequired("token")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func loadConfig(configFile string) Config {
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(file, &config); err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}

	return config
}

func parseDuration(duration string) (time.Duration, error) {
	if len(duration) < 2 {
		return 0, fmt.Errorf("invalid duration format")
	}

	unit := duration[len(duration)-1:]
	value := duration[:len(duration)-1]

	// Handle multi-character units like "mo" (months)
	if unit == "o" && len(duration) > 2 && duration[len(duration)-2:] == "mo" {
		unit = "mo"
		value = duration[:len(duration)-2]
	}

	numValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}

	switch unit {
	case "s":
		return time.Duration(numValue) * time.Second, nil
	case "m":
		return time.Duration(numValue) * time.Minute, nil
	case "h":
		return time.Duration(numValue) * time.Hour, nil
	case "d":
		return time.Duration(numValue) * 24 * time.Hour, nil
	case "w":
		return time.Duration(numValue) * 7 * 24 * time.Hour, nil
	case "mo":
		return time.Duration(numValue) * 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid duration unit")
	}
}

func fetchAllMergedPRs(config Config) {
	var wg sync.WaitGroup
	wg.Add(len(config.Handles))

	for _, handle := range config.Handles {
		go func(handle string) {
			defer wg.Done()
			fetchMergedPRs(handle, config.Orgs, config.Repos)
		}(handle)
	}

	wg.Wait()
}

func fetchMergedPRs(handle string, orgs []string, repos []string) {
	client := &http.Client{}
	query := fmt.Sprintf("author:%s is:pr is:merged", handle)

	// Calculate startDate if duration is provided
	if duration != "" {
		parsedDuration, err := parseDuration(duration)
		if err != nil {
			log.Fatalf("Error parsing duration: %v", err)
		}
		startTime := time.Now().Add(-parsedDuration)
		startDate = startTime.Format("2006-01-02")
		if enableLog {
			log.Printf("Parsed duration: %s, start date: %s\n", duration, startDate)
		}
	}

	if startDate != "" {
		query += fmt.Sprintf(" merged:>=%s", startDate)
	}
	if endDate != "" {
		query += fmt.Sprintf(" merged:<=%s", endDate)
	}

	if len(orgs) > 0 {
		for _, org := range orgs {
			orgQuery := query + fmt.Sprintf(" org:%s", org)
			escapedQuery := url.QueryEscape(orgQuery)
			url := fmt.Sprintf("https://api.github.com/search/issues?q=%s", escapedQuery)

			if enableLog {
				log.Printf("Fetching merged PRs for %s in org %s with query: %s\n", handle, org, url)
			}

			makeRequest(client, url)
		}
	} else if len(repos) > 0 {
		for _, repo := range repos {
			repoQuery := query + fmt.Sprintf(" repo:%s", repo)
			escapedQuery := url.QueryEscape(repoQuery)
			url := fmt.Sprintf("https://api.github.com/search/issues?q=%s", escapedQuery)

			if enableLog {
				log.Printf("Fetching merged PRs for %s in repo %s with query: %s\n", handle, repo, url)
			}

			makeRequest(client, url)
		}
	} else {
		escapedQuery := url.QueryEscape(query)
		url := fmt.Sprintf("https://api.github.com/search/issues?q=%s", escapedQuery)

		if enableLog {
			log.Printf("Fetching merged PRs for %s with query: %s\n", handle, url)
		}

		makeRequest(client, url)
	}
}

func makeRequest(client *http.Client, url string) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Error: received non-200 response code %d", resp.StatusCode)
	}

	var result struct {
		Items []PullRequest `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatalf("Error decoding response: %v", err)
	}

	for _, pr := range result.Items {
		fmt.Printf("- %s: %s\n", pr.Title, pr.URL)
	}
}
