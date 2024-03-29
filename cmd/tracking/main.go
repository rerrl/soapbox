package main

import (
	"flag"
	"log"

	"github.com/dukex/mixpanel"

	"github.com/soapboxsocial/soapbox/pkg/activeusers"
	"github.com/soapboxsocial/soapbox/pkg/conf"
	"github.com/soapboxsocial/soapbox/pkg/pubsub"
	"github.com/soapboxsocial/soapbox/pkg/redis"
	"github.com/soapboxsocial/soapbox/pkg/sql"
	"github.com/soapboxsocial/soapbox/pkg/tracking/backends"
	"github.com/soapboxsocial/soapbox/pkg/tracking/trackers"
)

type Conf struct {
	Trackers struct {
		RoomTimeLog bool `mapstructure:"roomtimelog"`
		Mixpanel    bool `mapstructure:"mixpanel"`
		LastActive  bool `mapstructure:"lastactive"`
	} `mapstructure:"trackers"`
	Mixpanel struct {
		Token string `mapstructure:"token"`
		URL   string `mapstructure:"url"`
	} `mapstructure:"mixpanel"`
	Redis conf.RedisConf    `mapstructure:"redis"`
	DB    conf.PostgresConf `mapstructure:"db"`
}

func parse() (*Conf, error) {
	var file string
	flag.StringVar(&file, "c", "config.toml", "config file")
	flag.Parse()

	config := &Conf{}
	err := conf.Load(file, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func main() {
	config, err := parse()
	if err != nil {
		log.Fatal("failed to parse config")
	}

	rdb := redis.NewRedis(config.Redis)
	queue := pubsub.NewQueue(rdb)

	db, err := sql.Open(config.DB)
	if err != nil {
		log.Fatalf("failed to open db: %s", err)
	}

	t := make([]trackers.Tracker, 0)

	if config.Trackers.Mixpanel {
		client := mixpanel.New(config.Mixpanel.Token, config.Mixpanel.URL)
		mt := trackers.NewMixpanelTracker(client)
		t = append(t, mt)
	}

	if config.Trackers.RoomTimeLog {
		backend := backends.NewUserRoomLogBackend(db)
		rt := trackers.NewUserRoomLogTracker(backend, queue)
		t = append(t, rt)
	}

	if config.Trackers.LastActive {
		backend := activeusers.NewBackend(db)
		at := trackers.NewRecentlyActiveTracker(backend, redis.NewTimeoutStore(rdb))
		t = append(t, at)
	}

	events := queue.Subscribe(pubsub.RoomTopic, pubsub.UserTopic, pubsub.StoryTopic)

	for evt := range events {
		for _, tracker := range t {
			if !tracker.CanTrack(evt) {
				continue
			}

			err := tracker.Track(evt)
			if err != nil {
				log.Printf("tacker.Track err %v", err)
			}
		}
	}
}
