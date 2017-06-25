package main

import (
	"github.com/jdevelop/fs4map/kmlapi"
	"github.com/twpayne/go-kml"
	"os"
	"time"
	"github.com/spf13/viper"
	"bufio"
)

const Year = time.Duration(24*365) * time.Hour

type TopLevel map[string]string
type Root map[string]string

func resolveCategories(token kmlapi.FSQToken) (root Root, idToName TopLevel) {

	root = make(map[string]string)

	idToName = make(map[string]string)

	cats, err := kmlapi.FetchCategories(token)

	if err != nil {
		println(err)
		return nil, nil
	}

	var walk func(*kmlapi.GlobalCategory, string)

	walk = func(c *kmlapi.GlobalCategory, id string) {
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

	return root, idToName
}

func BuildKML(token kmlapi.FSQToken, before *time.Time, after *time.Time) *kml.CompoundElement {

	venues, _ := kmlapi.FetchVenues(token, before, after)

	folders := make(map[string]*kml.CompoundElement)

	k := kml.KML()
	d := kml.Document()

	categoriesMap, idToName := resolveCategories(token)

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
	return k
}

func main() {

	viper.SetConfigName("config")
	viper.AddConfigPath("$HOME/.kmlexport")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	url := kmlapi.PreAuthenticate(viper.GetString("client.id"), viper.GetString("client.redirect.url"))

	println(url)

	reader := bufio.NewReader(os.Stdin)
	tokenStr, _ := reader.ReadString('\n')

	before := time.Now()
	after := before.Add(- (7 * Year))

	token, err := kmlapi.Authenticate(viper.GetString("client.id"),
		viper.GetString("client.secret"),
		tokenStr,
		viper.GetString("client.redirect.url"),
	)

	if err != nil {
		panic(err)
	}

	k := BuildKML(kmlapi.NewToken(token), &before, &after)

	w, _ := os.Create("/tmp/kml.kml")

	k.WriteIndent(w, "", "  ")
	w.Sync()
	w.Close()

}