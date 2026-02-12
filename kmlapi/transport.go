package kmlapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	fsqOAuth2Base  = "https://foursquare.com/oauth2"
	fsqOAuth2      = fsqOAuth2Base + "/authenticate?response_type=code&"
	fsqOAuth2Token = fsqOAuth2Base + "/access_token?grant_type=authorization_code&"
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
