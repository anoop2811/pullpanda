# PullPanda

PullPanda is a CLI tool designed to measure open-source contributions by fetching pull requests of specified GitHub handles. It supports querying across organizations and repositories, summarizing the results in a table format, and optionally displaying detailed information about each pull request.

## Features

- Fetch pull requests for specified GitHub handles.
- Query pull requests across specified organizations and repositories.
- Support multiple PR statuses (`open`, `closed`, `merged`).
- Summarize results in a table format with a total count.
- Optionally display detailed information about each pull request.

## Installation

To install PullPanda, ensure you have Go installed on your machine and run:

```sh
go get github.com/yourusername/pullpanda

## Configuration

PullPanda uses a YAML configuration file to specify the GitHub handles, organizations, repositories, and PR statuses to query. Below is a sample config.yaml file

```yaml
handles:
  - octocat
  - torvalds
  - gaearon
orgs:
  - myorg
repos:
  - myorg/myrepo
  - otherorg/otherrepo
statuses:
  - open
  - closed
  - merged  # Options: "open", "closed", "merged"

## Usage

To run PullPanda, use the following command:

```sh
go run main.go --config=config.yaml --token=your_github_token --duration=7d --enable-log=true --show-prs=true

### Command-Line Flags

  - --config: Path to the configuration file (default is config.yaml).
  - --token: GitHub personal access token (required).
  - --start-date: Start date in YYYY-MM-DD format (optional).
  - --end-date: End date in YYYY-MM-DD format (optional).
  - --duration: Duration like 1mo, 1w, 1d, 1h, 1m, 1s (optional).
  - --enable-log: Enable logging (optional, default is false).
  - --show-prs: Show detailed PRs after the summary table (optional, default is false).

## Build and Run as CLI

### Build the project

```sh
go build -o pullpanda


### Run the executable with the desired flags

```sh
./pullpanda --config=config.yaml --token=your_github_token --duration=7d --enable-log=true --show-prs=true


## Output

The tool will output a summary table with the counts of pull requests for each handle and status, along with a total count. If the --show-prs flag is enabled, it will also display detailed information about each pull request.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
