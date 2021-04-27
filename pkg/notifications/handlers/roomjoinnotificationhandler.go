package handlers

import (
	"context"
	"errors"
	"strconv"

	"github.com/soapboxsocial/soapbox/pkg/notifications"
	"github.com/soapboxsocial/soapbox/pkg/pubsub"
	"github.com/soapboxsocial/soapbox/pkg/rooms/pb"
)

var (
	errRoomPrivate   = errors.New("room is private")
	errNoRoomMembers = errors.New("room is empty")
	errFailedToSort  = errors.New("failed to sort")
	errEmptyResponse = errors.New("empty response")
)

type RoomJoinNotificationHandler struct {
	metadata pb.RoomServiceClient
	targets  *notifications.Settings
}

func NewRoomJoinNotificationHandler(targets *notifications.Settings, metadata pb.RoomServiceClient) *RoomJoinNotificationHandler {
	return &RoomJoinNotificationHandler{
		targets:  targets,
		metadata: metadata,
	}
}

func (r RoomJoinNotificationHandler) Type() pubsub.EventType {
	return pubsub.EventTypeRoomJoin
}

func (r RoomJoinNotificationHandler) Targets(event *pubsub.Event) ([]notifications.Target, error) {
	creator, err := event.GetInt("creator")
	if err != nil {
		return nil, err
	}

	targets, err := r.targets.GetSettingsFollowingUser(creator)
	if err != nil {
		return nil, err
	}

	return targets, nil
}

func (r RoomJoinNotificationHandler) Build(event *pubsub.Event) (*notifications.PushNotification, error) {
	if pubsub.RoomVisibility(event.Params["visibility"].(string)) == pubsub.Private {
		return nil, errRoomPrivate
	}

	creator, err := event.GetInt("creator")
	if err != nil {
		return nil, err
	}

	room := event.Params["id"].(string)
	response, err := r.metadata.GetRoom(context.Background(), &pb.GetRoomRequest{Id: room})
	if err != nil {
		return nil, err
	}

	if response == nil || response.State == nil {
		return nil, errEmptyResponse
	}

	state := response.State

	if len(state.Members) == 0 {
		return nil, errNoRoomMembers
	}

	translation := "join_room_with_"
	args := make([]string, 0)

	if state.Name != "" {
		translation = "join_room_name_with_"
		args = append(args, state.Name)
	}

	count := len(state.Members)

	if count == 1 {
		translation += "1"
		args = append(args, state.Members[0].DisplayName)
	}

	if count == 2 {
		translation += "2"
		args = append(args, state.Members[0].DisplayName, state.Members[1].DisplayName)
	}

	if count == 3 {
		translation += "3"
		args = append(args, state.Members[0].DisplayName, state.Members[1].DisplayName, state.Members[2].DisplayName)
	}

	if count > 3 {
		translation += "3_and_more"

		members := members(state.Members, creator)
		if len(members) < 3 {
			return nil, errFailedToSort
		}

		args = append(args, members[0], members[1], members[2], strconv.Itoa(count-3))
	}

	notification := &notifications.PushNotification{
		Category: notifications.ROOM_JOINED,
		Alert: notifications.Alert{
			Key:       translation + "_notification",
			Arguments: args,
		},
		CollapseID: room,
		Arguments:  map[string]interface{}{"id": room},
	}

	return notification, nil
}

func members(members []*pb.RoomState_RoomMember, first int) []string {
	names := make([]string, 0)

	for i, member := range members {
		if member.Id == int64(first) {
			names = append(names, member.DisplayName)
			members = append(members[:i], members[i+1:]...)
		}
	}

	names = append(names, members[0].DisplayName, members[1].DisplayName)

	return names
}