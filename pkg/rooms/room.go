package rooms

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/ion-sfu/pkg/sfu"
	"github.com/pion/webrtc/v3"
	"google.golang.org/protobuf/proto"

	"github.com/soapboxsocial/soapbox/pkg/minis"
	"github.com/soapboxsocial/soapbox/pkg/pubsub"
	"github.com/soapboxsocial/soapbox/pkg/rooms/internal"
	"github.com/soapboxsocial/soapbox/pkg/rooms/pb"
)

const CHANNEL = "soapbox"

type RoomConnectionState int

const (
	open RoomConnectionState = iota
	closed
)

type (
	JoinHandlerFunc         func(room *Room, me *Member, isNew bool)
	InviteHandlerFunc       func(room string, from, to int)
	DisconnectedHandlerFunc func(room string, peer *Member)
)

type Room struct {
	mux sync.RWMutex

	id         string
	name       string
	visibility pb.Visibility

	state RoomConnectionState

	members map[int]*Member

	adminInvites map[int]bool
	kicked       map[int]bool
	invited      map[int]bool

	// users that were admins when they disconnected.
	adminsOnDisconnected map[int]bool

	link string
	mini *pb.RoomState_Mini

	minis *minis.Backend

	peerToMember map[string]int

	onDisconnectedHandlerFunc DisconnectedHandlerFunc
	onInviteHandlerFunc       InviteHandlerFunc
	onJoinHandlerFunc         JoinHandlerFunc

	session sfu.Session

	queue *pubsub.Queue
}

func NewRoom(
	id,
	name string,
	owner int,
	visibility pb.Visibility,
	session sfu.Session,
	queue *pubsub.Queue,
	backend *minis.Backend,
) *Room {
	r := &Room{
		id:                   id,
		name:                 name,
		visibility:           visibility,
		state:                closed,
		members:              make(map[int]*Member),
		adminInvites:         make(map[int]bool),
		kicked:               make(map[int]bool),
		invited:              make(map[int]bool),
		peerToMember:         make(map[string]int),
		adminsOnDisconnected: make(map[int]bool),
		session:              session,
		queue:                queue,
		minis:                backend,
	}

	r.invited[owner] = true

	dc := sfu.NewDataChannel(CHANNEL)
	dc.OnMessage(func(ctx context.Context, args sfu.ProcessArgs) {
		m := &pb.Command{}
		err := proto.Unmarshal(args.Message.Data, m)
		if err != nil {
			log.Printf("error unmarshalling: %v", err)
			return
		}

		r.mux.RLock()
		user, ok := r.peerToMember[args.Peer.ID()]
		r.mux.RUnlock()

		if !ok {
			return
		}

		r.onMessage(user, m)
	})

	session.AddDataChannelMiddleware(dc)

	return r
}

func (r *Room) ID() string {
	return r.id
}

func (r *Room) PeerCount() int {
	r.mux.RLock()
	defer r.mux.RUnlock()
	return len(r.members)
}

func (r *Room) Name() string {
	r.mux.RLock()
	defer r.mux.RUnlock()
	return r.name
}

func (r *Room) WasAdminOnDisconnect(id int) bool {
	r.mux.RLock()
	defer r.mux.RUnlock()

	return r.adminsOnDisconnected[id]
}

func (r *Room) ConnectionState() RoomConnectionState {
	r.mux.RLock()
	defer r.mux.RUnlock()
	return r.state
}

func (r *Room) SetConnectionState(state RoomConnectionState) {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.state = state
}

func (r *Room) Visibility() pb.Visibility {
	r.mux.RLock()
	defer r.mux.RUnlock()
	return r.visibility
}

func (r *Room) IsKicked(id int) bool {
	r.mux.RLock()
	defer r.mux.RUnlock()
	return r.kicked[id]
}

func (r *Room) IsInvited(id int) bool {
	r.mux.RLock()
	defer r.mux.RUnlock()
	return r.invited[id]
}

func (r *Room) isAdmin(id int) bool {
	member := r.member(id)
	if member == nil {
		return false
	}

	return member.Role() == pb.RoomState_RoomMember_ROLE_ADMIN
}

func (r *Room) isInvitedToBeAdmin(id int) bool {
	r.mux.RLock()
	defer r.mux.RUnlock()

	return r.adminInvites[id]
}

func (r *Room) MapMembers(f func(member *Member)) {
	r.mux.RLock()
	defer r.mux.RUnlock()

	for _, member := range r.members {
		f(member)
	}
}

func (r *Room) OnDisconnected(f DisconnectedHandlerFunc) {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.onDisconnectedHandlerFunc = f
}

func (r *Room) OnInvite(f InviteHandlerFunc) {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.onInviteHandlerFunc = f
}

func (r *Room) OnJoin(f JoinHandlerFunc) {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.onJoinHandlerFunc = f
}

func (r *Room) ToProto() *pb.RoomState {
	r.mux.RLock()
	defer r.mux.RUnlock()

	members := make([]*pb.RoomState_RoomMember, 0)

	for _, member := range r.members {
		members = append(members, member.ToProto())
	}

	state := &pb.RoomState{
		Id:         r.id,
		Name:       r.name,
		Members:    members,
		Visibility: r.visibility,
		Link:       r.link,
		Mini:       r.mini,
	}

	if r.mini != nil {
		state.MiniOld = r.mini.Slug
	}

	return state
}

func (r *Room) Handle(me *Member) {
	r.mux.Lock()
	r.peerToMember[me.peer.ID()] = me.id
	r.mux.Unlock()

	isNew := r.ConnectionState() == closed
	if isNew {
		me.SetRole(pb.RoomState_RoomMember_ROLE_ADMIN)
	}

	if r.member(me.id) != nil {
		_ = me.Close()
		return
	}

	r.mux.Lock()
	delete(r.adminsOnDisconnected, me.id)
	r.mux.Unlock()

	me.StartChannel(CHANNEL)

	r.mux.Lock()
	r.members[me.id] = me
	r.mux.Unlock()

	me.peer.OnICEConnectionStateChange = func(state webrtc.ICEConnectionState) {
		log.Printf("connection state changed %d for peer %d", state, me.id)

		switch state {
		case webrtc.ICEConnectionStateConnected:
			if me.IsConnected() {
				return
			}

			r.onJoinHandlerFunc(r, me, isNew)
			r.SetConnectionState(open)
			me.MarkConnected()

			r.notify(&pb.Event{
				From: int64(me.id),
				Payload: &pb.Event_Joined_{
					Joined: &pb.Event_Joined{User: me.ToProto()},
				},
			})
		case webrtc.ICEConnectionStateClosed, webrtc.ICEConnectionStateFailed:
			r.onDisconnected(int64(me.id))
		}
	}

	err := me.RunSignal()
	if err != nil {
		_, ok := err.(*websocket.CloseError)
		if ok {
			r.onDisconnected(int64(me.id))
			return
		}

		log.Printf("me.Signal err: %v", err)
	}
}

func (r *Room) onDisconnected(id int64) {
	log.Printf("disconnected %d", id)

	peer := r.member(int(id))
	if peer == nil {
		return
	}

	err := peer.Close()
	if err != nil {
		log.Printf("rtc.Close error %v\n", err)
	}

	r.mux.Lock()
	if peer.Role() == pb.RoomState_RoomMember_ROLE_ADMIN {
		r.adminsOnDisconnected[int(id)] = true
	}

	delete(r.members, int(id))
	r.mux.Unlock()

	r.notify(&pb.Event{
		From:    id,
		Payload: &pb.Event_Left_{},
	})

	r.electRandomAdmin(id)

	r.onDisconnectedHandlerFunc(r.id, peer)
}

func (r *Room) electRandomAdmin(previous int64) {
	r.mux.Lock()
	defer r.mux.Unlock()

	hasAdmin := has(r.members, func(me *Member) bool {
		return me.Role() == pb.RoomState_RoomMember_ROLE_ADMIN
	})

	if hasAdmin {
		return
	}

	for k := range r.members {
		r.members[k].SetRole(pb.RoomState_RoomMember_ROLE_ADMIN)

		go r.notify(&pb.Event{
			From:    previous,
			Payload: &pb.Event_AddedAdmin_{AddedAdmin: &pb.Event_AddedAdmin{Id: int64(k)}},
		})
		break
	}
}

func (r *Room) ContainsUsers(users []int) bool {
	r.mux.RLock()
	defer r.mux.RUnlock()

	for _, id := range users {
		_, ok := r.members[id]
		if ok {
			return true
		}
	}

	return false
}

func has(members map[int]*Member, fn func(*Member) bool) bool {
	for _, member := range members {
		if fn(member) {
			return true
		}
	}

	return false
}

func (r *Room) onMessage(from int, command *pb.Command) {
	switch command.Payload.(type) {
	case *pb.Command_MuteUpdate_:
		r.onMuteUpdate(from, command.GetMuteUpdate())
	case *pb.Command_Reaction_:
		r.onReaction(from, command.GetReaction())
	case *pb.Command_LinkShare_:
		r.onLinkShare(from, command.GetLinkShare())
	case *pb.Command_InviteAdmin_:
		r.onInviteAdmin(from, command.GetInviteAdmin())
	case *pb.Command_AcceptAdmin_:
		r.onAcceptAdmin(from)
	case *pb.Command_RemoveAdmin_:
		r.onRemoveAdmin(from, command.GetRemoveAdmin())
	case *pb.Command_RenameRoom_:
		r.onRenameRoom(from, command.GetRenameRoom())
	case *pb.Command_InviteUser_:
		r.onInviteUser(from, command.GetInviteUser())
	case *pb.Command_KickUser_:
		r.onKickUser(from, command.GetKickUser())
	case *pb.Command_MuteUser_:
		r.onMuteUser(from, command.GetMuteUser())
	case *pb.Command_RecordScreen_:
		r.onRecordScreen(from)
	case *pb.Command_VisibilityUpdate_:
		r.onVisibilityUpdate(from, command.GetVisibilityUpdate())
	case *pb.Command_PinLink_:
		r.onPinLink(from, command.GetPinLink())
	case *pb.Command_UnpinLink_:
		r.onUnpinLink(from)
	case *pb.Command_OpenMini_:
		r.onOpenMini(from, command.GetOpenMini())
	case *pb.Command_CloseMini_:
		r.onCloseMini(from)
	case *pb.Command_RequestMini_:
		r.onRequestMini(from, command.GetRequestMini())
	}
}

func (r *Room) onMuteUpdate(from int, cmd *pb.Command_MuteUpdate) {
	member := r.member(from)
	if member == nil {
		log.Printf("member %d not found", from)
		return
	}

	if cmd.Muted {
		member.Mute()
	} else {
		member.Unmute()
	}

	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_MuteUpdated_{MuteUpdated: &pb.Event_MuteUpdated{IsMuted: cmd.Muted}},
	})
}

func (r *Room) onReaction(from int, cmd *pb.Command_Reaction) {
	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_Reacted_{Reacted: &pb.Event_Reacted{Emoji: cmd.Emoji}},
	})
}

func (r *Room) onLinkShare(from int, cmd *pb.Command_LinkShare) {
	_ = r.queue.Publish(
		pubsub.RoomTopic,
		pubsub.NewRoomLinkShareEvent(from, r.id),
	)

	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_LinkShared_{LinkShared: &pb.Event_LinkShared{Link: cmd.Link}},
	})
}

func (r *Room) onInviteAdmin(from int, cmd *pb.Command_InviteAdmin) {
	if !r.isAdmin(from) {
		return
	}

	member := r.member(int(cmd.Id))
	if member == nil {
		return
	}

	r.mux.Lock()
	r.adminInvites[int(cmd.Id)] = true
	r.mux.Unlock()

	event := &pb.Event{
		From:    int64(from),
		Payload: &pb.Event_InvitedAdmin_{InvitedAdmin: &pb.Event_InvitedAdmin{Id: cmd.Id}},
	}

	data, err := proto.Marshal(event)
	if err != nil {
		log.Printf("failed to marshal %v", err)
		return
	}

	err = member.Notify(data)
	if err != nil {
		log.Printf("failed to notify %v", err)
	}
}

func (r *Room) onAcceptAdmin(from int) {
	if !r.isInvitedToBeAdmin(from) {
		return
	}

	r.mux.Lock()
	delete(r.adminInvites, from)
	r.mux.Unlock()

	member := r.member(from)
	if member == nil {
		return
	}

	member.SetRole(pb.RoomState_RoomMember_ROLE_ADMIN)

	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_AddedAdmin_{AddedAdmin: &pb.Event_AddedAdmin{Id: int64(from)}},
	})
}

func (r *Room) onRemoveAdmin(from int, cmd *pb.Command_RemoveAdmin) {
	if !r.isAdmin(from) {
		return
	}

	member := r.member(from)
	if member == nil {
		log.Printf("member %d not found", from)
		return
	}

	delete(r.adminsOnDisconnected, from)

	member.SetRole(pb.RoomState_RoomMember_ROLE_ADMIN)

	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_RemovedAdmin_{RemovedAdmin: &pb.Event_RemovedAdmin{Id: cmd.Id}},
	})
}

func (r *Room) onRenameRoom(from int, cmd *pb.Command_RenameRoom) {
	if !r.isAdmin(from) {
		return
	}

	r.mux.Lock()
	r.name = internal.TrimRoomNameToLimit(cmd.Name)
	r.mux.Unlock()

	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_RenamedRoom_{RenamedRoom: &pb.Event_RenamedRoom{Name: r.name}},
	})
}

func (r *Room) onInviteUser(from int, cmd *pb.Command_InviteUser) {
	if r.Visibility() == pb.Visibility_VISIBILITY_PRIVATE && !r.isAdmin(from) {
		return
	}

	r.InviteUser(from, int(cmd.Id))
}

func (r *Room) InviteUser(from, to int) {
	r.mux.Lock()
	r.invited[to] = true
	r.mux.Unlock()

	r.onInviteHandlerFunc(r.id, from, to)
}

func (r *Room) onKickUser(from int, cmd *pb.Command_KickUser) {
	if !r.isAdmin(from) {
		return
	}

	p := r.member(int(cmd.Id))
	if p == nil {
		return
	}

	r.mux.Lock()
	r.kicked[int(cmd.Id)] = true
	r.mux.Unlock()

	_ = p.Close()
}

func (r *Room) onMuteUser(from int, cmd *pb.Command_MuteUser) {
	if !r.isAdmin(from) {
		return
	}

	member := r.member(int(cmd.Id))
	if member == nil {
		return
	}

	event := &pb.Event{
		From:    int64(from),
		Payload: &pb.Event_MutedByAdmin_{MutedByAdmin: &pb.Event_MutedByAdmin{Id: cmd.Id}},
	}

	data, err := proto.Marshal(event)
	if err != nil {
		log.Printf("failed to marshal %v", err)
		return
	}

	err = member.Notify(data)
	if err != nil {
		log.Printf("failed to notify %v", err)
	}
}

func (r *Room) onRecordScreen(from int) {
	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_RecordedScreen_{RecordedScreen: &pb.Event_RecordedScreen{}},
	})
}

func (r *Room) onVisibilityUpdate(from int, cmd *pb.Command_VisibilityUpdate) {
	if !r.isAdmin(from) {
		return
	}

	r.mux.Lock()
	r.visibility = cmd.Visibility

	for i := range r.members {
		r.invited[i] = true
	}

	r.mux.Unlock()

	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_VisibilityUpdated_{VisibilityUpdated: &pb.Event_VisibilityUpdated{Visibility: cmd.Visibility}},
	})
}

func (r *Room) onPinLink(from int, cmd *pb.Command_PinLink) {
	if !r.isAdmin(from) {
		return
	}

	r.mux.RLock()
	link := r.link
	mini := r.mini
	r.mux.RUnlock()

	if mini != nil {
		return
	}

	// @TODO MAY NEED TO BE BETTER
	if link != "" {
		return
	}

	r.mux.Lock()
	r.link = cmd.Link
	r.mux.Unlock()

	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_PinnedLink_{PinnedLink: &pb.Event_PinnedLink{Link: cmd.Link}},
	})
}

func (r *Room) onUnpinLink(from int) {
	if !r.isAdmin(from) {
		return
	}

	r.mux.Lock()
	r.link = ""
	r.mux.Unlock()

	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_UnpinnedLink_{UnpinnedLink: &pb.Event_UnpinnedLink{}},
	})
}

func (r *Room) onOpenMini(from int, mini *pb.Command_OpenMini) {
	if !r.isAdmin(from) {
		return
	}

	r.mux.RLock()
	link := r.link
	r.mux.RUnlock()

	if link != "" {
		return
	}

	_ = r.queue.Publish(
		pubsub.RoomTopic,
		pubsub.NewRoomOpenMiniEvent(from, int(mini.Id), r.id),
	)

	resp, err := r.getMini(mini)
	if err != nil {
		return
	}

	minipb := &pb.RoomState_Mini{
		Id:   int64(resp.ID),
		Slug: resp.Slug,
		Size: pb.RoomState_Mini_Size(int32(resp.Size)),
		Name: resp.Name,
	}

	r.mux.Lock()
	r.mini = minipb
	r.mux.Unlock()

	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_OpenedMini_{OpenedMini: &pb.Event_OpenedMini{Slug: minipb.Slug, Mini: minipb}},
	})
}

func (r *Room) onCloseMini(from int) {
	if !r.isAdmin(from) {
		return
	}

	r.mux.Lock()
	r.mini = nil
	r.mux.Unlock()

	r.notify(&pb.Event{
		From:    int64(from),
		Payload: &pb.Event_ClosedMini_{ClosedMini: &pb.Event_ClosedMini{}},
	})
}

func (r *Room) onRequestMini(from int, cmd *pb.Command_RequestMini) {
	id := cmd.GetId()
	if id == 0 {
		return
	}

	mini, err := r.minis.GetMiniWithID(int(id))
	if err != nil {
		log.Printf("failed to get mini: %d err: %s", id, err)
		return
	}

	msg := &pb.Event{
		From: int64(from),
		Payload: &pb.Event_RequestedMini_{
			RequestedMini: &pb.Event_RequestedMini{
				Mini: &pb.RoomState_Mini{
					Id:   int64(mini.ID),
					Name: mini.Name,
					Slug: mini.Slug,
					Size: pb.RoomState_Mini_Size(int32(mini.Size)),
				},
			},
		},
	}

	raw, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal err: %s", err)
		return
	}

	r.MapMembers(func(member *Member) {
		if member.Role() != pb.RoomState_RoomMember_ROLE_ADMIN {
			return
		}

		err := member.Notify(raw)
		if err != nil {
			log.Printf("member.Notify err: %s", err)
		}
	})
}

func (r *Room) getMini(cmd *pb.Command_OpenMini) (*minis.Mini, error) {
	if cmd.GetId() != 0 {
		return r.minis.GetMiniWithID(int(cmd.Id))
	}

	if cmd.GetMini() != "" {
		return r.minis.GetMiniWithSlug(cmd.Mini)
	}

	return nil, errors.New("no mini found")
}

func (r *Room) member(id int) *Member {
	r.mux.RLock()
	defer r.mux.RUnlock()

	member, ok := r.members[id]
	if !ok {
		return nil
	}

	return member
}

func (r *Room) notify(event *pb.Event) {
	data, err := proto.Marshal(event)
	if err != nil {
		log.Printf("failed to marshal: %v", err)
		return
	}

	r.mux.RLock()
	members := r.members
	r.mux.RUnlock()

	for id, member := range members {
		if id == int(event.From) {
			continue
		}

		err := member.Notify(data)
		if err != nil {
			if err == io.EOF {
				r.onDisconnected(int64(id))
				continue
			}

			log.Printf("failed to notify: %v\n", err)
		}
	}
}
