package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"

	"github.com/ppicom/bookma/internal/aimharder"
)

type AppConfig struct {
	AimHarder aimharder.Config
}

func main() {
	var config AppConfig
	err := envconfig.Process("AIMHARDER", &config)
	if err != nil {
		log.Fatal(err.Error())
	}

	client, err := aimharder.New(config.AimHarder)
	if err != nil {
		log.Fatal(err.Error())
	}

	dates, err := nextWeekDates()
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Printf("Next week dates: %v\n", dates)

	rest, saturday := dates[:len(dates)-1], dates[len(dates)-1]

	var errs []error
	err = client.BookClass(config.AimHarder, saturday, "1100_60")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to book date: %s, error: %w", saturday, err))
	}

	for _, date := range rest {
		err = client.BookClass(config.AimHarder, date, "1800_60")
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to book date: %s, error: %w", date, err))
		} else {
			log.Printf("Class booked for date: %s\n", date)
		}

		log.Printf("Sleeping for 5 seconds\n")
		time.Sleep(5 * time.Second) // We don't want to wake the dragon
	}

	if len(errs) > 0 {
		log.Fatalf("Failed to book some classes: %v", errors.Join(errs...))
	}

	log.Println("Successfully booked all classes")
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

func bookClass(client *http.Client, config AppConfig, date string, timeID string) error {
	classes, err := getClasses(client, config, date)
	if err != nil {
		return err
	}
	class, err := findOneAt(classes, timeID)
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
