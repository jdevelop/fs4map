package kmlapi

import (
	"github.com/twpayne/go-kml"
	"time"
)

const Year = time.Duration(24*365) * time.Hour

type TopLevel map[string]string
type Root map[string]string

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

	venues, err := FetchVenues(token, before, after)
	if err != nil {
		return nil, err
	}

	folders := make(map[string]*kml.CompoundElement)

	k := kml.KML()
	d := kml.Document()

	categoriesMap, idToName, err := ResolveCategories(token)
	if err != nil {
		return nil, err
	}

	for _, item := range venues {
		place := kml.Placemark(
			kml.Name(item.Name),
			kml.Point(
				kml.Coordinates(kml.Coordinate{Lon: item.Location.Lng, Lat: item.Location.Lat}),
			),
		)
		for _, c := range item.Categories {
			topLevelName := idToName[categoriesMap[c.Id]]
			if topLevelName == "" {
				topLevelName = "Undefined"
			}
			folder := folders[topLevelName]
			if folder == nil {
				folder = kml.Folder(kml.Name(topLevelName))
				folders[topLevelName] = folder
			}
			folder.Add(place)
		}
	}

	for _, f := range folders {
		d.Add(f)
	}

	k.Add(d)
	return k, nil
}
