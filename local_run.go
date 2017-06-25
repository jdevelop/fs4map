package main

import (
	"github.com/jdevelop/fs4map/kmlapi"
	"os"
	"time"
	"github.com/spf13/viper"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"net/url"
	"sync"
)

const Year = time.Duration(24*365) * time.Hour

type TopLevel map[string]string
type Root map[string]string

func main() {

	viper.SetConfigName("config")
	viper.AddConfigPath("$HOME/.kmlexport")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	authUrl := kmlapi.PreAuthenticate(viper.GetString("client.id"), viper.GetString("client.redirect.url"))

	svc := httprouter.New()

	var tokenStr string

	wait := sync.WaitGroup{}
	wait.Add(1)

	svc.GET("/auth", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		u, _ := url.Parse(r.RequestURI)
		tokenStr = u.Query().Get("code")
		wait.Done()
	})

	go http.ListenAndServe(":8080", svc)

	println(authUrl)

	wait.Wait()

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

	k := kmlapi.BuildKML(kmlapi.NewToken(token), &before, &after)

	w, _ := os.Create("/tmp/kml.kml")

	k.WriteIndent(w, "", "  ")
	w.Sync()
	w.Close()

}
