package kmlapi

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type rewriteTransport struct {
	base *url.URL
	rt   http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = t.base.Scheme
	cloned.URL.Host = t.base.Host
	cloned.Host = t.base.Host
	return t.rt.RoundTrip(cloned)
}

func withMockFSQServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	original := defaultHTTPClient
	defaultHTTPClient = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &rewriteTransport{
			base: baseURL,
			rt:   http.DefaultTransport,
		},
	}
	t.Cleanup(func() {
		defaultHTTPClient = original
	})
}

func TestResolveCategoriesMapsChildrenToTopLevel(t *testing.T) {
	withMockFSQServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/venues/categories" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"response": {
				"categories": [
					{
						"id": "top-food",
						"name": "Food",
						"categories": [
							{
								"id": "child-coffee",
								"name": "Coffee Shop",
								"categories": []
							}
						]
					}
				]
			}
		}`)
	})

	root, idToName, err := ResolveCategories(NewToken("token"))
	if err != nil {
		t.Fatalf("ResolveCategories returned error: %v", err)
	}

	if root["top-food"] != "top-food" {
		t.Fatalf("expected top-level category to map to itself, got %q", root["top-food"])
	}
	if root["child-coffee"] != "top-food" {
		t.Fatalf("expected child category to map to top-level id, got %q", root["child-coffee"])
	}
	if idToName["top-food"] != "Food" {
		t.Fatalf("expected top-level name to be tracked, got %q", idToName["top-food"])
	}
}

func TestBuildKMLBuildsFolderedOutput(t *testing.T) {
	withMockFSQServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/venues/categories":
			fmt.Fprint(w, `{
				"response": {
					"categories": [
						{
							"id": "top-food",
							"name": "Food",
							"categories": [
								{
									"id": "child-coffee",
									"name": "Coffee Shop",
									"categories": []
								}
							]
						}
					]
				}
				}`)
		case "/v2/users/self/venuehistory":
			fmt.Fprint(w, `{
				"response": {
					"venues": {
						"items": [
							{
								"venue": {
									"id": "v1",
									"name": "Cafe One",
									"location": {"lat": 1.1, "lng": 2.2},
									"categories": [{"id": "child-coffee", "name": "Coffee Shop"}]
								}
							}
						]
					}
				}
				}`)
		case "/v2/users/self/checkins":
			fmt.Fprint(w, `{
				"response": {
					"checkins": {
						"count": 2,
						"items": [
							{
								"createdAt": 1770785520,
								"venue": {"id": "v1"}
							},
							{
								"createdAt": 1770770687,
								"venue": {"id": "v1"}
							}
						]
					}
				}
			}`)
		default:
			http.NotFound(w, r)
		}
	})

	before := time.Now()
	after := before.Add(-24 * time.Hour)

	k, err := BuildKML(NewToken("token"), &before, &after)
	if err != nil {
		t.Fatalf("BuildKML returned error: %v", err)
	}

	var buf bytes.Buffer
	if err := k.WriteIndent(&buf, "", "  "); err != nil {
		t.Fatalf("WriteIndent returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "<name>Food</name>") {
		t.Fatalf("expected top-level folder name in KML output, got: %s", out)
	}
	if !strings.Contains(out, "<name>Cafe One</name>") {
		t.Fatalf("expected placemark name in KML output, got: %s", out)
	}
	if !strings.Contains(out, "Visit count: 2") {
		t.Fatalf("expected visit count in KML output, got: %s", out)
	}
	if !strings.Contains(out, "<ExtendedData>") {
		t.Fatalf("expected ExtendedData in KML output, got: %s", out)
	}
	if !strings.Contains(out, "visit_timestamps_unix") {
		t.Fatalf("expected visit_timestamps_unix field in KML output, got: %s", out)
	}
}

func TestBuildKMLReturnsErrorWhenVenueHistoryFails(t *testing.T) {
	withMockFSQServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/users/self/venuehistory" {
			http.Error(w, "upstream failure", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"response":{"categories":[]}}`)
	})

	before := time.Now()
	after := before.Add(-24 * time.Hour)

	_, err := BuildKML(NewToken("token"), &before, &after)
	if err == nil {
		t.Fatal("expected error from BuildKML when venues fetch fails")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected HTTP status in error, got: %v", err)
	}
}
