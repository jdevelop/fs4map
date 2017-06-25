package kmlapi

import (
	"time"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"net/url"
	"fmt"
)

type FSQToken string

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

func NewToken(s string) FSQToken {
	return FSQToken(s)
}

func FetchVenues(token FSQToken, before *time.Time, after *time.Time) ([]Venue, error) {

	q := commonQuery(token)

	if before != nil {
		q.Add("beforeTimestamp", fmt.Sprintf("%1d", before.Unix()))
	}
	if after != nil {
		q.Add("afterTimestamp", fmt.Sprintf("%1d", after.Unix()))
	}

	urlStr := fsqHistory + q.Encode()

	resp, err := http.Get(urlStr)

	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	fsq := fsqResponse{}

	json.Unmarshal(content, &fsq)

	venues := make([]Venue, len(fsq.Response.Venues.Items))

	for i, v := range fsq.Response.Venues.Items {
		venues[i] = v.Venue
	}

	return venues, err
}

func FetchCategories(token FSQToken) ([]GlobalCategory, error) {
	q := commonQuery(token)

	urlStr := fsqCategories + q.Encode()
	resp, err := http.Get(urlStr)

	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	fsq := fsqCategory{}

	json.Unmarshal(content, &fsq)

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

	resp, err := http.Get(fsqOAuth2Token + q.Encode())

	if err != nil {
		return "", err
	}

	type AuthResponse struct {
		AccessToken string `json:"access_token"`
	}

	content, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return "", err
	}

	tokenResponse := AuthResponse{}

	json.Unmarshal(content, &tokenResponse)

	return tokenResponse.AccessToken, nil

}
