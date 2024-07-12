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

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Handles  []string `yaml:"handles"`
	Orgs     []string `yaml:"orgs"`
	Repos    []string `yaml:"repos"`
	Statuses []string `yaml:"statuses"`
}

type PullRequest struct {
	URL    string `json:"url"`
	Title  string `json:"title"`
	Merged bool   `json:"merged"`
}

type Summary struct {
	Handle string
	Counts map[string]int
	PRs    []PullRequest
}

var (
	configFile string
	token      string
	startDate  string
	endDate    string
	duration   string
	enableLog  bool
	showPRs    bool
)

var rootCmd = &cobra.Command{
	Use:   "pullpanda",
	Short: "CLI to measure open-source contributions by fetching pull requests of specified GitHub handles",
	Run: func(cmd *cobra.Command, args []string) {
		config := loadConfig(configFile)
		if enableLog {
			log.Printf("Loaded config: %+v\n", config)
		}
		summaries := fetchAllPRs(config)
		printSummaryTable(summaries, config.Statuses)
		if showPRs {
			printDetailedPRs(summaries)
		}
	},
}

func Execute() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "config.yaml", "config file (default is config.yaml)")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "GitHub personal access token")
	rootCmd.PersistentFlags().StringVar(&startDate, "start-date", "", "Start date in YYYY-MM-DD format")
	rootCmd.PersistentFlags().StringVar(&endDate, "end-date", "", "End date in YYYY-MM-DD format")
	rootCmd.PersistentFlags().StringVar(&duration, "duration", "", "Duration like 1mo, 1w, 1d, 1h, 1m, 1s")
	rootCmd.PersistentFlags().BoolVar(&enableLog, "enable-log", false, "Enable logging")
	rootCmd.PersistentFlags().BoolVar(&showPRs, "show-prs", false, "Show detailed PRs after the summary table")
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

	// Set default statuses if not provided
	if len(config.Statuses) == 0 {
		config.Statuses = []string{"merged"}
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

func fetchAllPRs(config Config) []Summary {
	var wg sync.WaitGroup
	summaries := make([]Summary, len(config.Handles))

	for i, handle := range config.Handles {
		wg.Add(1)
		go func(i int, handle string) {
			defer wg.Done()
			summaries[i] = fetchPRs(handle, config.Orgs, config.Repos, config.Statuses)
		}(i, handle)
	}

	wg.Wait()
	return summaries
}

func fetchPRs(handle string, orgs []string, repos []string, statuses []string) Summary {
	client := &http.Client{}
	summary := Summary{
		Handle: handle,
		Counts: make(map[string]int),
	}

	for _, status := range statuses {
		query := fmt.Sprintf("author:%s is:pr is:%s", handle, status)

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

		if status == "merged" {
			if startDate != "" {
				query += fmt.Sprintf(" merged:>=%s", startDate)
			}
			if endDate != "" {
				query += fmt.Sprintf(" merged:<=%s", endDate)
			}
		} else {
			if startDate != "" {
				query += fmt.Sprintf(" created:>=%s", startDate)
			}
			if endDate != "" {
				query += fmt.Sprintf(" created:<=%s", endDate)
			}
		}

		if len(orgs) > 0 {
			for _, org := range orgs {
				orgQuery := query + fmt.Sprintf(" org:%s", org)
				escapedQuery := url.QueryEscape(orgQuery)
				url := fmt.Sprintf("https://api.github.com/search/issues?q=%s", escapedQuery)

				if enableLog {
					log.Printf("Fetching %s PRs for %s in org %s with query: %s\n", status, handle, org, url)
				}

				prs := makeRequest(client, url)
				summary.Counts[status] += len(prs)
				summary.PRs = append(summary.PRs, prs...)
			}
		} else if len(repos) > 0 {
			for _, repo := range repos {
				repoQuery := query + fmt.Sprintf(" repo:%s", repo)
				escapedQuery := url.QueryEscape(repoQuery)
				url := fmt.Sprintf("https://api.github.com/search/issues?q=%s", escapedQuery)

				if enableLog {
					log.Printf("Fetching %s PRs for %s in repo %s with query: %s\n", status, handle, repo, url)
				}

				prs := makeRequest(client, url)
				summary.Counts[status] += len(prs)
				summary.PRs = append(summary.PRs, prs...)
			}
		} else {
			escapedQuery := url.QueryEscape(query)
			url := fmt.Sprintf("https://api.github.com/search/issues?q=%s", escapedQuery)

			if enableLog {
				log.Printf("Fetching %s PRs for %s with query: %s\n", status, handle, url)
			}

			prs := makeRequest(client, url)
			summary.Counts[status] += len(prs)
			summary.PRs = append(summary.PRs, prs...)
		}
	}

	return summary
}

func makeRequest(client *http.Client, url string) []PullRequest {
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

	return result.Items
}

func printSummaryTable(summaries []Summary, statuses []string) {
	table := tablewriter.NewWriter(os.Stdout)
	header := append([]string{"Handle"}, statuses...)
	header = append(header, "Total")
	table.SetHeader(header)

	var totalCounts = make(map[string]int)

	for _, summary := range summaries {
		row := []string{summary.Handle}
		total := 0
		for _, status := range statuses {
			count := summary.Counts[status]
			row = append(row, strconv.Itoa(count))
			total += count
			totalCounts[status] += count
		}
		row = append(row, strconv.Itoa(total))
		table.Append(row)
	}

	totalRow := []string{"Total"}
	grandTotal := 0
	for _, status := range statuses {
		total := totalCounts[status]
		totalRow = append(totalRow, strconv.Itoa(total))
		grandTotal += total
	}
	totalRow = append(totalRow, strconv.Itoa(grandTotal))
	table.SetFooter(totalRow)
	table.SetFooterAlignment(tablewriter.ALIGN_RIGHT)
	table.SetAutoMergeCellsByColumnIndex([]int{0})

	table.Render()
}

func printDetailedPRs(summaries []Summary) {
	fmt.Println("\nDetailed PRs:")
	for _, summary := range summaries {
		for _, pr := range summary.PRs {
			fmt.Printf("- [%s] %s\n", pr.Title, pr.URL)
		}
	}
}
