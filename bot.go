package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/ChimeraCoder/anaconda"
	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/joho/godotenv/autoload"
)

const dataURL string = "https://impfdashboard.de/static/data/germany_vaccinations_timeseries_v2.tsv"

type APICred struct {
	APIKEY       string
	APISECRET    string
	ACCESSTOKEN  string
	ACCESSSECRET string
}

type Stats struct {
	Day        string
	FirstVacc  float64
	FirstDiff  float64
	SecondVacc float64
	SecondDiff float64
}

func LoadEnv() (env APICred) {
	env = APICred{
		os.Getenv("API_KEY"),
		os.Getenv("API_SECRET"),
		os.Getenv("ACCESS_TOKEN"),
		os.Getenv("ACCESS_SECRET")}

	return env
}

func ParseFloat(str string) float64 {
	number, err := strconv.ParseFloat(str, 64)
	number = math.Round(number*10000) / 100
	if err != nil {
		panic(err)
	}

	return number
}

func BuildStats(data []string) Stats {
	firstVacc := ParseFloat(data[10])
	secondVacc := ParseFloat(data[11])
	stats := Stats{Day: data[0], FirstVacc: firstVacc, SecondVacc: secondVacc}

	return stats
}

func LoadStats() Stats {
	resp, err := http.Get(dataURL)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
	reader := csv.NewReader(resp.Body)
	reader.Comma = '\t'
	data, err := reader.ReadAll()
	if err != nil {
		panic(err)
	}

	lastDayData := data[len(data)-1]
	prevDayData := data[len(data)-2]
	lastDay := BuildStats(lastDayData)
	prevDay := BuildStats(prevDayData)
	lastDay.FirstDiff = lastDay.FirstVacc - prevDay.FirstVacc
	lastDay.SecondDiff = lastDay.SecondVacc - prevDay.SecondVacc

	return lastDay
}

func DrawProgress(percent float64) string {
	sections := 15
	sectionPercent := 100.0 / float64(sections)

	filledSections := int(percent / sectionPercent)
	unfilledSections := sections - filledSections

	bar := strings.Repeat("▓", filledSections)
	bar += strings.Repeat("░", unfilledSections)

	return bar
}

func SendTweet() {
	env := LoadEnv()
	stats := LoadStats()
	api := anaconda.NewTwitterApiWithCredentials(env.ACCESSTOKEN, env.ACCESSSECRET, env.APIKEY, env.APISECRET)

	rawTweet := "COVID-19 vaccinations in the Germany:\n\n" +
		"Partially vaccinated:\n%s\n%.2f%% (+%.2f)\n\n" +
		"Fully vaccinated:\n%s\n%.2f%% (+%.2f)"

	firstBar := DrawProgress(stats.FirstVacc)
	secondBar := DrawProgress(stats.SecondVacc)

	tweet := fmt.Sprintf(rawTweet,
		firstBar, stats.FirstVacc, stats.FirstDiff,
		secondBar, stats.SecondVacc, stats.SecondDiff)

	_, err := api.PostTweet(tweet, url.Values{})
	if err != nil {
		panic(err)
	}
}

func main() {
	lambda.Start(SendTweet)
}
