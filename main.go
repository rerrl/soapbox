package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pion/webrtc/v3"

	"github.com/ephemeral-networks/voicely/pkg/rooms"
)

type RoomPayload struct {
	ID      int      `json:"id"`
	Members []string `json:"members"`
}

type SDPPayload struct {
	ID   *int   `json:"id,omitempty"`
	SDP  string `json:"sdp"`
	Type string `json:"type"`
}

type JoinPayload struct {
	Members []string   `json:"members"`
	SDP     SDPPayload `json:"sdp"`
}

func main() {

	manager := rooms.NewRoomManager()

	r := mux.NewRouter()

	r.HandleFunc("/v1/rooms", func(w http.ResponseWriter, r *http.Request) {
		data := make([]RoomPayload, 0)

		manager.MapRooms(func(room *rooms.Room) {
			if room == nil {
				return
			}

			r := RoomPayload{ID: room.GetID(), Members: make([]string, 0)}

			room.MapPeers(func(s string, peer rooms.Peer) {
				r.Members = append(r.Members, s)
			})

			data = append(data, r)
		})

		err := jsonEncode(w, data)
		if err != nil {
			fmt.Println(err)
		}
	}).Methods("GET")

	r.HandleFunc("/v1/rooms/create", func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		payload := &SDPPayload{}
		err = json.Unmarshal(b, payload)
		if err != nil {
			log.Printf("failed to decode payload: %s\n", err.Error())
			return
		}

		err, t := getType(payload.Type)
		if err != nil {
			// @todo more errors this shit is invalid
			return
		}

		p := webrtc.SessionDescription{
			Type: t,
			SDP:  payload.SDP,
		}

		room := manager.CreateRoom()

		sdp, err := room.Join(r.RemoteAddr, p)
		if err != nil {
			// @todo
			return
		}

		id := room.GetID()
		resp := &SDPPayload{ID: &id, Type: strings.ToLower(sdp.Type.String()), SDP: sdp.SDP}

		err = jsonEncode(w, resp)
		if err != nil {
			fmt.Println(err)
		}
	}).Methods("POST")

	r.HandleFunc("/v1/rooms/{id:[0-9]+}/join", func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		payload := &SDPPayload{}
		err = json.Unmarshal(b, payload)
		if err != nil {
			log.Printf("failed to decode payload: %s\n", err.Error())
			return
		}

		err, t := getType(payload.Type)
		if err != nil {
			// @todo more errors this shit is invalid
			return
		}

		p := webrtc.SessionDescription{
			Type: t,
			SDP:  payload.SDP,
		}

		vars := mux.Vars(r)
		id, err := strconv.Atoi(vars["id"])
		if err != nil {
			return
		}

		room, err := manager.GetRoom(id)
		if err != nil {
			// @todo handle
			return
		}

		sdp, err := room.Join(r.RemoteAddr, p)
		if err != nil {
			// @todo
			return
		}

		members := make([]string, 0)

		room.MapPeers(func(s string, _ rooms.Peer) {
			// @todo will need changing
			if s == r.RemoteAddr {
				return
			}

			members = append(members, s)
		})

		resp := &JoinPayload{
			Members: members,
			SDP: SDPPayload{
				ID:   &id,
				Type: strings.ToLower(sdp.Type.String()),
				SDP:  sdp.SDP,
			},
		}

		err = jsonEncode(w, resp)
		if err != nil {
			fmt.Println(err)
		}
	}).Methods("POST")

	log.Fatal(http.ListenAndServe(":8080", r))
}

func jsonEncode(w http.ResponseWriter, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}

func getType(t string) (error, webrtc.SDPType) {
	switch t {
	case "offer":
		return nil, webrtc.SDPTypeOffer
	case "prAnswer":
		return nil, webrtc.SDPTypePranswer
	case "answer":
		return nil, webrtc.SDPTypeAnswer
	}

	return fmt.Errorf("unknown type: %s", t), webrtc.SDPType(-1)
}
