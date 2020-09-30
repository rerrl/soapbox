package main

import (
	"errors"
	"log"
	"strconv"

	"github.com/dukex/mixpanel"
	"github.com/go-redis/redis/v8"

	"github.com/soapboxsocial/soapbox/pkg/pubsub"
)

func main() {
	tracker := mixpanel.New("d124ce8f1516eb7baa7980f4de68ded5", "https://api-eu.mixpanel.com")

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	queue := pubsub.NewQueue(rdb)

	events := queue.Subscribe(pubsub.RoomTopic, pubsub.UserTopic)

	for event := range events {
		event := handleEvent(event)
		if event == nil {
			continue
		}

		go func() {
			err := tracker.Track(event.id, event.name, event.evt)
			if err != nil {
				log.Printf("tracker.Track err: %v\n", err)
			}
		}()
	}
}

type Event struct {
	id   string
	name string
	evt  *mixpanel.Event
}

func handleEvent(event *pubsub.Event) *Event {
	switch event.Type {
	case pubsub.EventTypeNewRoom:
		id, err := getId(event, "creator")
		if err != nil {
			return nil
		}

		return &Event{
			id:   strconv.Itoa(id),
			name: "room_new",
			evt: &mixpanel.Event{
				IP: "0",
				Properties: map[string]interface{}{
					"room_id":    event.Params["id"],
					"visibility": event.Params["visibility"],
				},
			},
		}
	case pubsub.EventTypeRoomJoin:
		id, err := getId(event, "creator")
		if err != nil {
			return nil
		}

		return &Event{
			id:   strconv.Itoa(id),
			name: "room_join",
			evt: &mixpanel.Event{
				IP: "0",
				Properties: map[string]interface{}{
					"room_id":    event.Params["id"],
					"visibility": event.Params["visibility"],
				},
			},
		}
	case pubsub.EventTypeRoomLeft:
		id, err := getId(event, "creator")
		if err != nil {
			return nil
		}

		return &Event{
			id:   strconv.Itoa(id),
			name: "room_left",
			evt: &mixpanel.Event{
				IP: "0",
				Properties: map[string]interface{}{
					"room_id": event.Params["id"],
				},
			},
		}
	case pubsub.EventTypeNewUser:
		id, err := getId(event, "id")
		if err != nil {
			return nil
		}

		return &Event{
			id:   strconv.Itoa(id),
			name: "new_user",
			evt: &mixpanel.Event{
				IP: "0",
				Properties: map[string]interface{}{
					"user_id": event.Params["id"],
					"username": event.Params["username"],
				},
			},
		}
	}

	return nil
}

func getId(event *pubsub.Event, field string) (int, error) {
	creator, ok := event.Params[field].(float64)
	if !ok {
		return 0, errors.New("failed to recover creator")
	}

	return int(creator), nil
}
