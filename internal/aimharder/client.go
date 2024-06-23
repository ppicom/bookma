package aimharder

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	Host   string `envconfig:"AIMHARDER_HOST" required:"true"`
	BoxID  string `envconfig:"AIMHARDER_BOX_ID" required:"true"`
	Cookie struct {
		Name  string `envconfig:"AIMHARDER_COOKIE_NAME" required:"true"`
		Value string `envconfig:"AIMHARDER_COOKIE_VALUE" required:"true"`
	}
	LogRequests bool `envconfig:"AIMHARDER_LOG_REQUESTS" default:"false"`
}

type Client struct {
	client *http.Client
	config Config
}

func New(config Config) (*Client, error) {
	cookie := &http.Cookie{
		Name:   config.Cookie.Name,
		Value:  config.Cookie.Value,
		Domain: config.Host,
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(fmt.Sprintf("https://%s", config.Host))
	if err != nil {
		return nil, err
	}
	jar.SetCookies(u, []*http.Cookie{cookie})
	client := &http.Client{
		Jar:       jar,
		Timeout:   1 * time.Minute,
		Transport: &loggingRoundTripper{rt: http.DefaultTransport, active: config.LogRequests},
	}
	return &Client{
		client: client,
		config: config,
	}, nil
}

func (cli *Client) BookClass(config Config, date string, timeID string) error {
	classes, err := cli.getClasses(date)
	if err != nil {
		return err
	}
	class, err := cli.findOneAt(classes, timeID)
	if err != nil {
		return err
	}
	return cli.book(class)
}

func (cli *Client) getClasses(date string) ([]Booking, error) {
	bookingsUrl := fmt.Sprintf(
		"https://%s/api/bookings?day=%s&box=%s",
		cli.config.Host,
		date,
		cli.config.BoxID,
	)
	req, err := http.NewRequest("GET", bookingsUrl, nil)
	if err != nil {
		return nil, err
	}

	res, err := cli.client.Do(req)
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

func (cli *Client) findOneAt(bookings []Booking, timeID string) (Booking, error) {
	for _, booking := range bookings {
		if booking.TimeID == timeID {
			return booking, nil
		}
	}
	return Booking{}, fmt.Errorf("no class found at %s", timeID)
}

func (cli *Client) book(booking Booking) error {
	bookUrl := fmt.Sprintf(
		"https://%s/api/book",
		cli.config.Host,
	)
	str := fmt.Sprintf("id=%d&day=%s", booking.ID, booking.Date)
	payload := strings.NewReader(str)
	req, err := http.NewRequest("POST", bookUrl, payload)
	if err != nil {
		return err
	}
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	res, err := cli.client.Do(req)
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
