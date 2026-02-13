package kmlapi

import (
	"encoding/json"
	"fmt"
	"github.com/twpayne/go-kml"
	"strconv"
	"strings"
	"time"
)

const Year = time.Duration(24*365) * time.Hour
const unknownCategoryFolder = "Unknown"

type TopLevel map[string]string
type Root map[string]string
type ProgressCallback func(stage string, fetched int, total int)
type ExportStats struct {
	VenuesFetched                 int
	VenuesExported                int
	UnknownCategoryVenues         int
	CheckinsRawFetched            int
	CheckinsUniqueRetained        int
	CheckinsMatchedToVenues       int
	CheckinsUnmatchedToVenues     int
	UnmatchedVenueIDs             int
	CheckinsMissingVenueOrTime    int
	CheckinsDeduplicatedByVenueTs int
}

func reportProgress(progress ProgressCallback, stage string, fetched int, total int) {
	if progress != nil {
		progress(stage, fetched, total)
	}
}

func ResolveCategories(token FSQToken) (Root, TopLevel, error) {

	cats, err := FetchCategories(token)

	if err != nil {
		return nil, nil, err
	}

	root := make(map[string]string)

	idToName := make(map[string]string)

	var walk func(*GlobalCategory, string)

	walk = func(c *GlobalCategory, id string) {
		if c == nil {
			return
		}
		for _, inner := range c.Children {
			root[inner.Id] = c.Id
			walk(&inner, id)
		}
	}

	for _, c := range cats {
		idToName[c.Id] = c.Name
		root[c.Id] = c.Id
		walk(&c, c.Id)
	}

	return root, idToName, nil
}

func BuildKML(token FSQToken, before *time.Time, after *time.Time) (*kml.CompoundElement, error) {
	return BuildKMLWithProgress(token, before, after, nil)
}

func BuildKMLWithProgress(token FSQToken, before *time.Time, after *time.Time, progress ProgressCallback) (*kml.CompoundElement, error) {
	k, _, err := BuildKMLWithProgressAndStats(token, before, after, progress)
	return k, err
}

func BuildKMLWithProgressAndStats(token FSQToken, before *time.Time, after *time.Time, progress ProgressCallback) (*kml.CompoundElement, ExportStats, error) {
	stats := ExportStats{}
	venues, err := FetchVenues(token, before, after, progress)
	if err != nil {
		return nil, stats, err
	}
	stats.VenuesFetched = len(venues)

	checkinsByVenue, checkinStats, err := FetchCheckins(token, before, after, progress)
	if err != nil {
		return nil, stats, err
	}
	stats.CheckinsRawFetched = checkinStats.RawCheckinsFetched
	stats.CheckinsUniqueRetained = checkinStats.UniqueCheckinsRetained
	stats.CheckinsMissingVenueOrTime = checkinStats.MissingVenueOrTimestamp
	stats.CheckinsDeduplicatedByVenueTs = checkinStats.DeduplicatedByVenueAndTime

	venueSet := make(map[string]struct{}, len(venues))
	for i := range venues {
		venueSet[venues[i].Id] = struct{}{}
		venues[i].VisitTimestamps = checkinsByVenue[venues[i].Id]
	}
	for venueID, timestamps := range checkinsByVenue {
		if _, ok := venueSet[venueID]; ok {
			stats.CheckinsMatchedToVenues += len(timestamps)
			continue
		}
		stats.UnmatchedVenueIDs++
		stats.CheckinsUnmatchedToVenues += len(timestamps)
	}

	folders := make(map[string]*kml.CompoundElement)

	k := kml.KML()
	d := kml.Document()
	d.Add(
		kml.Schema(
			"visit-metadata",
			"VisitMetadata",
			kml.SimpleField("visit_count", "int"),
			kml.SimpleField("last_visit_unix", "int"),
			kml.SimpleField("visit_timestamps_unix", "string"),
		),
	)

	categoriesMap, idToName, err := ResolveCategories(token)
	if err != nil {
		return nil, stats, err
	}

	for _, item := range venues {
		place := kml.Placemark(
			kml.Name(item.Name),
			kml.Description(buildVisitDescription(item.VisitTimestamps)),
			buildVisitExtendedData(item.VisitTimestamps),
			kml.Point(
				kml.Coordinates(kml.Coordinate{Lon: item.Location.Lng, Lat: item.Location.Lat}),
			),
		)

		if len(item.Categories) == 0 {
			stats.UnknownCategoryVenues++
			folder := folders[unknownCategoryFolder]
			if folder == nil {
				folder = kml.Folder(kml.Name(unknownCategoryFolder))
				folders[unknownCategoryFolder] = folder
			}
			folder.Add(place)
			stats.VenuesExported++
			continue
		}

		for _, c := range item.Categories {
			topLevelName := idToName[categoriesMap[c.Id]]
			if topLevelName == "" {
				topLevelName = unknownCategoryFolder
			}
			folder := folders[topLevelName]
			if folder == nil {
				folder = kml.Folder(kml.Name(topLevelName))
				folders[topLevelName] = folder
			}
			folder.Add(place)
		}
		stats.VenuesExported++
	}

	for _, f := range folders {
		d.Add(f)
	}

	k.Add(d)
	return k, stats, nil
}

func buildVisitDescription(timestamps []int64) string {
	if len(timestamps) == 0 {
		return "Visit count: 0"
	}

	lines := []string{
		fmt.Sprintf("Visit count: %d", len(timestamps)),
		fmt.Sprintf("Last visit (UTC): %s", time.Unix(timestamps[0], 0).UTC().Format(time.RFC3339)),
		"Recent visits (UTC):",
	}

	limit := 5
	if len(timestamps) < limit {
		limit = len(timestamps)
	}
	for i := 0; i < limit; i++ {
		lines = append(lines, time.Unix(timestamps[i], 0).UTC().Format(time.RFC3339))
	}

	return strings.Join(lines, "\n")
}

func buildVisitExtendedData(timestamps []int64) *kml.CompoundElement {
	lastVisit := int64(0)
	if len(timestamps) > 0 {
		lastVisit = timestamps[0]
	}

	jsonTimestamps, err := json.Marshal(timestamps)
	if err != nil {
		jsonTimestamps = []byte("[]")
	}

	return kml.ExtendedData(
		kml.SchemaData(
			"#visit-metadata",
			kml.SimpleData("visit_count", strconv.Itoa(len(timestamps))),
			kml.SimpleData("last_visit_unix", strconv.FormatInt(lastVisit, 10)),
			kml.SimpleData("visit_timestamps_unix", string(jsonTimestamps)),
		),
	)
}
