package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/event"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"
	"net/url"
	"os"
	"strings"
	"time"
)

func exit(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}

const (
	envURL      = "VCEL_URL"
	envInsecure = "VCEL_INSECURE"
)

func GetEnvString(key string, defaultValue string) string {
	r := os.Getenv(key)
	if r == "" {
		return defaultValue
	}
	return r
}

func GetEnvBool(key string, defaultValue bool) bool {
	r := os.Getenv(key)
	if r == "" {
		return defaultValue
	}
	return r == "1"
}

func main() {

	vcUrl := flag.String("url", GetEnvString(envURL, ""), fmt.Sprintf("vCenter Connect URL [%s]", envURL))
	insecure := flag.Bool("insecure", GetEnvBool(envInsecure, false), fmt.Sprintf("Don't verify Certificate [%s]", envInsecure))

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("url=%s insecure=%s\n", *vcUrl, *insecure)

	url, err := url.Parse(*vcUrl)
	if err != nil {
		exit(err)
	}

	client, err := govmomi.NewClient(ctx, url, *insecure)
	if err != nil {
		exit(err)
	}

	finder := find.NewFinder(client.Client, false)

	dc, err := finder.DefaultDatacenter(ctx)

	if err != nil {
		exit(err)
	}

	filter := types.EventFilterSpec{
		Entity: &types.EventFilterSpecByEntity{
			Entity:    dc.Reference(),
			Recursion: types.EventFilterSpecRecursionOptionAll,
		},
	}

	eventManager := event.NewManager(client.Client)

	collector, err := eventManager.CreateCollectorForEvents(ctx, filter)
	defer collector.Destroy(ctx)

	collector.SetPageSize(ctx, 0)
	collector.Reset(ctx)

	for true {
		items, err := collector.ReadNextEvents(ctx, 10)

		if err != nil {
			exit(err)
		}

		for _, e := range items {
			category, _ := eventManager.EventCategory(ctx, e)
			event := e.GetEvent()
			msg := strings.TrimSpace(event.FullFormattedMessage)
			m := make(map[string]interface{})
			m["type"] = "Event"
			if t, ok := e.(*types.TaskEvent); ok {
				m["type"] = "Task"
				m["targetType"] = t.Info.Entity.Type
				m["targetName"] = t.Info.EntityName
			}
			m["message"] = msg
			m["category"] = category
			if event.Host != nil && event.Host.Name != "" {
				m["host"] = event.Host.Name
			}
			if event.Vm != nil && event.Vm.Name != "" {
				m["vm"] = event.Vm.Name
			}
			if event.UserName != "" {
				m["username"] = event.UserName
			}
			t := event.CreatedTime
			m["time"] = t.Unix()
			s, _ := json.Marshal(m)
			fmt.Printf("%s\t%s\n", t.Local().Format(time.RFC3339), string(s))
		}
		time.Sleep(10 * 1000 * 1000 * 1000)

	}
}

// vim: sw=4 ts=4 sts=4 number
