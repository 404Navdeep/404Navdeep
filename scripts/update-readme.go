package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type GitHubEvent struct {
	Type      string       `json:"type"`
	Repo      GitHubRepo   `json:"repo"`
	Payload   EventPayload `json:"payload"`
	CreatedAt string       `json:"created_at"`
}

type GitHubRepo struct {
	Name string `json:"name"`
}

type EventPayload struct {
	Head string `json:"head"`
}

type HackatimeStats struct {
	Data UserStats `json:"data"`
}

type UserStats struct {
	TotalSeconds              int64          `json:"total_seconds"`
	DailyAverage              int64          `json:"daily_average"`
	HumanReadableTotal        string         `json:"human_readable_total"`
	HumanReadableDailyAverage string         `json:"human_readable_daily_average"`
	Languages                 []LanguageStat `json:"languages"`
}

type LanguageStat struct {
	Name         string  `json:"name"`
	TotalSeconds int64   `json:"total_seconds"`
	Text         string  `json:"text"`
	Hours        int64   `json:"hours"`
	Minutes      int64   `json:"minutes"`
	Percent      float64 `json:"percent"`
	Digital      string  `json:"digital"`
}

type WakatimeParams struct {
	start_date string
	end_date   string
}

type ReadmeParams struct {
	LastRepo      string
	LastCommit    string
	HoursWorked   string
	HackatimeData string
	UpdatedDate   string
	UpdatedUTC    string
	UpdatedIST    string
	YesterdayDate string
}

func main() {

	var params = ReadmeParams{}

	// check if the first argument is "local" to load the .env file

	if len(os.Args) > 1 && os.Args[1] == "local" {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
	}
	// Implementation for generating README goes here

	var slackID = os.Getenv("SLACK_ID")
	var githubUsername = os.Getenv("GITHUB_USERNAME")

	// fetch github data

	respGitHub, err := http.Get("https://api.github.com/users/" + githubUsername + "/events/public")
	if err != nil {
		log.Fatal(err)
	}
	defer respGitHub.Body.Close()

	var events []GitHubEvent
	if err := json.NewDecoder(respGitHub.Body).Decode(&events); err != nil {
		log.Fatal(err)
	}

	for _, event := range events {
		if event.Type == "PushEvent" {
			params.LastRepo = event.Repo.Name
			params.LastCommit = fmt.Sprintf("[%s](https://github.com/%s/commit/%s)", event.Payload.Head[:7], event.Repo.Name, event.Payload.Head)
			break
		}
	}

	// fetch hackatime data for yesterday

	yesterday := time.Now().AddDate(0, 0, -1)

	startDate := yesterday.Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")

	url := "https://hackatime.hackclub.com/api/v1/users/" + slackID +
		"/stats?start_date=" + startDate +
		"&end_date=" + endDate

	println("Fetching daily stats from URL: ", url)
	respDaily, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer respDaily.Body.Close()

	var dailyStats HackatimeStats
	if err := json.NewDecoder(respDaily.Body).Decode(&dailyStats); err != nil {
		panic(err)
	}

	// fetch all-time stats for language breakdown
	respAllTime, err := http.Get("https://hackatime.hackclub.com/api/v1/users/" + slackID + "/stats")
	if err != nil {
		panic(err)
	}
	defer respAllTime.Body.Close()

	var allTimeStats HackatimeStats
	if err := json.NewDecoder(respAllTime.Body).Decode(&allTimeStats); err != nil {
		panic(err)
	}

	// set params for hackatime

	params.HoursWorked = formatDuration(dailyStats.Data.TotalSeconds)
	params.HackatimeData = generateLanguageBars(allTimeStats.Data.Languages, 10)

	// set date and time

	now := time.Now()
	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		log.Fatal("Error loading IST timezone: ", err)
	}
	params.UpdatedDate = now.Format("02 Jan 2006")
	params.UpdatedUTC = now.UTC().Format("15:04")
	params.UpdatedIST = now.In(istLocation).Format("15:04")

	// generate README.md from template

	tmpl, err := template.ParseFiles("README.template.md")
	if err != nil {
		log.Fatalf("Error parsing template: %v. Check if 'README.template.md' exists.", err)
	}

	// Create the output file
	f, err := os.Create("../README.md")
	if err != nil {
		log.Fatal("Error creating README.md: ", err)
	}
	defer f.Close()

	// Execute the template
	if err := tmpl.Execute(f, params); err != nil {
		log.Fatal("Error executing template: ", err)
	}
}

const fullWidthSpace = " "

func visualWidth(s string) int {
	width := 0
	for _, r := range s {
		if r > 0xFF {
			width += 2
		} else {
			width += 1
		}
	}
	return width
}

func generateLanguageBars(langs []LanguageStat, top int) string {
	if len(langs) < top {
		top = len(langs)
	}
	if top == 0 {
		return ""
	}

	md := ""
	barLength := 20

	maxNameWidth := 0
	for i := 0; i < top; i++ {
		w := visualWidth(langs[i].Name)
		if w > maxNameWidth {
			maxNameWidth = w
		}
	}

	maxHours := langs[0].Hours
	if maxHours == 0 {
		maxHours = 1
	}

	for i := 0; i < top; i++ {
		lang := langs[i]
		var filled int
		if i == 0 {
			filled = barLength
		} else {
			filled = int(float64(lang.Hours)/float64(maxHours)*float64(barLength) + 0.5)
		}
		if filled > barLength {
			filled = barLength
		}
		empty := barLength - filled
		bar := fmt.Sprintf("[%s%s] %dh", strings.Repeat("█", filled), strings.Repeat("░", empty), lang.Hours)

		padding := maxNameWidth - visualWidth(lang.Name)
		md += fmt.Sprintf("↳ %s%s %s", lang.Name, strings.Repeat(fullWidthSpace, padding), bar)
		if i < top-1 {
			md += "\n"
		}
	}

	return md
}

func formatDuration(seconds int64) string {
	d := time.Duration(seconds) * time.Second

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dmin", int(d.Minutes()))
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%02d", hours, minutes)
	}
}
