package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type AppConfig struct {
	AimHarder struct {
		Host   string `envconfig:"AIMHARDER_HOST" required:"true"`
		BoxID  string `envconfig:"AIMHARDER_BOX_ID" required:"true"`
		Cookie struct {
			Name  string `envconfig:"AIMHARDER_COOKIE_NAME" required:"true"`
			Value string `envconfig:"AIMHARDER_COOKIE_VALUE" required:"true"`
		}
	}
}

func main() {
	var config AppConfig
	err := envconfig.Process("AIMHARDER", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	client, err := spinUpClient(config)
	if err != nil {
		log.Fatal(err.Error())
	}

	dates, err := nextWeekDates()
	if err != nil {
		log.Fatal(err.Error())
	}

	var errs []error
	for _, date := range dates {
		log.Printf("Booking for date: %s\n", date)

		// Book the date
		err = bookClass(client, config, date)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to book date: %s, error: %w", date, err))
		}

		log.Printf("Sleeping for 5 seconds\n")
		time.Sleep(10 * time.Second) // We don't want to wake the dragon
	}

	if len(errs) > 0 {
		for _, e := range errs {
			log.Println(e)
		}
		log.Fatalf("Failed to book some classes: %v", errors.Join(errs...))
	}

	log.Println("Successfully booked all classes")
}

func spinUpClient(config AppConfig) (*http.Client, error) {
	cookie := &http.Cookie{
		Name:   config.AimHarder.Cookie.Name,
		Value:  config.AimHarder.Cookie.Value,
		Domain: config.AimHarder.Host,
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(fmt.Sprintf("https://%s", config.AimHarder.Host))
	if err != nil {
		return nil, err
	}
	jar.SetCookies(u, []*http.Cookie{cookie})
	client := &http.Client{
		Jar:       jar,
		Timeout:   1 * time.Minute,
		Transport: &loggingRoundTripper{rt: http.DefaultTransport},
	}
	return client, nil
}

var now = time.Now

func nextWeekDates() ([]string, error) {
	var days []string
	today := now()
	for ; today.Weekday() != time.Monday; today = today.AddDate(0, 0, 1) {
	}
	// Get Monday through Saturday
	for i := 0; i < 6; i++ {
		days = append(days, today.AddDate(0, 0, i).Format("20060102"))
	}
	return days, nil
}

func bookClass(client *http.Client, config AppConfig, date string) error {
	classes, err := getClasses(client, config, date)
	if err != nil {
		return err
	}
	class, err := findOneAt(classes, "1800_60")
	if err != nil {
		return err
	}
	return book(client, config, class)
}

func getClasses(client *http.Client, config AppConfig, date string) ([]Booking, error) {
	bookingsUrl := fmt.Sprintf(
		"https://%s/api/bookings?day=%s&box=%s",
		config.AimHarder.Host,
		date,
		config.AimHarder.BoxID,
	)
	req, err := http.NewRequest("GET", bookingsUrl, nil)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch classes: %s", res.Status)
	}

	var bookingsResponse BookingsResponse
	err = json.NewDecoder(res.Body).Decode(&bookingsResponse)
	if err != nil {
		return nil, err
	}

	for i := range bookingsResponse.Bookings {
		bookingsResponse.Bookings[i].Date = date
	}

	log.Printf("Found %d classes for date: %s\n", len(bookingsResponse.Bookings), date)

	return bookingsResponse.Bookings, nil
}

func findOneAt(bookings []Booking, timeID string) (Booking, error) {
	for _, booking := range bookings {
		if booking.TimeID == timeID {
			return booking, nil
		}
	}
	return Booking{}, fmt.Errorf("no class found at %s", timeID)
}

func book(client *http.Client, config AppConfig, booking Booking) error {
	bookUrl := fmt.Sprintf(
		"https://%s/api/book",
		config.AimHarder.Host,
	)
	str := fmt.Sprintf("id=%d&day=%s", booking.ID, booking.Date)
	payload := strings.NewReader(str)
	req, err := http.NewRequest("POST", bookUrl, payload)
	if err != nil {
		return err
	}
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Check if body contains errorMssg property despite having received a 200
	var response BookingResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK || response.ErrorMssg != "" {
		return fmt.Errorf("failed to book class: %s", response.ErrorMssg)
	}

	return nil
}

type BookingsResponse struct {
	Bookings []Booking `json:"bookings"`
}

type Booking struct {
	ID        int    `json:"id"`
	TimeID    string `json:"timeid"`
	ClassName string `json:"className"`
	Date      string `json:"-"` // I add it to the struct to use it in the book function, not coming from the server.
}

// Define the struct to match the JSON structure
type BookingResponse struct {
	ClasesContratadas string `json:"clasesContratadas"`
	BookState         int    `json:"bookState"`
	ErrorMssg         string `json:"errorMssg"`
	ErrorMssgLang     string `json:"errorMssgLang"`
}

type loggingRoundTripper struct {
	rt http.RoundTripper
}

func (lrt *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Log the request
	logRequest(req)

	// Perform the request
	start := time.Now()
	resp, err := lrt.rt.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		log.Printf("Request failed: %v", err)
		return nil, err
	}

	// Log the response
	logResponse(resp, duration)

	return resp, nil
}

func logRequest(req *http.Request) {
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	log.Printf("Request: %s %s\nHeaders: %v\nBody: %s\n",
		req.Method, req.URL, req.Header, string(bodyBytes))
}

func logResponse(resp *http.Response, duration time.Duration) {
	var bodyBytes []byte
	if resp.Body != nil {
		bodyBytes, _ = io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	log.Printf("Response: %s\nDuration: %v\nStatus: %d\nHeaders: %v\nBody: %s\n",
		resp.Request.URL, duration, resp.StatusCode, resp.Header, string(bodyBytes))
}
