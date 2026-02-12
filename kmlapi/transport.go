package kmlapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FSQToken string

var defaultHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

const (
	fsqBase       = "https://api.foursquare.com/v2"
	fsqHistory    = fsqBase + "/users/self/venuehistory?"
	fsqCategories = fsqBase + "/venues/categories?"
	fsqCheckins   = fsqBase + "/users/self/checkins?"

	fsqOAuth2Base  = "https://foursquare.com/oauth2"
	fsqOAuth2      = fsqOAuth2Base + "/authenticate?response_type=code&"
	fsqOAuth2Token = fsqOAuth2Base + "/access_token?grant_type=authorization_code&"

	checkinsPageLimit = 250
	maxCheckinsPages  = 1000
)

func commonQuery(token FSQToken) url.Values {
	q := url.Values{}

	q.Add("oauth_token", string(token))
	q.Add("v", "201301016")

	return q
}

func getJSON(urlStr string, out interface{}) error {
	resp, err := defaultHTTPClient.Get(urlStr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		content, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return fmt.Errorf("request failed with status %s", resp.Status)
		}
		msg := strings.TrimSpace(string(content))
		if msg == "" {
			return fmt.Errorf("request failed with status %s", resp.Status)
		}
		return fmt.Errorf("request failed with status %s: %s", resp.Status, msg)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(content, out); err != nil {
		return err
	}

	return nil
}

func NewToken(s string) FSQToken {
	return FSQToken(s)
}

func FetchVenues(token FSQToken, before *time.Time, after *time.Time) ([]Venue, error) {

	q := commonQuery(token)

	if before != nil {
		q.Add("beforeTimestamp", strconv.FormatInt(before.Unix(), 10))
	}
	if after != nil {
		q.Add("afterTimestamp", strconv.FormatInt(after.Unix(), 10))
	}

	urlStr := fsqHistory + q.Encode()

	type fsqResponse struct {
		Response struct {
			Venues struct {
				Items []struct {
					Venue Venue `json:"venue"`
				} `json:"items"`
			} `json:"venues"`
		} `json:"response"`
	}

	fsq := fsqResponse{}
	if err := getJSON(urlStr, &fsq); err != nil {
		return nil, err
	}
	venues := make([]Venue, len(fsq.Response.Venues.Items))
	for i, v := range fsq.Response.Venues.Items {
		venues[i] = v.Venue
	}

	return venues, nil
}

func FetchCategories(token FSQToken) ([]GlobalCategory, error) {
	q := commonQuery(token)
	urlStr := fsqCategories + q.Encode()

	var fsq fsqCategory
	if err := getJSON(urlStr, &fsq); err != nil {
		return nil, err
	}

	return fsq.Response.Categories, nil
}

func FetchCheckins(token FSQToken, before *time.Time, after *time.Time) (map[string][]int64, error) {
	type checkinItem struct {
		CreatedAt int64 `json:"createdAt"`
		Venue     struct {
			Id string `json:"id"`
		} `json:"venue"`
	}

	type fsqResponse struct {
		Response struct {
			Checkins struct {
				Count int           `json:"count"`
				Items []checkinItem `json:"items"`
			} `json:"checkins"`
		} `json:"response"`
	}

	checkinsByVenue := make(map[string][]int64)
	seenByVenue := make(map[string]map[int64]struct{})
	offset := 0

	for page := 0; page < maxCheckinsPages; page++ {
		q := commonQuery(token)
		q.Add("limit", strconv.Itoa(checkinsPageLimit))
		q.Add("offset", strconv.Itoa(offset))
		if before != nil {
			q.Add("beforeTimestamp", strconv.FormatInt(before.Unix(), 10))
		}
		if after != nil {
			q.Add("afterTimestamp", strconv.FormatInt(after.Unix(), 10))
		}

		var fsq fsqResponse
		if err := getJSON(fsqCheckins+q.Encode(), &fsq); err != nil {
			return nil, err
		}

		items := fsq.Response.Checkins.Items
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			if item.Venue.Id == "" || item.CreatedAt == 0 {
				continue
			}
			seen := seenByVenue[item.Venue.Id]
			if seen == nil {
				seen = make(map[int64]struct{})
				seenByVenue[item.Venue.Id] = seen
			}
			if _, exists := seen[item.CreatedAt]; exists {
				continue
			}
			seen[item.CreatedAt] = struct{}{}
			checkinsByVenue[item.Venue.Id] = append(checkinsByVenue[item.Venue.Id], item.CreatedAt)
		}

		offset += len(items)
		if offset >= fsq.Response.Checkins.Count {
			break
		}
	}

	for venueID := range checkinsByVenue {
		sort.Slice(checkinsByVenue[venueID], func(i, j int) bool {
			return checkinsByVenue[venueID][i] > checkinsByVenue[venueID][j]
		})
	}

	return checkinsByVenue, nil
}

func PreAuthenticate(clientId string, redirectUri string) string {
	q := url.Values{}
	q.Add("client_id", clientId)
	q.Add("redirect_uri", redirectUri)

	return fsqOAuth2 + q.Encode()
}

func Authenticate(clientId string, clientSecret string, code string, redirectUri string) (string, error) {
	q := url.Values{}
	q.Add("client_id", clientId)
	q.Add("redirect_uri", redirectUri)
	q.Add("client_secret", clientSecret)
	q.Add("code", code)

	type AuthResponse struct {
		AccessToken string `json:"access_token"`
	}

	var tokenResponse AuthResponse
	if err := getJSON(fsqOAuth2Token+q.Encode(), &tokenResponse); err != nil {
		return "", err
	}

	return tokenResponse.AccessToken, nil
}
