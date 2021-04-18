package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-redis/redis/v8"
	_ "github.com/joho/godotenv/autoload"
)

const dataURL string = "https://impfdashboard.de/static/data/germany_vaccinations_timeseries_v2.tsv"
const redisKey string = "lastProcessedDay"

var ctx = context.Background()

type APICred struct {
	APIKEY       string
	APISECRET    string
	ACCESSTOKEN  string
	ACCESSSECRET string
}

type Stats struct {
	Date       time.Time
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

func RedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_URL"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
}

func ParseDate(str string) time.Time {
	date, err := time.Parse("2006-01-02", str)
	if err != nil {
		panic(err)
	}

	return date
}

func ParseFloat(str string) float64 {
	number, err := strconv.ParseFloat(str, 64)
	number = math.Round(number*10000) / 100
	if err != nil {
		panic(err)
	}

	return number
}

func LoadLastProcessedDay(rdb *redis.Client) (exists bool, val string) {
	val, err := rdb.Get(ctx, redisKey).Result()
	if err != nil {
		return false, ""
	}

	return true, val
}

func SetLastProcessedDay(rdb *redis.Client, stats Stats) {
	err := rdb.Set(ctx, redisKey, stats.Date.Format("2006-01-02"), 0).Err()
	if err != nil {
		panic(err)
	}
}

func BuildStats(data []string) Stats {
	date := ParseDate(data[0])
	firstVacc := ParseFloat(data[10])
	secondVacc := ParseFloat(data[11])
	stats := Stats{Date: date, FirstVacc: firstVacc, SecondVacc: secondVacc}

	return stats
}

func LoadData() [][]string {
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

	return data
}

func ParseStatsAt(data [][]string, pos int) Stats {
	dayData := data[pos]
	prevDayData := data[pos-1]
	day := BuildStats(dayData)
	prevDay := BuildStats(prevDayData)
	day.FirstDiff = day.FirstVacc - prevDay.FirstVacc
	day.SecondDiff = day.SecondVacc - prevDay.SecondVacc

	return day
}

func LoadStatsUntil(until time.Time) []Stats {
	data := LoadData()
	days := []Stats{}
	pos := len(data) - 1
	var stats Stats

	for {
		stats = ParseStatsAt(data, pos)

		if stats.Date == until {
			break
		}

		days = append([]Stats{stats}, days...)
		pos--
	}

	return days
}

func LoadMostRecentStats() []Stats {
	data := LoadData()

	lastDay := ParseStatsAt(data, len(data)-1)

	return []Stats{lastDay}
}

func LoadUnprocessedDays(rdb *redis.Client) []Stats {
	exists, lastDayStr := LoadLastProcessedDay(rdb)

	if exists {
		lastDay := ParseDate(lastDayStr)
		return LoadStatsUntil(lastDay)
	} else {
		return LoadMostRecentStats()
	}
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

func SendTweet(api *anaconda.TwitterApi, stats Stats) {
	rawTweet := "COVID-19 vaccinations in Germany as of %s:\n\n" +
		"Partially vaccinated:\n%s\n%.1f%% (+%.1f)\n\n" +
		"Fully vaccinated:\n%s\n%.1f%% (+%.1f)"

	firstBar := DrawProgress(stats.FirstVacc)
	secondBar := DrawProgress(stats.SecondVacc)

	tweet := fmt.Sprintf(rawTweet, stats.Date.Format("02.01.2006"),
		firstBar, stats.FirstVacc, stats.FirstDiff,
		secondBar, stats.SecondVacc, stats.SecondDiff)

	_, err := api.PostTweet(tweet, url.Values{})
	if err != nil {
		panic(err)
	}
}

func ProcessDataAndSendTweets() {
	env := LoadEnv()
	rdb := RedisClient()
	api := anaconda.NewTwitterApiWithCredentials(env.ACCESSTOKEN, env.ACCESSSECRET, env.APIKEY, env.APISECRET)

	days := LoadUnprocessedDays(rdb)

	if len(days) == 0 {
		return
	}

	for _, day := range days {
		SendTweet(api, day)
	}

	SetLastProcessedDay(rdb, days[len(days)-1])
}

func main() {
	lambda.Start(ProcessDataAndSendTweets)
}
