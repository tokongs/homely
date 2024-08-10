package main

import (
	"context"
	"log"

	"github.com/spf13/pflag"
	"github.com/tokongs/homely"
)

func main() {
	username := pflag.StringP("username", "u", "", "Username to your Homely account.")
	password := pflag.StringP("password", "p", "", "Password to your Homely account.")
	pflag.Parse()

	if *username == "" || *password == "" {
		log.Fatal("must provide username and password")
	}

	c := homely.New(homely.Config{
		Username: *username,
		Password: *password,
	})

	locs, err := c.Locations(context.Background())
	if err != nil {
		log.Fatal("get locations ", err)
	}

	if len(locs) < 1 {
		log.Fatal("failed to find locations")
	}

	l, err := c.LocationDetails(context.Background(), locs[0].LocationID)
	if err != nil {
		log.Fatal("get location details ", err)
	}

  log.Println(l)

	err = c.Stream(context.Background(), locs[0].LocationID, func(e homely.Event) {
    log.Println(e)
	})

	if err != nil {
		log.Fatal(err)
	}
}
