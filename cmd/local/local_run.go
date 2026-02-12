package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
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

	k, err := kmlapi.BuildKML(kmlapi.NewToken(token), &before, &after)
	if err != nil {
		log.Fatal(err)
	}

	w, err := os.Create(fmt.Sprintf("export-%s-%s.kml", after.Format(DatePattern), before.Format(DatePattern)))
	if err != nil {
		log.Fatal(err)
	}

	k.WriteIndent(w, "", "  ")
	w.Sync()
	w.Close()

}
