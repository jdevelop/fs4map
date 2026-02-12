package kmlapi

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestFetchCheckinsPaginatesAndAggregatesByVenue(t *testing.T) {
	requests := 0
	withMockFSQServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/users/self/checkins" {
			http.NotFound(w, r)
			return
		}

		if r.URL.Query().Get("limit") != "250" {
			t.Fatalf("expected limit=250, got %q", r.URL.Query().Get("limit"))
		}
		if r.URL.Query().Get("beforeTimestamp") == "" || r.URL.Query().Get("afterTimestamp") == "" {
			t.Fatalf("expected beforeTimestamp and afterTimestamp to be set")
		}

		offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
		if err != nil {
			t.Fatalf("invalid offset: %v", err)
		}
		requests++

		switch offset {
		case 0:
			fmt.Fprint(w, `{
				"response": {
					"checkins": {
						"count": 3,
						"items": [
							{"createdAt": 200, "venue": {"id": "v1"}},
							{"createdAt": 100, "venue": {"id": "v1"}}
						]
					}
				}
			}`)
		case 2:
			fmt.Fprint(w, `{
				"response": {
					"checkins": {
						"count": 3,
						"items": [
							{"createdAt": 300, "venue": {"id": "v2"}}
						]
					}
				}
			}`)
		default:
			fmt.Fprint(w, `{"response":{"checkins":{"count":3,"items":[]}}}`)
		}
	})

	before := time.Unix(1000, 0)
	after := time.Unix(10, 0)

	byVenue, err := FetchCheckins(NewToken("token"), &before, &after, nil)
	if err != nil {
		t.Fatalf("FetchCheckins returned error: %v", err)
	}

	if requests != 2 {
		t.Fatalf("expected 2 paginated requests, got %d", requests)
	}
	if len(byVenue["v1"]) != 2 || byVenue["v1"][0] != 200 || byVenue["v1"][1] != 100 {
		t.Fatalf("unexpected v1 timestamps: %#v", byVenue["v1"])
	}
	if len(byVenue["v2"]) != 1 || byVenue["v2"][0] != 300 {
		t.Fatalf("unexpected v2 timestamps: %#v", byVenue["v2"])
	}
}

func TestFetchCheckinsReturnsErrorOnNon2xx(t *testing.T) {
	withMockFSQServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	})

	_, err := FetchCheckins(NewToken("token"), nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for non-2xx checkins response")
	}
}
