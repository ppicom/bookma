package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
	}

	err = errors.Join(errs...)
	if err != nil {
		log.Fatalf("Failed to book some classes: %s", err.Error())
	}

	log.Println("Successfully booked all classes")
}

func spinUpClient(config AppConfig) (*http.Client, error) {
	cookie := &http.Cookie{
		Name:  config.AimHarder.Cookie.Name,
		Value: config.AimHarder.Cookie.Value,
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
		Jar:     jar,
		Timeout: 1 * time.Minute,
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

func getClasses(
	client *http.Client,
	config AppConfig,
	date string,
) ([]Booking, error) {
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

	var bookingsResponse BookingsResponse
	err = json.NewDecoder(res.Body).Decode(&bookingsResponse)
	if err != nil {
		return nil, err
	}

	for i := range bookingsResponse.Bookings {
		bookingsResponse.Bookings[i].Date = date
	}

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
	str := fmt.Sprintf("id=%s&day=%s&insist=0&familyId=", booking.ID, booking.Date)
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

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to book class: %s", res.Status)
	}

	return nil
}

type BookingsResponse struct {
	Bookings []Booking `json:"bookings"`
}

type Booking struct {
	ID        string `json:"id"`
	TimeID    string `json:"timeid"`
	ClassName string `json:"className"`
	Date      string `json:"-"` // I add it to the struct to use it in the book function, not coming from the server.
}
