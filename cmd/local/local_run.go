package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jdevelop/fs4map/kmlapi"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
)

const Year = time.Duration(24*365) * time.Hour

type TopLevel map[string]string
type Root map[string]string

const (
	ClientId          = "client.id"
	ClientRedirectUrl = "client.redirect.url"
	ClientToken       = "client.token"
	ClientSecret      = "client.secret"
	DatePattern       = "2006-01-02"
)

var (
	before     = time.Now()
	after      = before.Add(-(10 * Year)) // could fail
	flagBefore = flag.String("to", before.Format(DatePattern), "start date")
	flagAfter  = flag.String("from", after.Format(DatePattern), "end date")
)

func renderProgressBar(fetched int, total int) string {
	const width = 30
	if total <= 0 {
		return fmt.Sprintf("[%s] %d", strings.Repeat("=", width), fetched)
	}
	if fetched < 0 {
		fetched = 0
	}
	if fetched > total {
		fetched = total
	}
	filled := int((float64(fetched) / float64(total)) * float64(width))
	if filled > width {
		filled = width
	}
	return fmt.Sprintf("[%s%s] %d/%d", strings.Repeat("=", filled), strings.Repeat(" ", width-filled), fetched, total)
}

func main() {

	flag.Parse()

	viper.SetConfigName("config")
	viper.AddConfigPath("$HOME/.kmlexport")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}
	var token = viper.GetString(ClientToken)

	if token == "" {

		authUrl := kmlapi.PreAuthenticate(viper.GetString(ClientId), viper.GetString(ClientRedirectUrl))

		svc := httprouter.New()

		var wait sync.WaitGroup
		wait.Add(1)

		svc.GET("/api/export", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
			codeStr := r.URL.Query().Get("code")
			if codeStr == "" {
				http.Error(w, "missing code query parameter", http.StatusBadRequest)
				return
			}
			authToken, err := kmlapi.Authenticate(viper.GetString(ClientId),
				viper.GetString(ClientSecret),
				codeStr,
				viper.GetString(ClientRedirectUrl),
			)
			if err != nil {
				log.Printf("authenticate failed: %v", err)
				http.Error(w, "authentication failed", http.StatusBadGateway)
				return
			}
			token = authToken
			viper.Set(ClientToken, token)
			if err := viper.WriteConfig(); err != nil {
				log.Printf("failed to write config: %v", err)
				http.Error(w, "failed to persist token", http.StatusInternalServerError)
				return
			}
			log.Println("Token saved successfully")
			w.WriteHeader(http.StatusNoContent)
			wait.Done()
		})

		log.Println("Started server on :8080")

		go http.ListenAndServe(":8080", svc)

		println(authUrl)
		wait.Wait()
	}

	if v, err := time.Parse(DatePattern, *flagBefore); err == nil {
		before = v
	} else {
		log.Println("using default end time", before)
	}
	if v, err := time.Parse(DatePattern, *flagAfter); err == nil {
		after = v
	} else {
		log.Println("using default start time", after)
	}

	currentStage := ""
	k, stats, err := kmlapi.BuildKMLWithProgressAndStats(kmlapi.NewToken(token), &before, &after, func(stage string, fetched int, total int) {
		if stage != currentStage {
			if currentStage != "" {
				fmt.Println()
			}
			currentStage = stage
			fmt.Printf("%s: ", stage)
		}
		fmt.Printf("\r%s: %s", stage, renderProgressBar(fetched, total))
		if total > 0 && fetched >= total {
			fmt.Println()
		}
	})
	if err != nil {
		log.Fatal(err)
	}

	w, err := os.Create(fmt.Sprintf("export-%s-%s.kml", after.Format(DatePattern), before.Format(DatePattern)))
	if err != nil {
		log.Fatal(err)
	}

	if err := k.WriteIndent(w, "", "  "); err != nil {
		log.Fatal(err)
	}
	if err := w.Sync(); err != nil {
		log.Fatal(err)
	}
	if err := w.Close(); err != nil {
		log.Fatal(err)
	}

	if stats.UnmatchedVenueIDs > 0 {
		log.Printf("WARN: %d venue IDs from checkins were not present in fetched venue details (%d checkins unmatched)",
			stats.UnmatchedVenueIDs, stats.CheckinsUnmatchedToVenues)
	}
	if stats.CheckinsMissingVenueOrTime > 0 {
		log.Printf("WARN: skipped %d checkins with missing venue.id or createdAt", stats.CheckinsMissingVenueOrTime)
	}
	if stats.CheckinsDeduplicatedByVenueTs > 0 {
		log.Printf("WARN: deduplicated %d checkins by (venue.id, createdAt)", stats.CheckinsDeduplicatedByVenueTs)
	}

	fmt.Println("Export stats:")
	fmt.Printf("  Venues fetched: %d\n", stats.VenuesFetched)
	fmt.Printf("  Venues exported: %d\n", stats.VenuesExported)
	fmt.Printf("  Unknown-category venues: %d\n", stats.UnknownCategoryVenues)
	fmt.Printf("  Checkins raw fetched: %d\n", stats.CheckinsRawFetched)
	fmt.Printf("  Checkins retained after cleaning/dedupe: %d\n", stats.CheckinsUniqueRetained)
	fmt.Printf("  Checkins matched to exported venues: %d\n", stats.CheckinsMatchedToVenues)
	fmt.Printf("  Checkins unmatched to venue details: %d\n", stats.CheckinsUnmatchedToVenues)
	fmt.Printf("  Unmatched checkin venue IDs: %d\n", stats.UnmatchedVenueIDs)
	fmt.Printf("  Checkins skipped (missing venue/time): %d\n", stats.CheckinsMissingVenueOrTime)
	fmt.Printf("  Checkins deduplicated (venue/time): %d\n", stats.CheckinsDeduplicatedByVenueTs)
	fmt.Printf("  Output file: %s\n", fmt.Sprintf("export-%s-%s.kml", after.Format(DatePattern), before.Format(DatePattern)))

}
