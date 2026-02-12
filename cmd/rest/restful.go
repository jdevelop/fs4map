package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jdevelop/fs4map/kmlapi"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
	"net/http"
	"time"
)

var (
	port   = flag.Int("port", 8080, "port to listen on")
	host   = flag.String("host", "localhost", "port to listen on")
	prefix = flag.String("prefix", "/api/", "url prefix, must end with /")
)

func main() {

	flag.Parse()

	viper.SetConfigName("config")
	viper.AddConfigPath("$HOME/.kmlexport")
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}

	authUrl := kmlapi.PreAuthenticate(viper.GetString("client.id"), viper.GetString("client.redirect.url"))

	println(authUrl)

	type PreauthResponse struct {
		Url string `json:"auth"`
	}

	svc := httprouter.New()

	svc.GET(*prefix+"preauth", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Add("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.Encode(PreauthResponse{Url: authUrl})
	})

	svc.GET(*prefix+"export", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		tokenStr := r.URL.Query().Get("code")

		before := time.Now()
		after := before.Add(-(7 * kmlapi.Year))

		token, err := kmlapi.Authenticate(viper.GetString("client.id"),
			viper.GetString("client.secret"),
			tokenStr,
			viper.GetString("client.redirect.url"),
		)

		if err != nil {
			http.Error(w, "Can not fetch checkins", 500)
		} else {
			k, err := kmlapi.BuildKML(kmlapi.NewToken(token), &before, &after)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Disposition", "attachment; filename=kml-export.kml")
			w.Header().Add("Content-Type", "application/vnd.google-earth.kml+xml")
			k.WriteIndent(w, "", "  ")
		}
	})

	http.ListenAndServe(fmt.Sprintf("%1s:%2d", *host, *port), svc)

}
